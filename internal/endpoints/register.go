package endpoints

import (
	"ketsuna.com/server/internal/hooks"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// RegisterAll registers all custom API endpoints
func RegisterAll(app *pocketbase.PocketBase, inv *hooks.InventoryLogic, eco *hooks.EconomyLogic, emp *hooks.EmployeeLogic, graph *hooks.GraphEconomy) {
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// Register static game data endpoints (public)
		registerGameDataEndpoints(app, e)

		// Register domain-specific endpoints
		registerWorkshopEndpoints(app, e, inv)
		registerInventoryEndpoints(app, e, inv, graph)
		registerCompanyEndpoints(app, e, eco)
		registerEmployeesEndpoints(app, e, emp)
		registerMachineEndpoints(app, e)
		registerExplorationEndpoints(app, e)
		registerDepositEndpoints(app, e) // Endpoints for assigning employees to deposits

		return e.Next()
	})
}
