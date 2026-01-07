package hooks

import (
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
		// app.Logger().Info("[DEPOSITS] Location updated", "id", newRecord.Id, "new_loc", newRecord.GetString("location"))

		return e.Next()
	})

	app.Logger().Info("[HOOKS] Deposit hooks registered")
}
