package hooks

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/pocketbase/dbx"
)

// EconomyLogic struct is defined in economy_logic.go

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
		1000,
		0,
	)
	if err != nil {
		return err
	}

	if len(assignedMachines) == 0 {
		return nil
	}

	// Expand 'machine' item relation for performance
	for _, m := range assignedMachines {
		l.app.ExpandRecord(m, []string{"machine"}, nil)
	}

	// Pre-fetch employees for efficiency calculation
	employees, err := l.app.FindRecordsByFilter(
		"employees",
		fmt.Sprintf("employer = '%s'", companyId),
		"",
		1000,
		0,
	)
	if err != nil {
		return err
	}

	// Track energy updates for storage machines
	energySurplus := energyStatus.EnergyProduced - energyStatus.EnergyDemand

	// We'll do two passes for energy storage:
	// 1. Machines of type "Machine" (Internal buffers)
	// 2. Machines of type "Stockage" (Dedicated batteries)

	distributeEnergy := func(targetType string) {
		if energySurplus == 0 {
			return
		}
		for _, assignment := range assignedMachines {
			machineItem := assignment.ExpandedOne("machine")
			if machineItem == nil {
				continue
			}

			if machineItem.GetString("type") != targetType {
				continue
			}

			canStoreEnergy := machineItem.GetFloat("can_store_energy")
			if canStoreEnergy > 0 {
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
					energySurplus -= (newStored - currentStored)
				}
			}
			if energySurplus == 0 {
				break
			}
		}
	}

	distributeEnergy("Machine")
	distributeEnergy("Stockage")

	// Now process machine logic (Production, etc.)
	for _, assignment := range assignedMachines {
		err = l.ProcessMachine(companyId, assignment, energyStatus, employees)
		if err != nil {
			// l.app.Logger().Error("Error processing machine", "id", assignment.Id, "err", err)
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

		// 1. Calculate Circulating Supply (Just for stats)
		var invCount float64
		var resCount float64
		_ = l.app.DB().Select("SUM(quantity)").From("inventory").Where(dbx.HashExp{"item": item.Id}).Row(&invCount)
		_ = l.app.DB().Select("SUM(quantity)").From("reserve").Where(dbx.HashExp{"item": item.Id}).Row(&resCount)
		playerSupply := int(invCount + resCount)
		item.Set("circulating_supply", playerSupply)

		// 2. Daily Stock Refill (Only for non-minable items e.g. Machines)
		// User Rule: "limited to 10 Units per player... limited by day"
		// We use 'market_demand' field to store the Available Market Stock.
		if !isMinable {
			baseRefill := float64(playerCompanyCount * 10)

			// Add Variance: + 5% to 10%
			// "100 machines + 5 / 10%"
			variance := baseRefill * (0.05 + rand.Float64()*0.05) // 0.05 to 0.10

			newStock := int(math.Round(baseRefill + variance))

			// Set the Market Stock (using market_demand field)
			item.Set("market_demand", newStock)
		}

		// 3. Remove Volatility & Random Price Changes
		// Prices are now updated strictly on Buy/Sell actions in inventory hooks.

		l.app.Save(item)
	}

	l.app.Logger().Info("[CRON] Market prices updated",
		"player_companies", playerCompanyCount,
		"total_employees", totalEmployees)
}

func (l *EconomyLogic) UpdateStockPrices() {
	// Volatility removed. Prices update only on transactions (if implemented) or stay static.
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
