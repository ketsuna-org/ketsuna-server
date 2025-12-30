package hooks

import (
	"fmt"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func registerInventoryHooks(app *pocketbase.PocketBase) {
	app.OnRecordCreateRequest("inventory").BindFunc(func(e *core.RecordRequestEvent) error {

		if e.Auth == nil {
			return apis.NewBadRequestError("You need to be logged in", nil)
		}
		r := e.Record
		companyId := r.GetString("company")
		itemId := r.GetString("item")
		qty := r.GetInt("quantity")
		if companyId == "" || itemId == "" {
			return apis.NewBadRequestError("Company et Item sont requis", nil)
		}
		if qty <= 0 {
			return apis.NewBadRequestError("La quantité doit être supérieure à 0", nil)
		}

		company, err := e.App.FindRecordById("companies", companyId)
		if err != nil {
			return apis.NewBadRequestError("Company introuvable", nil)
		}
		item, err := e.App.FindRecordById("items", itemId)
		if err != nil {
			return apis.NewBadRequestError("Item introuvable", nil)
		}

		// If inventory exists, update quantity and deduct money if CEO
		existing, _ := e.App.FindFirstRecordByFilter("inventory", fmt.Sprintf("company='%s' && item='%s'", companyId, itemId))
		if existing != nil {
			curr := existing.GetInt("quantity")
			existing.Set("quantity", curr+qty)
			if err := e.App.Save(existing); err != nil {
				e.App.Logger().Error("Erreur mise à jour inventaire", "error", err)
				return apis.NewBadRequestError("Erreur mise à jour inventaire", nil)
			}

			// Deduct money if CEO
			if e.Auth != nil && e.Auth.Id == company.GetString("ceo") {
				itemPrice := item.GetInt("base_price")
				totalCost := itemPrice * qty
				current := company.GetInt("balance")
				if current < totalCost {
					// Revert the update
					existing.Set("quantity", curr)
					e.App.Save(existing)
					return apis.NewBadRequestError(fmt.Sprintf("Fonds insuffisants. Coût: %d€, Solde: %d€", totalCost, current), nil)
				}
				company.Set("balance", current-totalCost)
				if err := e.App.Save(company); err != nil {
					// Revert the update
					existing.Set("quantity", curr)
					e.App.Save(existing)
					e.App.Logger().Error("[PURCHASE] erreur save company", "error", err)
				}
				e.App.Logger().Info("[PURCHASE] Company purchased item", "companyId", companyId, "itemId", itemId, "qty", qty, "totalCost", totalCost)
			}

			return apis.NewBadRequestError(fmt.Sprintf("Item déjà en inventaire. Quantité mise à jour: %d", curr+qty), nil)
		}

		// Deduct money for new item if CEO
		if e.Auth != nil && e.Auth.Id == company.GetString("ceo") {
			itemPrice := item.GetInt("base_price")
			totalCost := itemPrice * qty
			current := company.GetInt("balance")
			if current < totalCost {
				return apis.NewBadRequestError(fmt.Sprintf("Fonds insuffisants. Coût: %d€, Solde: %d€", totalCost, current), nil)
			}
			company.Set("balance", current-totalCost)
			if err := e.App.Save(company); err != nil {
				e.App.Logger().Error("[PURCHASE] erreur save company", "error", err)
			}
			e.App.Logger().Info("[PURCHASE] Company purchased item", "companyId", companyId, "itemId", itemId, "qty", qty, "totalCost", totalCost)
		}

		return nil
	})

	app.OnRecordUpdateRequest("inventory").BindFunc(func(e *core.RecordRequestEvent) error {
		rec := e.Record
		orig := rec.Original()
		if rec.GetString("item") != orig.GetString("item") || rec.GetString("company") != orig.GetString("company") {
			return apis.NewBadRequestError("Action illégale : Impossible de transférer cet inventaire.", nil)
		}

		company, err := e.App.FindRecordById("companies", rec.GetString("company"))
		if err != nil {
			return apis.NewBadRequestError("Company introuvable", nil)
		}
		companyAuthId := company.GetString("ceo")
		if e.Auth == nil || e.Auth.Id != companyAuthId {
			return apis.NewForbiddenError("Seul le CEO peut modifier l'inventaire", nil)
		}

		oldQ := orig.GetInt("quantity")
		newQ := rec.GetInt("quantity")
		if newQ > oldQ {
			added := newQ - oldQ
			item, _ := e.App.FindRecordById("items", rec.GetString("item"))
			price := item.GetInt("base_price")
			totalCost := price * added
			bal := company.GetInt("balance")
			if bal < totalCost {
				return apis.NewBadRequestError(fmt.Sprintf("Fonds insuffisants pour acheter %d items. Coût: %d€, Solde: %d€", added, totalCost, bal), nil)
			}
			company.Set("balance", bal-totalCost)
			if err := e.App.Save(company); err != nil {
				e.App.Logger().Error("[PURCHASE-UPDATE] erreur save company", "error", err)
			}
			e.App.Logger().Info("[PURCHASE-UPDATE] Company purchased more items", "companyId", company.Id, "added", added, "totalCost", totalCost)
		}

		if newQ < 0 {
			return apis.NewBadRequestError("La quantité ne peut pas être négative", nil)
		}

		return nil
	})

	app.OnRecordAfterUpdateSuccess("inventory").BindFunc(func(e *core.RecordEvent) error {
		if e.Record.GetInt("quantity") == 0 {
			if err := e.App.Delete(e.Record); err != nil {
				e.App.Logger().Error("Erreur lors de la suppression de l'inventaire", "error", err)
			}
		}
		return nil
	})
}
