package hooks

import (
	"fmt"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"ketsuna.com/server/internal/gamedata"
)

func registerInventoryHooks(app *pocketbase.PocketBase) {
	app.OnRecordCreateRequest("inventory").BindFunc(func(e *core.RecordRequestEvent) error {

		// Allow Admin/Superuser to bypass all checks and logic
		if e.Auth != nil && e.Auth.IsSuperuser() {
			return e.Next()
		}

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

		// Use static Gamedata
		item := gamedata.GetItem(itemId)
		if item == nil {
			return apis.NewBadRequestError("Item introuvable", nil)
		}

		if !item.MarketAvailable {
			return apis.NewBadRequestError("Cet item n'est pas disponible sur le marché", nil)
		}

		// If inventory exists, return error
		existing, _ := e.App.FindFirstRecordByFilter("inventory", fmt.Sprintf("company='%s' && item='%s'", companyId, itemId))
		if existing != nil {
			return apis.NewBadRequestError("Item already in inventory, use update API to add quantity", nil)
		}

		// Deduct money for new item if CEO
		if e.Auth != nil && e.Auth.Id == company.GetString("ceo") {
			itemPrice := item.BasePrice
			totalCost := itemPrice * float64(qty)
			current := company.GetFloat("balance")

			if current < totalCost {
				return apis.NewBadRequestError(fmt.Sprintf("Fonds insuffisants. Coût: %.2f€, Solde: %.2f€", totalCost, current), nil)
			}

			// --- NO MARKET STOCK LOGIC FOR STATIC ITEMS ---
			// Since items are static code, we cannot track global market stock in them.
			// Ideally this should be moved to a separate 'market_state' collection if dynamic stock is needed.
			// For now, we assume infinite stock but respect MarketAvailable.

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
		if e.Auth != nil && e.Auth.IsSuperuser() {
			return e.Next()
		}
		orig := rec.Original()
		if rec.GetString("item") != orig.GetString("item") || rec.GetString("company") != orig.GetString("company") {
			return apis.NewBadRequestError("Action illégale : Impossible de transférer cet inventaire.", nil)
		}

		company, err := e.App.FindRecordById("companies", rec.GetString("company"))
		if err != nil {
			return apis.NewBadRequestError("Company introuvable", nil)
		}
		companyAuthId := company.GetString("ceo")

		// Allow Admin/Superuser
		isSuperUser := e.Auth != nil && e.Auth.IsSuperuser()

		if !isSuperUser && (e.Auth == nil || e.Auth.Id != companyAuthId) {
			return apis.NewForbiddenError("Seul le CEO peut modifier l'inventaire", nil)
		}

		// Skip Purchase Logic/Cost deduction for Superusers
		if isSuperUser {
			return e.Next()
		}

		oldQ := orig.GetInt("quantity")
		newQ := rec.GetInt("quantity")
		if newQ > oldQ {
			added := newQ - oldQ
			// Use static Gamedata
			item := gamedata.GetItem(rec.GetString("item"))
			if item == nil {
				return apis.NewBadRequestError("Item introuvable", nil) // Should not happen for existing inv
			}

			if !item.MarketAvailable {
				return apis.NewBadRequestError("Cet item n'est pas disponible sur le marché", nil)
			}

			price := item.BasePrice
			totalCost := price * float64(added)
			bal := company.GetFloat("balance")

			if bal < totalCost {
				return apis.NewBadRequestError(fmt.Sprintf("Fonds insuffisants pour acheter %d items. Coût: %.2f€, Solde: %.2f€", added, totalCost, bal), nil)
			}

			// --- NO MARKET STOCK LOGIC FOR STATIC ITEMS ---

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
