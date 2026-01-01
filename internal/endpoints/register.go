package endpoints

import (
	"ketsuna.com/server/internal/hooks"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// RegisterAll registers all custom API endpoints
func RegisterAll(app *pocketbase.PocketBase, inv *hooks.InventoryLogic, eco *hooks.EconomyLogic, emp *hooks.EmployeeLogic) {
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// Register domain-specific endpoints
		registerWorkshopEndpoints(app, e, inv)
		registerInventoryEndpoints(app, e, inv)
		registerCompanyEndpoints(app, e, eco)
		registerEmployeesEndpoints(app, e, emp)
		registerReserveEndpoints(app, e)
		registerHarvestEndpoints(app, e, inv)
		registerMachineEndpoints(app, e)

		return e.Next()
	})
}
