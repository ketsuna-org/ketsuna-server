package hooks

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// package-local RNG (preferred over global rand.Seed as of Go 1.20)
// var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

// GetLogicHandlers returns the logic handlers for use by the endpoints package
func GetLogicHandlers(app *pocketbase.PocketBase) (*InventoryLogic, *EconomyLogic, *EmployeeLogic, *GraphEconomy) {
	invLogic := NewInventoryLogic(app)
	ecoLogic := NewEconomyLogic(app, invLogic)
	empLogic := NewEmployeeLogic(app)
	graphEco := NewGraphEconomy(app)
	return invLogic, ecoLogic, empLogic, graphEco
}

// RegisterHooks registers a subset of game hooks (companies, employees, inventory, recipes)
// and starts a simple economy ticker (cron-like) to simulate the JS hooks behavior.
// NOTE: Endpoints are now registered separately via endpoints.RegisterAll()
// RegisterHooks registers a subset of game hooks (companies, employees, inventory, recipes)
// and starts a simple economy ticker (cron-like) to simulate the JS hooks behavior.
// NOTE: Endpoints are now registered separately via endpoints.RegisterAll()
func RegisterHooks(app *pocketbase.PocketBase, inv *InventoryLogic, eco *EconomyLogic, emp *EmployeeLogic, graph *GraphEconomy) {

	registerCompanyHooks(app)
	registerEmployeeHooks(app)
	registerCompanyTechHooks(app)
	registerStockHooks(app)
	registerShareholderHooks(app)
	registerInventoryHooks(app)
	registerInventoryHooks(app)
	registerRecipeHooks(app)
	registerMachineHooks(app, inv, graph)
	RegisterExplorationCron(app)

	// Run data correction and initialization on startup
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {

		app.Logger().Info("[STARTUP] Running initial Market Supply Update...")

		return e.Next()
	})

	app.Cron().Add("daily_payroll_market", "0 6 * * *", func() {
		app.Logger().Info("[CRON] Executing Daily Payroll & Market Update (06:00 UTC)")
		eco.UpdateMarketPrices()
		eco.DeductDailyPayroll()
		SoftCleanWAL(app) // Periodic WAL checkpoint
	})

	// Start Background Jobs
	// startEconomyTicker(app, ecoLogic)
}
