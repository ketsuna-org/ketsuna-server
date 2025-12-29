package hooks

import (
	"math/rand"
	"time"

	"github.com/pocketbase/pocketbase"
)

// package-local RNG (preferred over global rand.Seed as of Go 1.20)
var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

// RegisterHooks registers a subset of game hooks (companies, employees, inventory, recipes)
// and starts a simple economy ticker (cron-like) to simulate the JS hooks behavior.
func RegisterHooks(app *pocketbase.PocketBase) {
	invLogic := NewInventoryLogic(app)
	ecoLogic := NewEconomyLogic(app, invLogic)

	registerCompanyHooks(app)
	registerEmployeeHooks(app)
	registerCompanyTechHooks(app)
	registerStockHooks(app)
	registerShareholderHooks(app)
	registerInventoryHooks(app)
	registerRecipeHooks(app)
	registerMachineHooks(app, invLogic)

	// Register API Endpoints
	RegisterEndpoints(app, invLogic, ecoLogic)

	// Start Background Jobs
	startEconomyTicker(app, ecoLogic)
}

func startEconomyTicker(app *pocketbase.PocketBase, eco *EconomyLogic) {
	// 1. Fast Ticker (Sub-minute tasks) - Keep using Ticker
	// User set this to 20s
	go func() {
		t := time.NewTicker(20 * time.Second)
		defer t.Stop()
		for range t.C {
			companies, err := app.FindRecordsByFilter("companies", "", "", 0, 0)
			if err == nil {
				for _, c := range companies {
					eco.ProcessCompanyEconomy(c.Id)
				}
			}
			eco.UpdateStockPrices()
		}
	}()

	// 2. Daily Cron (Tasks > 1h) - Use PocketBase Cron
	// Runs at 06:00 UTC every day
	app.Cron().Add("daily_payroll_market", "0 6 * * *", func() {
		app.Logger().Info("[CRON] Executing Daily Payroll & Market Update (06:00 UTC)")
		eco.UpdateMarketPrices()
		eco.DeductDailyPayroll()
	})
}
