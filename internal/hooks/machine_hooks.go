package hooks

import (
	"fmt"
	"log"

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

		log.Printf("[MACHINES] Machine %s assignée pour company %s. Stock déduit.\n", machineItemId, companyId)
		return nil
	})

	app.OnRecordUpdateRequest("machines").BindFunc(func(e *core.RecordRequestEvent) error {
		record := e.Record
		employeeIds := record.GetStringSlice("employees")

		if len(employeeIds) > 0 {
			for _, empId := range employeeIds {
				// exclude current machine
				filter := fmt.Sprintf("id != '%s' && employees ~ '%s'", record.Id, empId)
				found, err := app.FindRecordsByFilter("machines", filter, "", 1, 0)
				if err == nil && len(found) > 0 {
					return apis.NewBadRequestError("Un ou plusieurs employés sont déjà assignés à une autre machine.", nil)
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
				log.Printf("[MACHINES] Erreur remise en stock: %v\n", err)
			} else {
				log.Printf("[MACHINES] Assignation supprimée. Machine %s renvoyée au stock.\n", machineItemId)
			}
		}
		return nil
	})
}
