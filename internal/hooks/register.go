package hooks

import (
	"ketsuna.com/server/internal/gamedata"

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
	registerDepositHooks(app)
	registerRecipeHooks(app)
	registerMachineHooks(app, inv, graph)
	RegisterEdgeRelationHooks(app) // Sync edges with machine/deposit fields
	RegisterExplorationCron(app)

	// Run data correction and initialization on startup
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {

		app.Logger().Info("[STARTUP] Running initialization...")

		// Migrate existing company_techs to have status="completed"
		MigrateTechStatusOnStartup(app)

		// Ensure all companies have wood deposit and CEO
		InitializeCompaniesOnStartup(app)

		// Clean up orphaned records (items that no longer exist in gamedata)
		CleanupOrphanedRecords(app)

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

// CleanupOrphanedRecords removes records from machines, inventory, and deposits
// that reference items/resources that no longer exist in gamedata.Items
func CleanupOrphanedRecords(app *pocketbase.PocketBase) {
	app.Logger().Info("[CLEANUP] Checking for orphaned records...")

	deletedMachines := 0
	deletedInventory := 0
	deletedDeposits := 0

	// Clean up machines with invalid machine_id
	machines, err := app.FindAllRecords("machines")
	if err == nil {
		for _, record := range machines {
			machineID := record.GetString("machine_id")
			if machineID != "" && gamedata.GetItem(machineID) == nil {
				app.Logger().Warn("[CLEANUP] Deleting orphaned machine",
					"record_id", record.Id,
					"machine_id", machineID,
				)
				if err := app.Delete(record); err == nil {
					deletedMachines++
				}
			}
		}
	}

	// Clean up inventory with invalid item_id
	inventory, err := app.FindAllRecords("inventory")
	if err == nil {
		for _, record := range inventory {
			itemID := record.GetString("item_id")
			if itemID != "" && gamedata.GetItem(itemID) == nil {
				app.Logger().Warn("[CLEANUP] Deleting orphaned inventory",
					"record_id", record.Id,
					"item_id", itemID,
				)
				if err := app.Delete(record); err == nil {
					deletedInventory++
				}
			}
		}
	}

	// Clean up deposits with invalid ressource_id
	deposits, err := app.FindAllRecords("deposits")
	if err == nil {
		for _, record := range deposits {
			ressourceID := record.GetString("ressource_id")
			if ressourceID != "" && gamedata.GetItem(ressourceID) == nil {
				app.Logger().Warn("[CLEANUP] Deleting orphaned deposit",
					"record_id", record.Id,
					"ressource_id", ressourceID,
				)
				if err := app.Delete(record); err == nil {
					deletedDeposits++
				}
			}
		}
	}

	if deletedMachines > 0 || deletedInventory > 0 || deletedDeposits > 0 {
		app.Logger().Info("[CLEANUP] Orphaned records removed",
			"machines", deletedMachines,
			"inventory", deletedInventory,
			"deposits", deletedDeposits,
		)
	} else {
		app.Logger().Info("[CLEANUP] No orphaned records found")
	}
}
