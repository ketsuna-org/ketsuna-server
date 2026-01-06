package hooks

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/pocketbase/dbx"
)

// cron_jobs.go - Daily scheduled tasks (Payroll & Market updates)
// Note: Old economy ticker logic was removed in favor of graph_economy.go

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
