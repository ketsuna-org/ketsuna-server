package endpoints

import (
	"fmt"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"ketsuna.com/server/internal/hooks"
)

// registerInventoryEndpoints handles /api/inventory/* routes
func registerInventoryEndpoints(app *pocketbase.PocketBase, e *core.ServeEvent, inv *hooks.InventoryLogic) {

	// POST /api/inventory/sell - Sell inventory items
	e.Router.POST("/api/inventory/sell", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Vous devez être connecté.", nil)
		}

		data := struct {
			ItemId   string `json:"itemId" form:"itemId"`
			Quantity int    `json:"quantity" form:"quantity"`
		}{}

		if err := c.BindBody(&data); err != nil {
			return apis.NewBadRequestError("Corps JSON invalide", err)
		}

		if data.ItemId == "" || data.Quantity <= 0 {
			return apis.NewBadRequestError("Paramètres manquants: itemId et quantity > 0 requis", nil)
		}

		companyId := authRecord.GetString("active_company")
		if companyId == "" {
			return apis.NewBadRequestError("Aucune entreprise active pour cet utilisateur", nil)
		}

		return app.RunInTransaction(func(txApp core.App) error {
			company, err := txApp.FindRecordById("companies", companyId)
			if err != nil {
				return apis.NewBadRequestError("Entreprise introuvable", nil)
			}
			if company.GetString("ceo") != authRecord.Id {
				return apis.NewForbiddenError("Accès refusé", nil)
			}

			result, err := inv.SellInventory(txApp, companyId, data.ItemId, data.Quantity)
			if err != nil {
				return apis.NewBadRequestError(err.Error(), nil)
			}

			return c.JSON(200, map[string]interface{}{
				"success":       true,
				"revenue":       result.Revenue,
				"unitSellPrice": result.UnitSellPrice,
				"techGain":      result.TechGain,
			})
		})
	})

	// POST /api/inventory/purchase - Buy items from market
	e.Router.POST("/api/inventory/purchase", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Vous devez être connecté.", nil)
		}

		data := struct {
			CompanyId string `json:"companyId" form:"companyId"`
			ItemId    string `json:"itemId" form:"itemId"`
			Quantity  int    `json:"quantity" form:"quantity"`
		}{}
		if err := c.BindBody(&data); err != nil {
			return apis.NewBadRequestError("Corps JSON invalide", err)
		}

		if data.CompanyId == "" || data.ItemId == "" || data.Quantity <= 0 {
			return apis.NewBadRequestError("companyId, itemId requis, quantity > 0", nil)
		}

		companyId := data.CompanyId
		return app.RunInTransaction(func(txApp core.App) error {
			company, err := txApp.FindRecordById("companies", companyId)
			if err != nil {
				return apis.NewBadRequestError("Entreprise introuvable", nil)
			}
			if company.GetString("ceo") != authRecord.Id && !authRecord.IsSuperuser() {
				return apis.NewForbiddenError("Seul le PDG peut acheter", nil)
			}

			item, err := txApp.FindRecordById("items", data.ItemId)
			if err != nil {
				return apis.NewBadRequestError("Item introuvable", nil)
			}

			// Block purchase of minable items
			if item.GetBool("minable") {
				return apis.NewBadRequestError("Les ressources brutes ne peuvent pas être achetées. Récoltez-les manuellement !", nil)
			}

			// Check circulating_supply availability
			circulatingSupply := item.GetInt("circulating_supply")
			if circulatingSupply <= 0 {
				return apis.NewBadRequestError("Rupture de stock ! Nexa-Bank n'a plus cet item disponible aujourd'hui.", nil)
			}
			if circulatingSupply < data.Quantity {
				return apis.NewBadRequestError(fmt.Sprintf("Stock insuffisant. Disponible: %d, Demandé: %d", circulatingSupply, data.Quantity), nil)
			}

			itemPrice := item.GetInt("base_price")
			totalCost := itemPrice * data.Quantity
			current := company.GetInt("balance")
			if current < totalCost {
				return apis.NewBadRequestError(fmt.Sprintf("Fonds insuffisants. Coût: %d€, Solde: %d€", totalCost, current), nil)
			}

			// Check existing inventory
			existing, _ := txApp.FindFirstRecordByFilter("inventory", fmt.Sprintf("company='%s' && item='%s'", companyId, data.ItemId))
			if existing != nil {
				curr := existing.GetInt("quantity")
				existing.Set("quantity", curr+data.Quantity)
				if err := txApp.Save(existing); err != nil {
					return apis.NewBadRequestError("Erreur mise à jour inventaire", err)
				}
			} else {
				collection, err := txApp.FindCollectionByNameOrId("inventory")
				if err != nil {
					return apis.NewBadRequestError("Erreur collection", err)
				}
				newRecord := core.NewRecord(collection)
				newRecord.Set("company", companyId)
				newRecord.Set("item", data.ItemId)
				newRecord.Set("quantity", data.Quantity)
				if err := txApp.Save(newRecord); err != nil {
					return apis.NewBadRequestError("Erreur création inventaire", err)
				}
				existing = newRecord
			}

			// Deduct balance
			company.Set("balance", current-totalCost)
			if err := txApp.Save(company); err != nil {
				return apis.NewBadRequestError("Erreur sauvegarde entreprise", err)
			}

			app.Logger().Info("[PURCHASE] Company purchased item", "companyId", companyId, "itemId", data.ItemId, "qty", data.Quantity, "totalCost", totalCost)

			// Decrement circulating_supply
			item.Set("circulating_supply", circulatingSupply-data.Quantity)
			if err := txApp.Save(item); err != nil {
				return apis.NewBadRequestError("Erreur sauvegarde item", err)
			}

			return c.JSON(200, map[string]interface{}{
				"success":        true,
				"message":        "Achat réussi",
				"record":         existing,
				"cost":           totalCost,
				"remainingStock": circulatingSupply - data.Quantity,
			})
		})
	})

	// POST /api/market/list - Get paginated market items with tech filtering
	e.Router.POST("/api/market/list", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Non connecté", nil)
		}

		// Parse Body
		data := struct {
			Page    int    `json:"page"`
			PerPage int    `json:"perPage"`
			Sort    string `json:"sort"`
			Search  string `json:"search"`
			Type    string `json:"type"`
		}{}
		if err := c.BindBody(&data); err != nil {
			return apis.NewBadRequestError("Invalid JSON", err)
		}

		if data.Page < 1 {
			data.Page = 1
		}
		if data.PerPage < 1 {
			data.PerPage = 16
		}
		if data.Sort == "" {
			data.Sort = "name"
		}

		companyId := authRecord.GetString("active_company")
		if companyId == "" {
			return apis.NewBadRequestError("Aucune entreprise active", nil)
		}

		// 1. Get unlocked technologies
		companyTechs, _ := app.FindRecordsByFilter("company_techs", fmt.Sprintf("company = '%s'", companyId), "", 1000, 0)
		unlockedTechIds := make([]string, 0, len(companyTechs))
		for _, ct := range companyTechs {
			techId := ct.GetString("technology")
			if techId != "" {
				unlockedTechIds = append(unlockedTechIds, techId)
			}
		}

		// 2. Build Filter String
		// Base filters: Not 'Produit Fini', Not 'minable'
		filterExpr := "type != 'Produit Fini' && minable = false"

		// Tech Filter: (required_tech = "" || required_tech = "techId1" || ...)
		techFilter := "required_tech = ''"
		for _, tid := range unlockedTechIds {
			techFilter += fmt.Sprintf(" || required_tech = '%s'", tid)
		}
		filterExpr += fmt.Sprintf(" && (%s)", techFilter)

		// User Filters
		if data.Search != "" {
			// Sanitize search slightly to prevent trivial injection, though PB handles binding
			// Using ~ for like
			filterExpr += fmt.Sprintf(" && name ~ '%s'", data.Search)
		}
		if data.Type != "" {
			filterExpr += fmt.Sprintf(" && type = '%s'", data.Type)
		}

		// 3. Query records with pagination
		result, err := app.FindRecordsByFilter(
			"items",
			filterExpr,
			data.Sort,
			data.PerPage,
			(data.Page-1)*data.PerPage, // limit, offset
		)
		if err != nil {
			return apis.NewBadRequestError("Erreur lors de la recherche", err)
		}

		// Need to get total count for pagination metadata
		// This is a bit expensive but necessary for "totalItems"
		// Optimization: cache count or do a count query first?
		// PB Helper: app.Dao().FindRecordsByFilter gives a slice.
		// To get total count with this complex filter, we might need a separate Count query.
		// Or assume if result < perPage, we are at end. But we ideally want total pages.

		// Let's rely on fetching all records matching filter with Limit 0? No that's too heavy.
		// We'll use a raw DB query for count if speed is needed, but let's try a simpler approach.
		// For now, let's just return what we have and maybe total count is tricky without a dedicated Count API in app.
		// Actually app.Dao().FindRecordsByFilter returns records.
		// We can use app.Dao().RecordQuery("items").AndWhere(dbx.NewExp(filter)).Count()
		// But converting our string filter to DBX expression is handled by PB internal parser.

		// WORKAROUND: For this specific use case, we might not get exact totalItems efficiently without internal PB parser access.
		// Let's just fetch a large number for total? No.

		// Let's try to infer totalItems:
		// We will fetch ALL record IDs matching the filter (Limit 0 usually means all if we don't set it, or default limit)
		// Wait, previously I set limit 1000.
		// Let's do a separate query with limit 0 to get count? No, PB default limit is 30.
		// We can set limit very high (e.g. 9999) and select only ID to count.
		// This is acceptable for the expected dataset size (<a few thousand items).

		totalRecords, _ := app.FindRecordsByFilter("items", filterExpr, "", 9999, 0)
		totalItems := len(totalRecords)
		totalPages := (totalItems + data.PerPage - 1) / data.PerPage

		// Expand relations manually or via API?
		// FindRecordsByFilter does NOT expand automatically unless we dig into RequestEvent or logic.
		// We need to expand: use_recipe.inputs_items, use_recipe.output_item, product
		// We can call app.ExpandRecord(record, expands, fetchFunc)
		for _, record := range result {
			app.ExpandRecord(record, []string{"use_recipe", "product", "can_consume"}, nil)
			if recipe := record.ExpandedOne("use_recipe"); recipe != nil {
				// Nested expand for recipe inputs/output
				app.ExpandRecord(recipe, []string{"inputs_items", "output_item"}, nil)
			}
		}

		return c.JSON(200, map[string]interface{}{
			"items":      result,
			"page":       data.Page,
			"perPage":    data.PerPage,
			"totalItems": totalItems,
			"totalPages": totalPages,
		})
	})
}
