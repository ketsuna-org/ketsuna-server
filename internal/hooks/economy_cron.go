package hooks

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/tools/types"
)

// EnergyStatus represents the energy state of a company
type EnergyStatus struct {
	EnergyProduced  float64 `json:"energyProduced"`
	EnergyDemand    float64 `json:"energyDemand"`
	EnergyStored    float64 `json:"energyStored"`
	MaxEnergyStored float64 `json:"maxEnergyStored"`
	EnergyRatio     float64 `json:"energyRatio"`
	IsSolarActive   bool    `json:"isSolarActive"`
	ProductionSpeed float64 `json:"productionSpeed"` // 0.0 to 1.0
}

// IsSolarProductionActive checks if current UTC hour is between 8h and 18h
func IsSolarProductionActive() bool {
	hour := time.Now().UTC().Hour()
	return hour >= 8 && hour < 18
}

// CalculateEnergyStatus computes energy production, consumption, and ratio for a company
func (l *EconomyLogic) CalculateEnergyStatus(companyId string) (*EnergyStatus, error) {
	status := &EnergyStatus{
		EnergyRatio:     1.0,
		ProductionSpeed: 1.0,
		IsSolarActive:   IsSolarProductionActive(),
	}

	// Fetch assigned machines
	assignedMachines, err := l.app.FindRecordsByFilter(
		"machines",
		fmt.Sprintf("company = '%s'", companyId),
		"",
		0,
		0,
	)
	if err != nil || len(assignedMachines) == 0 {
		return status, nil
	}

	// Fetch employees for efficiency calculation
	employees, _ := l.app.FindRecordsByFilter(
		"employees",
		fmt.Sprintf("employer = '%s'", companyId),
		"",
		0,
		0,
	)

	for _, assignment := range assignedMachines {
		machineItemId := assignment.GetString("machine")
		if machineItemId == "" {
			continue
		}

		machineItem, err := l.app.FindRecordById("items", machineItemId)
		if err != nil {
			continue
		}

		// Calculate efficiency from assigned employees
		assignedEmpIds := assignment.GetStringSlice("employees")
		totalEfficiency := 0.0
		for _, empId := range assignedEmpIds {
			for _, emp := range employees {
				if emp.Id == empId {
					totalEfficiency += emp.GetFloat("efficiency")
				}
			}
		}
		if totalEfficiency <= 0 {
			totalEfficiency = 1.0 // Default for machines without employees
		}

		// Check if this machine produces energy
		produceEnergy := machineItem.GetFloat("produce_energy")
		if produceEnergy > 0 {
			energyType := machineItem.GetString("energy_type")

			// Solar only produces during 8h-18h UTC
			if energyType == "Soleil" && !status.IsSolarActive {
				// Solar panel inactive, no production
			} else {
				status.EnergyProduced += produceEnergy * totalEfficiency
			}
		}

		// Check if this machine stores energy
		canStoreEnergy := machineItem.GetFloat("can_store_energy")
		if canStoreEnergy > 0 {
			status.MaxEnergyStored += canStoreEnergy
			status.EnergyStored += assignment.GetFloat("stored_energy")
		}

		// Check if this machine consumes energy
		needEnergy := machineItem.GetFloat("need_energy")
		if needEnergy > 0 {
			status.EnergyDemand += needEnergy
		}
	}

	// Cap stored energy at max
	if status.EnergyStored > status.MaxEnergyStored {
		status.EnergyStored = status.MaxEnergyStored
	}

	// Calculate energy ratio
	if status.EnergyDemand > 0 {
		// Available energy = production + stored buffer
		availableEnergy := status.EnergyProduced + status.EnergyStored
		status.EnergyRatio = availableEnergy / status.EnergyDemand

		if status.EnergyRatio > 1.0 {
			status.EnergyRatio = 1.0
		}
		if status.EnergyRatio < 0.0 {
			status.EnergyRatio = 0.0
		}

		status.ProductionSpeed = status.EnergyRatio
	}

	return status, nil
}

// ProcessCompanyEconomy handles machine production for a single company
func (l *EconomyLogic) ProcessCompanyEconomy(companyId string) error {
	_, err := l.app.FindRecordById("companies", companyId)
	if err != nil {
		return err
	}

	// Calculate energy status for this company
	energyStatus, _ := l.CalculateEnergyStatus(companyId)

	// Fetch assigned machines
	assignedMachines, err := l.app.FindRecordsByFilter(
		"machines",
		fmt.Sprintf("company = '%s'", companyId),
		"",
		0,
		0,
	)
	if err != nil {
		return err
	}

	if len(assignedMachines) == 0 {
		return nil
	}

	// Pre-fetch employees for efficiency calculation
	employees, err := l.app.FindRecordsByFilter(
		"employees",
		fmt.Sprintf("employer = '%s'", companyId),
		"",
		0,
		0,
	)
	if err != nil {
		return err
	}

	// Track energy updates for storage machines
	energySurplus := energyStatus.EnergyProduced - energyStatus.EnergyDemand

	for _, assignment := range assignedMachines {
		machineItemId := assignment.GetString("machine")
		if machineItemId == "" {
			continue
		}

		machineItem, err := l.app.FindRecordById("items", machineItemId)
		if err != nil {
			continue
		}

		// Handle energy storage: store surplus or drain for deficit
		canStoreEnergy := machineItem.GetFloat("can_store_energy")
		if canStoreEnergy > 0 && energySurplus != 0 {
			currentStored := assignment.GetFloat("stored_energy")
			newStored := currentStored + energySurplus

			// Clamp to [0, max]
			if newStored < 0 {
				newStored = 0
			}
			if newStored > canStoreEnergy {
				newStored = canStoreEnergy
			}

			if newStored != currentStored {
				assignment.Set("stored_energy", newStored)
				l.app.Save(assignment)
			}
			// Reset surplus after first storage machine handles it
			energySurplus = 0
		}

		// Handle energy-producing machines (generators, solar panels, etc.)
		produceEnergy := machineItem.GetFloat("produce_energy")
		if produceEnergy > 0 {
			// Calculate efficiency for this machine
			assignedEmpIds := assignment.GetStringSlice("employees")
			genEfficiency := 0.0
			for _, empId := range assignedEmpIds {
				for _, emp := range employees {
					if emp.Id == empId {
						genEfficiency += emp.GetFloat("efficiency")
					}
				}
			}
			if genEfficiency <= 0 {
				genEfficiency = 1.0 // Default for unmanned generators
			}

			recipeId := machineItem.GetString("use_recipe")

			// CASE 1: Simple Consumption (Direct Fuel -> Energy)
			consumeItemIds := machineItem.GetStringSlice("can_consume")
			if len(consumeItemIds) > 0 {
				// Initialize production if not started (instant consumption model for simple fuel)
				// Or we can use production_time from current machine if we want timed cycles?
				// Let's assume simple consumption is instant per tick for now, or check production_time of machine itself
				machineProdTime := machineItem.GetInt("production_time")

				if machineProdTime > 0 {
					startedAt := assignment.GetDateTime("production_started_at")
					if startedAt.IsZero() {
						// Don't consume if storage is full
						canStore := machineItem.GetFloat("can_store_energy")
						currentStored := assignment.GetFloat("stored_energy")
						if canStore > 0 && currentStored >= canStore {
							continue
						}

						// Try convert 1 fuel from any allowed
						consumed := false
						for _, itemId := range consumeItemIds {
							_, err := l.inventory.ConsumeItem(companyId, itemId, 1)
							if err == nil {
								consumed = true
								break
							}
						}

						if consumed {
							assignment.Set("production_started_at", types.NowDateTime())
							l.app.Save(assignment)
						}
					} else {
						// Check completion
						elapsed := time.Since(startedAt.Time()).Seconds()
						if elapsed >= float64(machineProdTime) {
							// Produce Energy
							canStore := machineItem.GetFloat("can_store_energy")
							if canStore > 0 {
								currentStored := assignment.GetFloat("stored_energy")
								energyProduced := produceEnergy * genEfficiency
								newStored := currentStored + energyProduced
								if newStored > canStore {
									newStored = canStore
								}
								assignment.Set("stored_energy", newStored)
							}
							// Reset
							assignment.Set("production_started_at", types.DateTime{})
							l.app.Save(assignment)
						}
					}
				} else {
					// Instant consumption (1 fuel per tick)
					// Don't consume if storage is full
					canStore := machineItem.GetFloat("can_store_energy")
					currentStored := assignment.GetFloat("stored_energy")
					if canStore > 0 && currentStored >= canStore {
						continue
					}

					// Try consume 1 fuel from any allowed
					for _, itemId := range consumeItemIds {
						_, err := l.inventory.ConsumeItem(companyId, itemId, 1)
						if err == nil {
							// Successfully consumed fuel, produce energy
							if canStore > 0 {
								currentStored := assignment.GetFloat("stored_energy")
								energyProduced := produceEnergy * genEfficiency
								newStored := currentStored + energyProduced
								if newStored > canStore {
									newStored = canStore
								}
								assignment.Set("stored_energy", newStored)
								// No need to save if nothing else changed, but stored_energy changed
								l.app.Save(assignment)
							}
							break // Stop after one success
						}
					}
				}

			} else if recipeId != "" {
				// CASE 2: Recipe (Complex -> potentially waste output)
				recipe, err := l.app.FindRecordById("recipes", recipeId)
				if err != nil {
					continue
				}
				productionTime := recipe.GetInt("production_time")

				if productionTime > 0 {
					startedAt := assignment.GetDateTime("production_started_at")
					if startedAt.IsZero() {
						// Don't consume if storage is full
						canStore := machineItem.GetFloat("can_store_energy")
						currentStored := assignment.GetFloat("stored_energy")
						if canStore > 0 && currentStored >= canStore {
							continue
						}

						// Try consume recipe inputs
						// Use ConsumeInputs which should handle both formats if implemented correctly,
						// or we need to implement robust recipe consumption here.
						// Assuming l.inventory.ConsumeInputs handles the recipe consumption logic (ingredients AND inputs_items)
						_, err := l.inventory.ConsumeInputs(companyId, recipeId, 1)
						if err == nil {
							assignment.Set("production_started_at", types.NowDateTime())
							l.app.Save(assignment)
						}
					} else {
						elapsed := time.Since(startedAt.Time()).Seconds()
						if elapsed >= float64(productionTime) {
							// 1. Produce Energy
							canStore := machineItem.GetFloat("can_store_energy")
							if canStore > 0 {
								currentStored := assignment.GetFloat("stored_energy")
								energyProduced := produceEnergy * genEfficiency
								newStored := currentStored + energyProduced
								if newStored > canStore {
									newStored = canStore
								}
								assignment.Set("stored_energy", newStored)
							}

							// 2. Produce Output (Waste/Bi-product) if any
							outputItemId := recipe.GetString("output_item")
							if outputItemId != "" {
								l.inventory.AddRefinedItem(companyId, outputItemId, 1)
							}

							// Reset
							assignment.Set("production_started_at", types.DateTime{})
							l.app.Save(assignment)
						}
					}
				}
			}
			// If neither, it's passive (solar)
			continue
		}

		// Calculate Efficiency
		assignedEmpIds := assignment.GetStringSlice("employees")
		totalEfficiency := 0.0

		for _, empId := range assignedEmpIds {
			for _, emp := range employees {
				if emp.Id == empId {
					totalEfficiency += emp.GetFloat("efficiency")
				}
			}
		}

		if totalEfficiency <= 0 {
			continue
		}

		// Calc Output Quantity
		productQty := float64(machineItem.GetInt("product_quantity"))
		if productQty == 0 {
			productQty = 1
		}

		finalQty := int(math.Floor(productQty * totalEfficiency))
		if finalQty < 1 {
			finalQty = 1
		}

		// If 0, skip production
		if finalQty == 0 {
			continue
		}

		recipeId := machineItem.GetString("use_recipe")
		productId := machineItem.GetString("product")

		// Check if this machine needs energy
		needEnergy := machineItem.GetFloat("need_energy")

		// Apply Energy Penalty (Time Shift)
		// Instead of increasing required time, we shift started_at forward to simulate slowdown.
		// This allows the frontend to visualize the slowdown accurately.
		if needEnergy > 0 && energyStatus.ProductionSpeed < 1.0 {
			if energyStatus.ProductionSpeed <= 0 {
				// No energy, no production
				continue
			}

			startedAt := assignment.GetDateTime("production_started_at")
			if !startedAt.IsZero() {
				lastUpdate := assignment.GetDateTime("updated").Time()
				if !lastUpdate.IsZero() {
					delta := time.Since(lastUpdate).Seconds()
					// Sanity check: if delta is huge (server restart), cap it or ignore?
					// Let's assume cron runs regularly.
					if delta > 0 && delta < 3600 {
						penalty := delta * (1.0 - energyStatus.ProductionSpeed)
						newStart := startedAt.Time().Add(time.Duration(penalty * float64(time.Second)))
						assignment.Set("production_started_at", newStart)
						// We must save this intermediate state if we don't complete production in this tick
						// But if we complete, we overwrite it. Ideally we save at the end of loop if modified.
						// For now, let's set a flag or just save if we don't complete.
						// Actually, we can just update the variable in memory and let the saving happen if we don't complete?
						// Current code only saves on completion/start.
						// We need to force save if we shifted time but didn't finish.
						l.app.Save(assignment)
					}
				}
			}
		}

		if recipeId != "" {
			// --- RECIPE PRODUCTION ---
			// Check technology requirement first
			hasTech, techName := l.inventory.HasRequiredTechnology(companyId, recipeId)
			if !hasTech {
				l.app.Logger().Info("[ECONOMY] Machine blocked: missing technology",
					"machine", machineItem.GetString("name"),
					"tech", techName)
				continue
			}

			recipe, err := l.app.FindRecordById("recipes", recipeId)
			if err != nil {
				continue
			}
			baseProductionTime := recipe.GetInt("production_time")

			// Use base time, penalty is applied to started_at
			effectiveProductionTime := float64(baseProductionTime)

			if baseProductionTime > 0 {
				// Timed production (Any duration > 0)
				startedAt := assignment.GetDateTime("production_started_at")

				if startedAt.IsZero() {
					// Start Production
					_, err := l.inventory.ConsumeInputs(companyId, recipeId, finalQty)
					if err == nil {
						assignment.Set("production_started_at", types.NowDateTime())
						l.app.Save(assignment)
					}
				} else {
					// Check if finished (using effective time)
					elapsed := time.Since(startedAt.Time()).Seconds()
					if elapsed >= effectiveProductionTime {
						err := l.inventory.CompleteProduction(companyId, recipeId, finalQty)
						if err == nil {
							// Reset
							assignment.Set("production_started_at", types.DateTime{}) // Zero
							l.app.Save(assignment)
						} else {
							l.app.Logger().Error("[ECONOMY] Machine Production Error", "machine", machineItem.GetString("name"), "error", err)
						}
					}
				}
			} else {
				// Short Production (Immediate) - Only if production_time is 0
				_, _ = l.inventory.ProduceItem(companyId, recipeId, finalQty)
			}

		} else if productId != "" {
			// --- PASSIVE PRODUCTION ---
			baseProductionTime := machineItem.GetInt("production_time")

			// Use base time, penalty is applied to started_at
			effectiveProductionTime := float64(baseProductionTime)

			if baseProductionTime > 0 {
				// Timed Passive Production
				startedAt := assignment.GetDateTime("production_started_at")

				if startedAt.IsZero() {
					// Start Production
					assignment.Set("production_started_at", types.NowDateTime())
					l.app.Save(assignment)
				} else {
					// Check if finished (using effective time)
					elapsed := time.Since(startedAt.Time()).Seconds()
					if elapsed >= effectiveProductionTime {
						// Complete Production
						err := l.inventory.UpdateInventory(companyId, productId, finalQty)
						if err == nil {
							// Reset
							assignment.Set("production_started_at", types.DateTime{}) // Zero
							l.app.Save(assignment)
						} else {
							l.app.Logger().Error("[ECONOMY] Passive Production Error", "machine", machineItem.GetString("name"), "error", err)
						}
					}
				}
			} else {
				// Immediate Production (Every tick) - Only if production_time is 0 or missing
				_ = l.inventory.UpdateInventory(companyId, productId, finalQty)
			}
		}
	}

	return nil
}

func (l *EconomyLogic) UpdateMarketPrices() {
	// Count non-NPC companies (player companies)
	playerCompanies, _ := l.app.FindRecordsByFilter("companies", "is_npc = false", "", 0, 0)
	playerCompanyCount := len(playerCompanies)
	if playerCompanyCount == 0 {
		playerCompanyCount = 1 // Minimum to avoid 0
	}

	// Count total employees (for buy limit calculation)
	allEmployees, _ := l.app.FindRecordsByFilter("employees", "", "", 0, 0)
	totalEmployees := len(allEmployees)
	if totalEmployees == 0 {
		totalEmployees = 1
	}

	items, _ := l.app.FindRecordsByFilter("items", "", "", 0, 0)
	for _, item := range items {
		isMinable := item.GetBool("minable")

		// 1. Calculate Circulating Supply from players
		// Supply = Sum(Inventory) + Sum(Reserve)
		var invCount float64
		var resCount float64
		_ = l.app.DB().Select("SUM(quantity)").From("inventory").Where(dbx.HashExp{"item": item.Id}).Row(&invCount)
		_ = l.app.DB().Select("SUM(quantity)").From("reserve").Where(dbx.HashExp{"item": item.Id}).Row(&resCount)

		playerSupply := int(invCount + resCount)

		// 2. NPC Companies inject supply (for non-minable items only)
		// Formula: playerCompanyCount × 1000 × random(0.5-1.5)
		npcInjection := 0
		if !isMinable {
			randomFactor := 0.5 + rand.Float64() // 0.5 to 1.5
			npcInjection = int(float64(playerCompanyCount) * 1000 * randomFactor)
		}

		totalSupply := playerSupply + npcInjection
		item.Set("circulating_supply", totalSupply)

		// 3. Market Demand (Random Walk) - only for non-minable
		if !isMinable {
			currentDemand := item.GetInt("market_demand")
			if currentDemand <= 0 {
				currentDemand = 1000
				if totalSupply > 0 {
					currentDemand = totalSupply
				}
			}

			// Change demand slightly (-10% to +10%)
			demandChange := (rand.Float64() - 0.5) * 0.2
			newDemand := int(float64(currentDemand) * (1 + demandChange))
			if newDemand < 10 {
				newDemand = 10
			}
			item.Set("market_demand", newDemand)

			// 4. Price Calculation (Supply/Demand Ratio)
			ratio := float64(newDemand) / float64(totalSupply+1)

			basePrice := item.GetFloat("base_price")
			volatility := item.GetFloat("volatility")
			if volatility <= 0 {
				volatility = 0.05
			}

			pctChange := (ratio - 1.0) * volatility

			// Cap extreme changes per day
			if pctChange > 0.2 {
				pctChange = 0.2
			}
			if pctChange < -0.2 {
				pctChange = -0.2
			}

			newPrice := basePrice * (1 + pctChange)
			if newPrice < 0.1 {
				newPrice = 0.1
			}

			item.Set("base_price", math.Round(newPrice*100)/100)
		}
		// Minable items: price stays fixed, no volatility applied

		l.app.Save(item)
	}

	l.app.Logger().Info("[CRON] Market prices updated",
		"player_companies", playerCompanyCount,
		"total_employees", totalEmployees)
}

func (l *EconomyLogic) UpdateStockPrices() {
	stocks, _ := l.app.FindRecordsByFilter("stocks", "", "", 0, 0)
	for _, stock := range stocks {
		currentPrice := stock.GetFloat("share_price")
		if currentPrice == 0 {
			currentPrice = 10
		}

		change := (rand.Float64() - 0.5) * 0.1
		newPrice := currentPrice * (1 + change)

		if newPrice < 0.1 {
			newPrice = 0.1
		}

		stock.Set("share_price", newPrice)
		l.app.Save(stock)
	}
}

func (l *EconomyLogic) DeductDailyPayroll() {
	companies, _ := l.app.FindRecordsByFilter("companies", "", "", 0, 0)
	for _, company := range companies {
		employees, _ := l.app.FindRecordsByFilter("employees", fmt.Sprintf("employer = '%s'", company.Id), "", 0, 0)
		machines, _ := l.app.FindRecordsByFilter("machines", fmt.Sprintf("company = '%s'", company.Id), "", 0, 0)

		monthlyCost := 0.0

		for _, emp := range employees {
			monthlyCost += float64(emp.GetInt("salary") * 30)
		}

		monthlyCost += float64(len(machines) * 7)

		currentBalance := company.GetFloat("balance")
		newBalance := currentBalance - monthlyCost
		if newBalance < 0 {
			newBalance = 0
		}

		company.Set("balance", newBalance)
		l.app.Save(company)

		if monthlyCost > 0 {
			l.app.Logger().Info("[PAYROLL] Deducted", "company", company.GetString("name"), "amount", monthlyCost, "employees", len(employees), "machines", len(machines))
		}
	}
}

func (l *EconomyLogic) SellReserveItems() {
	reserves, _ := l.app.FindRecordsByFilter("reserve", "", "", 0, 0)
	if len(reserves) == 0 {
		return
	}

	for _, res := range reserves {
		companyId := res.GetString("company")
		itemId := res.GetString("item")
		qty := res.GetInt("quantity")

		if qty <= 0 {
			continue
		}

		item, err := l.app.FindRecordById("items", itemId)
		if err != nil {
			continue
		}

		price := item.GetFloat("base_price")
		revenue := float64(qty) * price

		company, err := l.app.FindRecordById("companies", companyId)
		if err != nil {
			continue
		}

		// Update Company Balance
		newBalance := company.GetFloat("balance") + revenue
		company.Set("balance", newBalance)
		l.app.Save(company)

		// Delete Reserve Record (Emptying it)
		// Or Set to 0 if we want to keep the slot?
		// User said "Only the quantity decreases".
		// Deleting is cleaner for the DB.
		l.app.Delete(res)

		l.app.Logger().Info("[CRON] Sold Reserve Item", "company", company.GetString("name"), "item", item.GetString("name"), "qty", qty, "revenue", revenue)
	}
}
