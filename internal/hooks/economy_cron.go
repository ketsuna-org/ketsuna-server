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

		// Handle production logic (Energy, Recipe, Passive/Deposit)
		err = l.ProcessMachine(companyId, assignment, energyStatus, employees)
		if err != nil {
			// Log but don't stop the loop for other machines
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
