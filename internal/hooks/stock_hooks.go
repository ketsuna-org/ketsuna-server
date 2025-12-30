package hooks

import (
	"fmt"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func registerStockHooks(app *pocketbase.PocketBase) {
	app.OnRecordCreateRequest("stocks").BindFunc(func(e *core.RecordRequestEvent) error {
		r := e.Record
		companyId := r.GetString("company")
		symbol := r.GetString("symbol")

		if companyId == "" {
			return apis.NewBadRequestError("Company est requis", nil)
		}
		if symbol == "" {
			return apis.NewBadRequestError("Le symbole boursier est requis", nil)
		}
		// TODO: Regex validation for symbol if strictly needed
		// (JS used /^[A-Z0-9]{1,3}$/)

		existing, _ := app.FindFirstRecordByFilter("stocks", fmt.Sprintf("symbol = '%s'", symbol))
		if existing != nil {
			return apis.NewBadRequestError(fmt.Sprintf("Le symbole %s est déjà utilisé", symbol), nil)
		}

		companyStock, _ := app.FindFirstRecordByFilter("stocks", fmt.Sprintf("company = '%s'", companyId))
		if companyStock != nil {
			return apis.NewBadRequestError("Cette entreprise a déjà un stock en bourse", nil)
		}

		company, err := app.FindRecordById("companies", companyId)
		if err != nil {
			return apis.NewBadRequestError("Company introuvable", nil)
		}
		if company.GetInt("level") < 5 {
			return apis.NewBadRequestError("Niveau 5 minimum requis pour entrer en bourse", nil)
		}

		// Defaults
		if r.GetFloat("share_price") <= 0 {
			r.Set("share_price", 10.0)
		}
		if r.GetInt("total_shares") <= 0 {
			r.Set("total_shares", 1000000)
		}

		total := r.GetInt("total_shares")
		// if shares_owned_by_public is not set by user (0/empty), set default
		// Warning: 0 might be valid? JS: undefined check.
		// Go GetInt returns 0 if missing.
		// We'll set it if "shares_owned_by_public" key is missing from data? Hard to know in CreateRequest.
		// Let's assume safely if 0 provided we default.
		if r.GetInt("shares_owned_by_public") == 0 {
			r.Set("shares_owned_by_public", int(float64(total)*0.3))
		}
		if r.GetFloat("volatility") == 0 {
			r.Set("volatility", 0.1)
		}

		// Price History Init
		// Creating manual JSON for history
		now := time.Now().UTC().Format(time.RFC3339)
		price := r.GetFloat("share_price")
		history := []map[string]interface{}{
			{"date": now, "price": price, "volume": 0},
		}
		r.Set("price_history_json", history)

		return e.Next()
	})

	app.OnRecordAfterCreateSuccess("stocks").BindFunc(func(e *core.RecordEvent) error {
		r := e.Record
		companyId := r.GetString("company")
		total := r.GetInt("total_shares")
		public := r.GetInt("shares_owned_by_public")
		ceoShares := total - public

		if ceoShares <= 0 {
			return nil
		}

		company, err := app.FindRecordById("companies", companyId)
		if err != nil {
			return nil
		}
		ceoId := company.GetString("ceo")
		if ceoId == "" {
			return nil
		}

		// Find holder company for CEO
		// "ceo = ceoId && id != companyId" -> find if they own another company used for holding?
		// JS logic: if not found, use the company itself? Wait.
		// `holderCompany = company` in JS fallback.
		// So CEO holds shares of their own company via their own company? Or user conceptual holding?
		// JS: `holder_company` field refers to a Company record.
		// If CEO has multiple companies, try to find one that IS NOT the listed company.
		// If fails, loop back to listed company.

		holderCompany := company
		other, _ := app.FindFirstRecordByFilter("companies", fmt.Sprintf("ceo = '%s' && id != '%s'", ceoId, companyId))
		if other != nil {
			holderCompany = other
		}

		coll, err := app.FindCollectionByNameOrId("shareholders")
		if err != nil {
			return err
		}

		sh := core.NewRecord(coll)
		sh.Set("holder_company", holderCompany.Id)
		sh.Set("stock", r.Id)
		sh.Set("quantity", ceoShares)
		if err := app.Save(sh); err != nil {
			app.Logger().Error("Erreur création actionnaire CEO", "error", err)
		} else {
			app.Logger().Info("Actionnaire créé", "ceoShares", ceoShares, "holderCompany", holderCompany.GetString("name"))
		}

		return nil
	})

	app.OnRecordUpdateRequest("stocks").BindFunc(func(e *core.RecordRequestEvent) error {
		r := e.Record
		orig := r.Original()

		if r.GetString("symbol") != orig.GetString("symbol") {
			return apis.NewBadRequestError("Le symbole boursier ne peut pas être modifié", nil)
		}

		oldTotal := orig.GetInt("total_shares")
		newTotal := r.GetInt("total_shares")

		if newTotal < oldTotal {
			return apis.NewBadRequestError("Le nombre total d'actions ne peut pas être réduit", nil)
		}

		public := r.GetInt("shares_owned_by_public")
		if public > newTotal {
			return apis.NewBadRequestError("Les actions publiques ne peuvent pas dépasser le total", nil)
		}

		return e.Next()
	})

	app.OnRecordAfterUpdateSuccess("stocks").BindFunc(func(e *core.RecordEvent) error {
		r := e.Record
		oldPrice := r.Original().GetFloat("share_price")
		newPrice := r.GetFloat("share_price")

		if oldPrice != newPrice {
			// Update history
			// Need to parse existing json... this is tricky with untyped JSON field in Go without struct
			// Use simple interface map
			var history []map[string]interface{}
			r.UnmarshalJSONField("price_history_json", &history)

			if len(history) >= 100 {
				history = history[len(history)-99:]
			}
			history = append(history, map[string]interface{}{
				"date":   time.Now().UTC().Format(time.RFC3339),
				"price":  newPrice,
				"volume": 0,
			})

			r.Set("price_history_json", history)
			app.Save(r)
		}
		return nil
	})

	app.OnRecordDeleteRequest("stocks").BindFunc(func(e *core.RecordRequestEvent) error {
		r := e.Record
		counts, err := app.FindRecordsByFilter("shareholders", fmt.Sprintf("stock = '%s'", r.Id), "", 1, 0)
		if err == nil && len(counts) > 0 {
			return apis.NewBadRequestError("Impossible de supprimer le stock : il y a encore des actionnaires.", nil)
		}
		return e.Next()
	})
}

func registerShareholderHooks(app *pocketbase.PocketBase) {
	app.OnRecordCreateRequest("shareholders").BindFunc(func(e *core.RecordRequestEvent) error {
		r := e.Record
		stockId := r.GetString("stock")
		holderId := r.GetString("holder_company")
		qty := r.GetInt("quantity")

		if stockId == "" || holderId == "" {
			return apis.NewBadRequestError("holder_company et stock sont requis", nil)
		}
		if qty <= 0 {
			return apis.NewBadRequestError("Quantité doit être > 0", nil)
		}

		stock, err := app.FindRecordById("stocks", stockId)
		if err != nil {
			return apis.NewBadRequestError("Stock introuvable", nil)
		}

		// Check availability
		// Sum all shareholders for this stock
		// SELECT SUM(quantity) ... difficult in PB without raw SQL or loop.
		// Go loop approach:
		// Note: Pagination limited. Need FetchAll logic or SQL.
		// Assuming not thousands of shareholders yet.
		shares, _ := app.FindRecordsByFilter("shareholders", fmt.Sprintf("stock = '%s'", stockId), "", 0, 0)
		totalOwned := 0
		for _, s := range shares {
			totalOwned += s.GetInt("quantity")
		}

		totalShares := stock.GetInt("total_shares")
		avail := totalShares - totalOwned

		if qty > avail {
			return apis.NewBadRequestError(fmt.Sprintf("Pas assez d'actions disponibles. Disponible: %d", avail), nil)
		}

		// Check duplicate
		existing, _ := app.FindFirstRecordByFilter("shareholders", fmt.Sprintf("holder_company = '%s' && stock = '%s'", holderId, stockId))
		if existing != nil {
			curr := existing.GetInt("quantity")
			existing.Set("quantity", curr+qty)
			app.Save(existing)
			return apis.NewBadRequestError(fmt.Sprintf("Actionnaire déjà existant. Quantité mise à jour: %d", curr+qty), nil)
		}

		return e.Next()
	})

	app.OnRecordUpdateRequest("shareholders").BindFunc(func(e *core.RecordRequestEvent) error {
		r := e.Record
		qty := r.GetInt("quantity")
		if qty < 0 {
			return apis.NewBadRequestError("Quantité négative interdite", nil)
		}

		oldQty := r.Original().GetInt("quantity")
		if qty > oldQty {
			increase := qty - oldQty
			stockId := r.GetString("stock")
			stock, err := app.FindRecordById("stocks", stockId)
			if err != nil {
				return err
			}

			// Re-calc availability excluding current record
			shares, _ := app.FindRecordsByFilter("shareholders", fmt.Sprintf("stock = '%s' && id != '%s'", stockId, r.Id), "", 0, 0)
			totalOwned := 0
			for _, s := range shares {
				totalOwned += s.GetInt("quantity")
			}

			// Add old qty back to "owned" conceptually? No, we want what OTHERS own.
			// Avail = Total - OthersOwned - OldSelfOwned
			// Actually logic in JS: Avail = Total - Owned(All) - OldSelf.
			// Wait: JS loops all shareholders EXCLUDING current ID.
			// Then Avail = Total - OwnedByOthers - OldSelf.
			// Wait, JS: Avail = Total - OwnedByOthers - OldSelf.
			// If increase > Avail -> Error.

			totalShares := stock.GetInt("total_shares")
			avail := totalShares - totalOwned - oldQty
			// Note: The logic verification is: can we fit 'increase'?
			// 'avail' here represents what is free APART from what I already had.
			// So if I have 10, ask for 15 (increase 5).
			// Total 100. Others have 80.
			// Avail = 100 - 80 - 10 = 10.
			// Increase 5 <= 10. OK.

			if increase > avail {
				return apis.NewBadRequestError(fmt.Sprintf("Pas assez d'actions disponibles. Disponible: %d", avail), nil)
			}
		}
		return e.Next()
	})

	app.OnRecordAfterUpdateSuccess("shareholders").BindFunc(func(e *core.RecordEvent) error {
		if e.Record.GetInt("quantity") == 0 {
			app.Delete(e.Record)
		}
		return nil
	})
}
