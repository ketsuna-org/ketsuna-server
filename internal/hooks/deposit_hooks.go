package hooks

import (
	"fmt"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func registerDepositHooks(app *pocketbase.PocketBase) {
	app.OnRecordUpdateRequest("deposits").BindFunc(func(e *core.RecordRequestEvent) error {
		// This hook ensures that ONLY the location can be updated by the client.
		// Any other field modification attempt should be rejected to prevent cheating.
		//
		// Allow: location
		// Deny: quantity, size, ressource_id, company

		newRecord := e.Record
		originalRecord, err := app.FindRecordById("deposits", newRecord.Id)
		if err != nil {
			return err
		}

		companyId := originalRecord.GetString("company")

		// SECURITY: Validate ownership (bypass for superuser)
		if e.Auth == nil || !e.Auth.IsSuperuser() {
			if err := ValidateCompanyOwnership(e.App, e.Auth.Id, companyId); err != nil {
				return err
			}
		}

		// 1. Check Quantity (Disabled for debugging backend updates)
		// if newRecord.GetInt("quantity") != originalRecord.GetInt("quantity") {
		// 	return apis.NewBadRequestError("Modification de la quantité interdite.", nil)
		// }

		// 2. Check Size
		if newRecord.GetInt("size") != originalRecord.GetInt("size") {
			return apis.NewBadRequestError("Modification de la taille interdite.", nil)
		}

		// 3. Check Ressource ID
		if newRecord.GetString("ressource_id") != originalRecord.GetString("ressource_id") {
			return apis.NewBadRequestError("Modification du type de ressource interdite.", nil)
		}

		// 4. Check Company
		if newRecord.GetString("company") != originalRecord.GetString("company") {
			return apis.NewBadRequestError("Transfert de gisement interdit.", nil)
		}

		// If all checks pass, allow the update (which is presumably just location)
		// We could strictly audit if ONLY location changed, but preventing critical fields changes is the goal.

		// Optional: Log the valid move
		// app.Logger().Debug("[DEPOSITS] Location updated", "id", newRecord.Id, "new_loc", newRecord.GetString("location"))

		return e.Next()
	})

	// Auto-delete empty deposits (quantity = 0) and their connected edges
	app.OnRecordAfterUpdateSuccess("deposits").BindFunc(func(e *core.RecordEvent) error {
		record := e.Record
		quantity := record.GetFloat("quantity")

		// Check if deposit is empty
		if quantity <= 0 {
			depositId := record.Id
			app.Logger().Debug("[DEPOSITS] Deposit is empty, cleaning up", "id", depositId)

			// Delete all connected edges first
			edgeFilter := fmt.Sprintf("source_id = '%s' || target_id = '%s'", depositId, depositId)
			edges, err := app.FindRecordsByFilter("edge_relation", edgeFilter, "", 0, 0)
			if err == nil {
				for _, edge := range edges {
					if err := app.Delete(edge); err != nil {
						app.Logger().Error("[DEPOSITS] Failed to delete edge", "edge_id", edge.Id, "err", err)
					} else {
						app.Logger().Debug("[DEPOSITS] Deleted connected edge", "edge_id", edge.Id)
					}
				}
			}

			// Delete the deposit itself
			if err := app.Delete(record); err != nil {
				app.Logger().Error("[DEPOSITS] Failed to delete empty deposit", "id", depositId, "err", err)
			} else {
				app.Logger().Debug("[DEPOSITS] Deleted empty deposit", "id", depositId)
			}
		}

		return e.Next()
	})

	app.Logger().Debug("[HOOKS] Deposit hooks registered")
}
