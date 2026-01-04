package hooks

import (
	"fmt"
	"math"

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

		// If inventory exists, return error
		existing, _ := e.App.FindFirstRecordByFilter("inventory", fmt.Sprintf("company='%s' && item='%s'", companyId, itemId))
		if existing != nil {
			return apis.NewBadRequestError("Item already in inventory, use update API to add quantity", nil)
		}

		// Deduct money for new item if CEO
		if e.Auth != nil && e.Auth.Id == company.GetString("ceo") {
			itemPrice := item.GetFloat("base_price") // Changed to float
			totalCost := itemPrice * float64(qty)
			current := company.GetFloat("balance") // Changed to float to match schema (number)
			// Note: Schema says balance is 'number', so using GetFloat.

			if current < totalCost {
				return apis.NewBadRequestError(fmt.Sprintf("Fonds insuffisants. Coût: %.2f€, Solde: %.2f€", totalCost, current), nil)
			}

			// --- MARKET STOCK CHECK ---
			// market_demand used as Market Stock
			stock := item.GetInt("market_demand")
			// Only check stock for machines/non-minables if that's the rule, or ALL items?
			// User said "Auto-refill stock... limited to 10".
			// If minable items (Iron) depend on players selling, then Stock applies to them too.
			// So we check stock for everyone.
			if stock < qty {
				return apis.NewBadRequestError(fmt.Sprintf("Stock insuffisant sur le marché. Disponible: %d", stock), nil)
			}

			// Update Market Stock
			item.Set("market_demand", stock-qty)

			// Update Price (Buy -> Price Goes UP)
			// "I buy 5 machines, Price goes up !!"
			// Factor: 0.5% per unit?
			priceFactor := 0.005
			newPrice := itemPrice * (1 + float64(qty)*priceFactor)
			item.Set("base_price", math.Round(newPrice*100)/100)

			if err := e.App.Save(item); err != nil {
				return apis.NewBadRequestError("Erreur mise à jour marché", err)
			}

			// Update Company Balance
			company.Set("balance", current-totalCost)
			if err := e.App.Save(company); err != nil {
				e.App.Logger().Error("[PURCHASE] erreur save company", "error", err)
			}
			e.App.Logger().Info("[PURCHASE] Company purchased item", "companyId", companyId, "itemId", itemId, "qty", qty, "totalCost", totalCost)
		}

		return e.Next()
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
			price := item.GetFloat("base_price") // Use float
			totalCost := price * float64(added)
			bal := company.GetFloat("balance") // Use float

			if bal < totalCost {
				return apis.NewBadRequestError(fmt.Sprintf("Fonds insuffisants pour acheter %d items. Coût: %.2f€, Solde: %.2f€", added, totalCost, bal), nil)
			}

			// --- MARKET STOCK CHECK ---
			stock := item.GetInt("market_demand")
			if stock < added {
				return apis.NewBadRequestError(fmt.Sprintf("Stock insuffisant sur le marché. Disponible: %d", stock), nil)
			}

			// Update Market Stock
			item.Set("market_demand", stock-added)

			// Update Price (Buy -> Price Goes UP)
			priceFactor := 0.005
			newPrice := price * (1 + float64(added)*priceFactor)
			item.Set("base_price", math.Round(newPrice*100)/100)

			if err := e.App.Save(item); err != nil {
				return apis.NewBadRequestError("Erreur mise à jour marché", err)
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

		return e.Next()
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
