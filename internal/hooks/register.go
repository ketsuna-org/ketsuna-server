package hooks

import (
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// package-local RNG (preferred over global rand.Seed as of Go 1.20)
// var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

// GetLogicHandlers returns the logic handlers for use by the endpoints package
func GetLogicHandlers(app *pocketbase.PocketBase) (*InventoryLogic, *EconomyLogic, *EmployeeLogic) {
	invLogic := NewInventoryLogic(app)
	ecoLogic := NewEconomyLogic(app, invLogic)
	empLogic := NewEmployeeLogic(app)
	return invLogic, ecoLogic, empLogic
}

// RegisterHooks registers a subset of game hooks (companies, employees, inventory, recipes)
// and starts a simple economy ticker (cron-like) to simulate the JS hooks behavior.
// NOTE: Endpoints are now registered separately via endpoints.RegisterAll()
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
	RegisterExplorationCron(app)

	// Run data correction and initialization on startup
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		EnforceMaxEmployees(app)
		InitializeCompaniesOnStartup(app) // Create CEO + wood deposit for companies
		PurgeEmptyDeposits(app)           // Clean up empty deposits
		FixZeroLevelDeposits(app)         // Fix deposits with level 0
		EnforceDepositCapacity(app)       // Clean up surplus assignments
		SoftCleanWAL(app)                 // Perform initial WAL checkpoint

		app.Logger().Info("[STARTUP] Running initial Market Supply Update...")
		ecoLogic.UpdateMarketPrices()

		return e.Next()
	})

	// Start Background Jobs
	startEconomyTicker(app, ecoLogic)
}

func startEconomyTicker(app *pocketbase.PocketBase, eco *EconomyLogic) {
	// 1. Fast Ticker (Sub-minute tasks) - Keep using Ticker
	// User set this to 1s
	go func() {
		t := time.NewTicker(1 * time.Second)
		defer t.Stop()
		for range t.C {
			companies, err := app.FindRecordsByFilter("companies", "", "", 0, 0)
			if err == nil {
				for _, c := range companies {
					eco.ProcessCompanyEconomy(c.Id)
					eco.ProcessDepositHarvesting(c.Id) // Process employee-based deposit harvesting
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
		SoftCleanWAL(app) // Periodic WAL checkpoint
	})
}
