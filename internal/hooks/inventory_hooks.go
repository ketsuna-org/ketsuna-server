package hooks

import (
	"fmt"
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func registerInventoryHooks(app *pocketbase.PocketBase) {
	app.OnRecordCreateRequest("inventory").BindFunc(func(e *core.RecordRequestEvent) error {
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

		// If auth is CEO -> purchase
		if e.Auth != nil && e.Auth.Id == company.GetString("ceo") {
			itemPrice := item.GetInt("base_price")
			totalCost := itemPrice * qty
			current := company.GetInt("balance")
			if current < totalCost {
				return apis.NewBadRequestError(fmt.Sprintf("Fonds insuffisants. Coût: %d€, Solde: %d€", totalCost, current), nil)
			}
			company.Set("balance", current-totalCost)
			if err := e.App.Save(company); err != nil {
				log.Println("[PURCHASE] erreur save company:", err)
			}
			log.Printf("[PURCHASE] Company %s purchased item %s x%d for %d€\n", companyId, itemId, qty, totalCost)
		}

		// If inventory exists, update quantity and return an error informing about it
		existing, _ := e.App.FindFirstRecordByFilter("inventory", fmt.Sprintf("company='%s' && item='%s'", companyId, itemId))
		if existing != nil {
			curr := existing.GetInt("quantity")
			existing.Set("quantity", curr+qty)
			if err := e.App.Save(existing); err != nil {
				log.Println("Erreur mise à jour inventaire:", err)
			}
			return apis.NewBadRequestError(fmt.Sprintf("Item déjà en inventaire. Quantité mise à jour: %d", curr+qty), nil)
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
				log.Println("[PURCHASE-UPDATE] erreur save company:", err)
			}
			log.Printf("[PURCHASE-UPDATE] Company %s purchased %d more items for %d€\n", company.Id, added, totalCost)
		}

		if newQ < 0 {
			return apis.NewBadRequestError("La quantité ne peut pas être négative", nil)
		}

		return nil
	})

	app.OnRecordAfterUpdateSuccess("inventory").BindFunc(func(e *core.RecordEvent) error {
		if e.Record.GetInt("quantity") == 0 {
			if err := e.App.Delete(e.Record); err != nil {
				log.Println("Erreur lors de la suppression de l'inventaire:", err)
			}
		}
		return nil
	})
}
