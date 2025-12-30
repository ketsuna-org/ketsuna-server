package hooks

import (
	"fmt"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func registerMachineHooks(app *pocketbase.PocketBase, inv *InventoryLogic) {
	app.OnRecordCreateRequest("machines").BindFunc(func(e *core.RecordRequestEvent) error {
		record := e.Record
		companyId := record.GetString("company")
		machineItemId := record.GetString("machine")
		employeeIds := record.GetStringSlice("employees")

		if companyId == "" || machineItemId == "" {
			return apis.NewBadRequestError("ID de compagnie ou de machine manquant.", nil)
		}

		// 1. Validate employees not assigned elsewhere
		if len(employeeIds) > 0 {
			// Find other machines with these employees
			// filter usage: employees ~ 'id' || employees ~ 'id2'
			// This is complex to build in simple string, loop
			for _, empId := range employeeIds {
				found, err := app.FindRecordsByFilter("machines", fmt.Sprintf("employees ~ '%s'", empId), "", 1, 0)
				if err == nil && len(found) > 0 {
					return apis.NewBadRequestError("Un ou plusieurs employés sont déjà assignés à une autre machine.", nil)
				}
			}
		}

		// 2. Check Inventory Stock
		if !inv.HasEnoughItems(companyId, machineItemId, 1) {
			return apis.NewBadRequestError("Vous n'avez pas cette machine en stock dans votre inventaire.", nil)
		}

		// 3. Deduct from Inventory
		if err := inv.UpdateInventory(companyId, machineItemId, -1); err != nil {
			return apis.NewBadRequestError(fmt.Sprintf("Erreur lors de la mise à jour de l'inventaire: %v", err), nil)
		}

		app.Logger().Info("[MACHINES] Machine assigned", "machineId", machineItemId, "companyId", companyId)
		return nil
	})

	app.OnRecordUpdateRequest("machines").BindFunc(func(e *core.RecordRequestEvent) error {
		record := e.Record
		employeeIds := record.GetStringSlice("employees")

		if len(employeeIds) > 0 {
			// Check if any employee is already assigned to ANOTHER machine
			for _, empId := range employeeIds {
				// We search for ANY machine containing this employee
				found, err := app.FindRecordsByFilter("machines", fmt.Sprintf("employees ~ '%s'", empId), "", 10, 0)
				if err == nil {
					for _, m := range found {
						if m.Id != record.Id {
							app.Logger().Error("[MACHINES] Validation failed: Employee already busy", "empId", empId, "otherMachine", m.Id)
							return apis.NewBadRequestError("Un ou plusieurs employés sont déjà assignés à une autre machine.", nil)
						}
					}
				}
			}
		}
		return nil
	})

	app.OnRecordDeleteRequest("machines").BindFunc(func(e *core.RecordRequestEvent) error {
		record := e.Record
		companyId := record.GetString("company")
		machineItemId := record.GetString("machine")

		if companyId != "" && machineItemId != "" {
			if err := inv.UpdateInventory(companyId, machineItemId, 1); err != nil {
				app.Logger().Error("[MACHINES] Erreur remise en stock", "error", err)
			} else {
				app.Logger().Info("[MACHINES] Assignation supprimée. Machine renvoyée au stock", "machineId", machineItemId)
			}
		}
		return nil
	})
}
