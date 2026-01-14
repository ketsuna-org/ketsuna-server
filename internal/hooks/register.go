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
	registerInventoryHooks(app)
	registerDepositHooks(app)
	registerRecipeHooks(app)
	registerMachineHooks(app, inv, graph)
	RegisterEdgeRelationHooks(app) // Sync edges with machine/deposit fields
	RegisterExplorationCron(app)

	// Run data correction and initialization on startup
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {

		app.Logger().Debug("[STARTUP] Running initialization...")

		// Backfill base assets for existing companies (wood deposit, CEO, forestry machine)
		InitializeCompaniesOnStartup(app)

		// Migrate existing company_techs to have status="completed"
		MigrateTechStatusOnStartup(app)

		// Enforce constraint: Remove duplicate deposit -> machine edges
		CleanupDuplicateDepositEdges(app)

		return e.Next()
	})

	app.Cron().Add("daily_payroll_market", "0 6 * * *", func() {
		app.Logger().Debug("[CRON] Executing Daily Payroll & Market Update (06:00 UTC)")
		eco.DeductDailyPayroll()
	})

	// LAZY CALCULATION: Replaced Cron with On-Demand API
	// The frontend detects activity (user login/interaction) and calls this endpoint.
	// It calculates elapsed time since last run for each edge, functioning exactly like the cron but only when needed.
	edgeTransfer := NewEdgeTransferCron(app)
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		e.Router.POST("/api/factory/process", func(e *core.RequestEvent) error {
			var producedItems map[string]float64

			// 1. FIRST: Trigger production calculation (fills machine output buffers)
			// This makes machines actually produce items based on time elapsed
			if e.Auth != nil {
				companyId := e.Auth.GetString("active_company")
				if companyId != "" {
					var err error
					producedItems, err = graph.CalculateCompanyInventory(companyId)
					if err != nil {
						app.Logger().Error("[LAZY] Production calculation failed", "company", companyId, "err", err)
					} else {
						app.Logger().Debug("[LAZY] Production completed", "company", companyId, "items", len(producedItems))
					}
				}
			}

			// 2. THEN: Run edge transfer logic (moves items from buffers along edges)
			// Buffers are now filled by production, so transfer can move items
			if err := edgeTransfer.TransferAll(); err != nil {
				app.Logger().Error("[LAZY] Edge transfer failed", "err", err)
				return e.BadRequestError("Transfer failed", err)
			}

			return e.JSON(200, map[string]interface{}{
				"status":        "ok",
				"producedItems": producedItems,
			})
		})
		return e.Next()
	})

	// Start Background Jobs
	// startEconomyTicker(app, ecoLogic)
}
