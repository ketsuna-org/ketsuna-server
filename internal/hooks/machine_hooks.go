package hooks

import (
	"fmt"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"ketsuna.com/server/internal/gamedata"
)

func registerMachineHooks(app *pocketbase.PocketBase, inv *InventoryLogic, _ *GraphEconomy) {

	app.OnRecordCreateRequest("machines").BindFunc(func(e *core.RecordRequestEvent) error {
		record := e.Record
		companyId := record.GetString("company")
		machineItemId := record.GetString("machine_id")

		if companyId == "" || machineItemId == "" {
			return apis.NewBadRequestError("ID de compagnie ou de machine manquant.", nil)
		}

		// 1. Validate employees not assigned elsewhere
		// REMOVED: Employees are now assigned via employee record update, not machine record.
		// The endpoints/machines.go handles assignment validation.

		// 2. Check max_employee limit using static gamedata
		// REMOVED: Same reason.

		// 3. DEPOSIT ASSIGNMENT VIA EDGES
		// Machines are now connected to deposits via edge_relation only, not via machine.deposit field.
		// Edge constraints are validated in edge_hooks.go

		// 4. SET INITIAL DURABILITY AND START PRODUCTION (skip for storage)
		// Storage items don't have durability or production, they just store
		machineItem := gamedata.GetItem(machineItemId)
		if machineItem != nil && machineItem.Type != gamedata.ItemTypeStockage {
			// Initialize production timestamp so machines start producing immediately
			record.Set("production_started_at", time.Now())
		}

		// 5. ATOMIC: Check and Deduct Inventory using transaction
		// This prevents race conditions when multiple machines are created simultaneously
		var deductErr error
		txErr := app.RunInTransaction(func(txApp core.App) error {
			// Re-fetch inventory inside transaction for fresh data (using item_id)
			inventory, err := txApp.FindFirstRecordByFilter("inventory",
				fmt.Sprintf("company = '%s' && item_id = '%s'", companyId, machineItemId))
			if err != nil {
				deductErr = apis.NewBadRequestError("Vous n'avez pas cette machine en stock dans votre inventaire.", nil)
				return deductErr
			}

			currentQty := inventory.GetInt("quantity")
			if currentQty < 1 {
				deductErr = apis.NewBadRequestError("Stock insuffisant: vous n'avez plus cette machine en inventaire.", nil)
				return deductErr
			}

			// Deduct within transaction
			inventory.Set("quantity", currentQty-1)
			if err := txApp.Save(inventory); err != nil {
				deductErr = apis.NewBadRequestError("Erreur lors de la mise à jour de l'inventaire.", nil)
				return deductErr
			}

			app.Logger().Debug("[MACHINES] Machine assigned (tx)", "machineId", machineItemId, "companyId", companyId, "remaining", currentQty-1)
			return nil
		})

		if txErr != nil || deductErr != nil {
			if deductErr != nil {
				return deductErr
			}
			return apis.NewBadRequestError("Erreur de transaction", txErr)
		}

		return e.Next()
	})

	// Hook to auto-start production when machine is placed
	app.OnRecordAfterUpdateSuccess("machines").BindFunc(func(e *core.RecordEvent) error {
		record := e.Record

		// Check if machine was just placed (placed changed to true)
		if record.GetBool("placed") {
			// Check if production_started_at is not set
			startedAt := record.GetDateTime("production_started_at")
			if startedAt.Time().IsZero() {
				// Skip storage items - they don't need production
				machineItemId := record.GetString("machine_id")
				machineItem := gamedata.GetItem(machineItemId)

				if machineItem != nil && machineItem.Type != gamedata.ItemTypeStockage {
					// Initialize production timestamp
					record.Set("production_started_at", time.Now())
					if err := app.Save(record); err != nil {
						app.Logger().Error("[MACHINES] Failed to auto-start production", "err", err)
					} else {
						app.Logger().Debug("[MACHINES] Auto-started production for placed machine", "machineId", record.Id)
					}
				}
			}
		}

		return e.Next()
	})

	// SECURITY: Validate ownership before update
	app.OnRecordUpdateRequest("machines").BindFunc(func(e *core.RecordRequestEvent) error {
		record := e.Record
		original := record.Original()

		companyId := original.GetString("company")

		// Prevent transferring machines between companies
		if record.GetString("company") != companyId {
			return apis.NewBadRequestError("Transfert de machine interdit", nil)
		}

		// Validate ownership (bypass for superuser)
		if e.Auth == nil || !e.Auth.IsSuperuser() {
			if err := ValidateCompanyOwnership(e.App, e.Auth.Id, companyId); err != nil {
				return err
			}
		}

		// Protect critical fields from modification
		record.Set("machine_id", original.GetString("machine_id"))
		record.Set("company", original.GetString("company"))

		return e.Next()
	})

	app.OnRecordDeleteRequest("machines").BindFunc(func(e *core.RecordRequestEvent) error {
		record := e.Record
		companyId := record.GetString("company")

		// SECURITY: Validate ownership (bypass for superuser)
		if e.Auth == nil || !e.Auth.IsSuperuser() {
			if err := ValidateCompanyOwnership(e.App, e.Auth.Id, companyId); err != nil {
				return err
			}
		}

		machineItemId := record.GetString("machine_id")
		if companyId != "" && machineItemId != "" {
			if err := inv.UpdateInventory(app, companyId, machineItemId, 1); err != nil {
				app.Logger().Error("[MACHINES] Erreur remise en stock", "error", err)
			} else {
				app.Logger().Debug("[MACHINES] Assignation supprimée. Machine renvoyée au stock", "machineId", machineItemId)
			}
		}
		return e.Next()
	})
}

// EnforceMaxEmployees and AutoAssignDeposits have been removed.
// Employees are no longer used for deposit mining - they are only used in explorations.
// Deposit-to-machine connections now use edge_relation exclusively.
