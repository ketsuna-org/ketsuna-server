package endpoints

import (
	"fmt"
	"math"

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

			// Block purchase of minable items - REMOVED (Minable items are buyable and unlimited)
			// if item.GetBool("minable") {
			// 	return apis.NewBadRequestError("Les ressources brutes ne peuvent pas être achetées. Récoltez-les manuellement !", nil)
			// }

			// Check technology requirement
			requiredTechId := item.GetString("required_tech")
			if requiredTechId != "" {
				// Check if company has this technology
				_, err := txApp.FindFirstRecordByFilter(
					"company_techs",
					fmt.Sprintf("company='%s' && technology='%s'", companyId, requiredTechId),
				)
				if err != nil {
					return apis.NewBadRequestError("Technologie requise non débloquée ! Vous ne pouvez pas acheter cet item.", nil)
				}
			}

			// Check Stock (market_demand)
			// Minable items are UNLIMITED (no stock check)
			marketStock := item.GetInt("market_demand")
			isMinable := item.GetBool("minable")

			if !isMinable {
				if marketStock <= 0 {
					return apis.NewBadRequestError("Rupture de stock ! Nexa-Bank n'a plus cet item disponible aujourd'hui.", nil)
				}
				if marketStock < data.Quantity {
					return apis.NewBadRequestError(fmt.Sprintf("Stock insuffisant. Disponible: %d, Demandé: %d", marketStock, data.Quantity), nil)
				}
				// We update stock later
			}

			itemPrice := item.GetFloat("base_price") // Changed to Float
			totalCost := itemPrice * float64(data.Quantity)
			current := company.GetFloat("balance") // Changed to Float
			if current < totalCost {
				return apis.NewBadRequestError(fmt.Sprintf("Fonds insuffisants. Coût: %.2f€, Solde: %.2f€", totalCost, current), nil)
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

			// Update Market Data (Stock & Price)
			if !isMinable {
				item.Set("market_demand", marketStock-data.Quantity)
			}

			// Dynamic Pricing (Only for Machines)
			if item.GetString("type") == "Machine" {
				priceFactor := 0.005
				newPrice := itemPrice * (1 + float64(data.Quantity)*priceFactor)
				item.Set("base_price", math.Round(newPrice*100)/100)
			}

			if err := txApp.Save(item); err != nil {
				return apis.NewBadRequestError("Erreur sauvegarde item", err)
			}

			return c.JSON(200, map[string]interface{}{
				"success":        true,
				"message":        "Achat réussi",
				"record":         existing,
				"cost":           totalCost,
				"remainingStock": marketStock - data.Quantity,
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

		// 1. Get unlocked technologies set
		companyTechs, _ := app.FindRecordsByFilter("company_techs", fmt.Sprintf("company = '%s'", companyId), "", 1000, 0)
		unlockedTechs := make(map[string]bool)
		for _, ct := range companyTechs {
			techId := ct.GetString("technology")
			if techId != "" {
				unlockedTechs[techId] = true
			}
		}

		// 2. Build Base Filter (DB side)
		// Only fetch candidate items to reduce memory usage slightly
		dbFilter := "type != 'Produit Fini' && minable = false"
		if data.Search != "" {
			dbFilter += fmt.Sprintf(" && name ~ '%s'", data.Search)
		}
		if data.Type != "" {
			dbFilter += fmt.Sprintf(" && type = '%s'", data.Type)
		}

		// Fetch ALL matching items (high limit)
		// Note: We handle sorting in DB if possible, but filtering might break pagination if done after.
		// So we must fetch ALL candidates, filter them in Go, THEN sort/paginate.
		// Sorting in DB first is okay, we preserve order if we iterate in order.
		allCandidates, err := app.FindRecordsByFilter("items", dbFilter, data.Sort, 1000, 0)
		if err != nil {
			return apis.NewBadRequestError("Erreur lors de la recherche DB", err)
		}

		// 3. Filter by Technology (Go side)
		var validItems []*core.Record
		for _, item := range allCandidates {
			reqTech := item.GetString("required_tech")
			// If no tech required OR tech is unlocked
			if reqTech == "" || unlockedTechs[reqTech] {
				validItems = append(validItems, item)
			}
		}

		// 4. Pagination (Go side)
		totalItems := len(validItems)
		totalPages := (totalItems + data.PerPage - 1) / data.PerPage

		start := (data.Page - 1) * data.PerPage
		if start < 0 {
			start = 0
		}
		end := start + data.PerPage
		if end > totalItems {
			end = totalItems
		}

		var paginatedItems []*core.Record
		if start < totalItems {
			paginatedItems = validItems[start:end]
		} else {
			paginatedItems = []*core.Record{}
		}

		// 5. Expand relations
		for _, record := range paginatedItems {
			app.ExpandRecord(record, []string{"use_recipe", "product", "can_consume"}, nil)
			if recipe := record.ExpandedOne("use_recipe"); recipe != nil {
				// Nested expand for recipe inputs/output
				app.ExpandRecord(recipe, []string{"inputs_items", "output_item"}, nil)
			}
		}

		return c.JSON(200, map[string]interface{}{
			"items":      paginatedItems,
			"page":       data.Page,
			"perPage":    data.PerPage,
			"totalItems": totalItems,
			"totalPages": totalPages,
		})
	})
}
