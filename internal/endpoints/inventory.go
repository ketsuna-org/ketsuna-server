package endpoints

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"ketsuna.com/server/internal/gamedata"
	"ketsuna.com/server/internal/hooks"
)

// registerInventoryEndpoints handles /api/inventory/* routes
func registerInventoryEndpoints(app *pocketbase.PocketBase, e *core.ServeEvent, inv *hooks.InventoryLogic, graph *hooks.GraphEconomy) {

	// POST /api/inventory/sell - Sell inventory items
	e.Router.POST("/api/inventory/sell", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Vous devez être connecté.", nil)
		}

		data := struct {
			ItemId    string  `json:"itemId" form:"itemId"`
			Quantity  float64 `json:"quantity" form:"quantity"`
			StorageId string  `json:"storageId" form:"storageId"` // Optional: if set, sell from this storage's inventory
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

		// NOTE: Do NOT call graph.CalculateCompanyInventory here!
		// Calling it before sell caused a bug where production was instantly added
		// right before selling, making inventory appear to replenish immediately.
		// Production calculation should only happen on explicit refresh calls.

		return app.RunInTransaction(func(txApp core.App) error {
			company, err := txApp.FindRecordById("companies", companyId)
			if err != nil {
				return apis.NewBadRequestError("Entreprise introuvable", nil)
			}
			if company.GetString("ceo") != authRecord.Id {
				return apis.NewForbiddenError("Accès refusé", nil)
			}

			result, err := inv.SellInventory(txApp, companyId, data.ItemId, int(data.Quantity), data.StorageId)
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

	// POST /api/inventory/sell-bulk - Sell multiple items in one request
	e.Router.POST("/api/inventory/sell-bulk", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Vous devez être connecté.", nil)
		}

		body := struct {
			StorageId string `json:"storageId" form:"storageId"`
			Items     []struct {
				ItemId   string  `json:"itemId" form:"itemId"`
				Quantity float64 `json:"quantity" form:"quantity"`
			} `json:"items" form:"items"`
		}{}

		if err := c.BindBody(&body); err != nil {
			return apis.NewBadRequestError("Corps JSON invalide", err)
		}

		if len(body.Items) == 0 {
			return apis.NewBadRequestError("Aucun item à vendre", nil)
		}

		companyId := authRecord.GetString("active_company")
		if companyId == "" {
			return apis.NewBadRequestError("Aucune entreprise active pour cet utilisateur", nil)
		}

		// Keep same behavior as single sell: no pre-refresh, operate in a transaction
		return app.RunInTransaction(func(txApp core.App) error {
			company, err := txApp.FindRecordById("companies", companyId)
			if err != nil {
				return apis.NewBadRequestError("Entreprise introuvable", nil)
			}
			if company.GetString("ceo") != authRecord.Id {
				return apis.NewForbiddenError("Accès refusé", nil)
			}

			successes := make([]map[string]interface{}, 0)
			failures := make([]map[string]interface{}, 0)
			totalRevenue := 0.0

			for _, item := range body.Items {
				if item.ItemId == "" || item.Quantity <= 0 {
					failures = append(failures, map[string]interface{}{
						"itemId":   item.ItemId,
						"quantity": item.Quantity,
						"error":    "itemId et quantity > 0 requis",
					})
					continue
				}

				qty := int(item.Quantity)
				result, sellErr := inv.SellInventory(txApp, companyId, item.ItemId, qty, body.StorageId)
				if sellErr != nil {
					failures = append(failures, map[string]interface{}{
						"itemId":   item.ItemId,
						"quantity": qty,
						"error":    sellErr.Error(),
					})
					continue
				}

				totalRevenue += result.Revenue
				successes = append(successes, map[string]interface{}{
					"itemId":        item.ItemId,
					"quantity":      qty,
					"revenue":       result.Revenue,
					"unitSellPrice": result.UnitSellPrice,
				})
			}

			if len(successes) == 0 {
				errMsg := "Aucune vente réalisée"
				if len(failures) > 0 {
					errMsg = fmt.Sprintf("Aucune vente réalisée: %v", failures[0]["error"])
				}
				return apis.NewBadRequestError(errMsg, nil)
			}

			return c.JSON(200, map[string]interface{}{
				"success":      true,
				"storageId":    body.StorageId,
				"totalRevenue": totalRevenue,
				"itemsSold":    successes,
				"failures":     failures,
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

			// 1. Get Static Item
			item := gamedata.GetItem(data.ItemId) // Static lookup
			if item == nil {
				return apis.NewBadRequestError("Item introuvable", nil) // Logic Fix
			}

			if !item.MarketAvailable {
				return apis.NewBadRequestError("Cet item n'est pas disponible sur le marché", nil)
			}

			// 2. Check Tech Requirements
			isLocked, requiredTechId := gamedata.IsItemUnlockedByTech(item.ID)
			if isLocked {
				// Check if company has this technology
				_, err := txApp.FindFirstRecordByFilter(
					"company_techs",
					fmt.Sprintf("company='%s' && technology_id='%s'", companyId, requiredTechId),
				)
				if err != nil {
					return apis.NewBadRequestError("Technologie requise non débloquée ! Vous ne pouvez pas acheter cet item.", nil)
				}
			}

			// 3. Calculate Cost
			totalCost := item.BasePrice * float64(data.Quantity)
			currentBalance := company.GetFloat("balance")

			if currentBalance < totalCost {
				return apis.NewBadRequestError(fmt.Sprintf("Fonds insuffisants. Coût: %.2f€, Solde: %.2f€", totalCost, currentBalance), nil)
			}

			var resultRecord *core.Record

			// 4. Handle Purchase based on Type
			// Both Machine and Stockage types go to 'machines' collection (placeable)
			if item.Type == gamedata.ItemTypeMachine || item.Type == gamedata.ItemTypeStockage {
				// Machines & Storage -> 'machines' collection
				machinesCollection, err := txApp.FindCollectionByNameOrId("machines")
				if err != nil {
					return err
				}

				for i := 0; i < data.Quantity; i++ {
					machineRecord := core.NewRecord(machinesCollection)
					machineRecord.Set("company", companyId)
					machineRecord.Set("machine_id", item.ID) // Text field
					machineRecord.Set("durability", 100)
					machineRecord.Set("placed", false)
					machineRecord.Set("stored_energy", 0)
					machineRecord.Set("production_started_at", "") // not started

					if err := txApp.Save(machineRecord); err != nil {
						return apis.NewBadRequestError("Erreur lors de la création de la machine", err)
					}
					resultRecord = machineRecord // Return last created machine
				}

			} else {
				// Everyone else -> 'inventory' collection
				// Check existing inventory
				// Note: using item_id field because we don't have item relation anymore or rely on it
				existing, _ := txApp.FindFirstRecordByFilter(
					"inventory",
					fmt.Sprintf("company='%s' && item_id='%s'", companyId, item.ID),
				)

				if existing != nil {
					newQty := existing.GetInt("quantity") + data.Quantity
					existing.Set("quantity", newQty)
					if err := txApp.Save(existing); err != nil {
						return apis.NewBadRequestError("Erreur mise à jour inventaire", err)
					}
					resultRecord = existing
				} else {
					invCollection, err := txApp.FindCollectionByNameOrId("inventory")
					if err != nil {
						return err
					}
					newItem := core.NewRecord(invCollection)
					newItem.Set("company", companyId)
					newItem.Set("item_id", item.ID)
					newItem.Set("quantity", data.Quantity)
					if err := txApp.Save(newItem); err != nil {
						return apis.NewBadRequestError("Erreur création inventaire", err)
					}
					resultRecord = newItem
				}
			}

			// 5. Update Company Balance
			company.Set("balance", currentBalance-totalCost)
			if err := txApp.Save(company); err != nil {
				return apis.NewBadRequestError("Erreur sauvegarde entreprise", err)
			}

			app.Logger().Debug("[PURCHASE] Company purchased item", "companyId", companyId, "itemId", item.ID, "qty", data.Quantity, "totalCost", totalCost)

			return c.JSON(200, map[string]interface{}{
				"success": true,
				"message": "Achat réussi",
				"record":  resultRecord,
				"cost":    totalCost,
			})
		})
	})

	// POST /api/market/list - Get paginated market items with tech filtering
	// POST /api/market/list - Get paginated market items with tech filtering (Static Data)
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

		companyId := authRecord.GetString("active_company")
		if companyId == "" {
			return apis.NewBadRequestError("Aucune entreprise active", nil)
		}

		// 1. Get unlocked technologies set
		companyTechs, _ := app.FindRecordsByFilter("company_techs", fmt.Sprintf("company = '%s'", companyId), "", 1000, 0)
		unlockedTechs := make(map[string]bool)
		for _, ct := range companyTechs {
			// Schema uses 'technology_id'
			techId := ct.GetString("technology_id")
			if techId != "" {
				unlockedTechs[techId] = true
			}
		}

		// 2. Fetch Static Items
		allItems := gamedata.GetAllItems()
		var candidates []gamedata.Item

		searchLower := strings.ToLower(data.Search)

		for _, item := range allItems {
			// Filter MarketAvailable
			if !item.MarketAvailable {
				continue
			}

			// Filter Type: Only Machine & Stockage
			if item.Type != gamedata.ItemTypeMachine && item.Type != gamedata.ItemTypeStockage {
				continue
			}

			// Filter Type from request (if specified) e.g. "Machine" vs "Stockage"
			if data.Type != "" && string(item.Type) != data.Type {
				continue
			}

			// Filter Search
			if searchLower != "" {
				if !strings.Contains(strings.ToLower(item.Name), searchLower) {
					continue
				}
			}

			// Filter Tech
			isUnlocked, techID := gamedata.IsItemUnlockedByTech(item.ID)
			// gamedata logic: IsItemUnlockedByTech returns true if item requires tech.
			// The function name is IsItemUnlockedByTech...
			// Wait, let me check definition in Step 509 lines 196:
			// func IsItemUnlockedByTech(itemId string) (bool, string)
			// "checks if an item ID is unlocked by any technology"
			// returns (true, techID) if it IS unlocked by a tech.
			// So if true, we MUST have that tech unlocked.
			if isUnlocked {
				if !unlockedTechs[techID] {
					continue // Logic: Item requires tech, but we don't have it -> Skip
				}
			}

			candidates = append(candidates, item)
		}

		// 3. Sort (By Name default)
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].Name < candidates[j].Name
		})

		// 4. Pagination
		totalItems := len(candidates)
		totalPages := (totalItems + data.PerPage - 1) / data.PerPage

		start := (data.Page - 1) * data.PerPage
		if start < 0 {
			start = 0
		}
		end := start + data.PerPage
		if end > totalItems {
			end = totalItems
		}

		var paginatedItems []gamedata.Item
		if start < totalItems {
			paginatedItems = candidates[start:end]
		} else {
			paginatedItems = []gamedata.Item{}
		}

		return c.JSON(200, map[string]interface{}{
			"items":      paginatedItems,
			"page":       data.Page,
			"perPage":    data.PerPage,
			"totalItems": totalItems,
			"totalPages": totalPages,
		})
	})

	// GET /api/inventory/state - Read-only paginated inventory (no graph recompute)
	// This is the lightweight alternative to /api/inventory/refresh
	e.Router.GET("/api/inventory/state", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Non connecté", nil)
		}

		companyId := authRecord.GetString("active_company")
		if companyId == "" {
			return apis.NewBadRequestError("Aucune entreprise active", nil)
		}

		// Parse query parameters
		page := 1
		perPage := 20
		if p := c.Request.URL.Query().Get("page"); p != "" {
			fmt.Sscanf(p, "%d", &page)
		}
		if pp := c.Request.URL.Query().Get("perPage"); pp != "" {
			fmt.Sscanf(pp, "%d", &perPage)
		}
		if page < 1 {
			page = 1
		}
		if perPage < 1 {
			perPage = 20
		}
		if perPage > 100 {
			perPage = 100 // Cap at 100 items per page
		}

		filterParam := c.Request.URL.Query().Get("filter")
		sortParam := c.Request.URL.Query().Get("sort")
		expandParam := c.Request.URL.Query().Get("expand")

		// Build filter - always filter by company
		filter := fmt.Sprintf("company = '%s'", companyId)
		if filterParam != "" {
			// Allow additional filtering (e.g., item_id, linked_storage)
			filter = fmt.Sprintf("%s && (%s)", filter, filterParam)
		}

		// Default sort by item_id if not specified
		if sortParam == "" {
			sortParam = "item_id"
		}

		// Count total items first
		allRecords, err := app.FindRecordsByFilter("inventory", filter, "", 0, 0)
		if err != nil {
			return apis.NewBadRequestError("Erreur lors de la récupération de l'inventaire", err)
		}
		totalItems := len(allRecords)
		totalPages := (totalItems + perPage - 1) / perPage
		if totalPages < 1 {
			totalPages = 1
		}

		// Fetch paginated records
		offset := (page - 1) * perPage
		records, err := app.FindRecordsByFilter("inventory", filter, sortParam, perPage, offset)
		if err != nil {
			return apis.NewBadRequestError("Erreur lors de la récupération de l'inventaire", err)
		}

		// Build response items
		items := make([]map[string]interface{}, 0, len(records))
		for _, rec := range records {
			item := map[string]interface{}{
				"id":             rec.Id,
				"company":        rec.GetString("company"),
				"item_id":        rec.GetString("item_id"),
				"quantity":       rec.GetFloat("quantity"),
				"linked_storage": rec.GetString("linked_storage"),
				"created":        rec.Created.String(),
				"updated":        rec.Updated.String(),
			}

			// Handle expand parameter
			if expandParam != "" && strings.Contains(expandParam, "linked_storage") {
				storageId := rec.GetString("linked_storage")
				if storageId != "" {
					storage, err := app.FindRecordById("machines", storageId)
					if err == nil {
						item["expand"] = map[string]interface{}{
							"linked_storage": storage,
						}
					}
				}
			}

			items = append(items, item)
		}

		return c.JSON(200, map[string]interface{}{
			"items":      items,
			"page":       page,
			"perPage":    perPage,
			"totalItems": totalItems,
			"totalPages": totalPages,
		})
	}).Bind(apis.RequireAuth())

	// POST /api/inventory/refresh - Trigger lazy calculation and pull production into inventory
	e.Router.POST("/api/inventory/refresh", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Non connecté", nil)
		}

		companyId := authRecord.GetString("active_company")
		if companyId == "" {
			return apis.NewBadRequestError("Aucune entreprise active", nil)
		}

		// Verify company ownership
		company, err := app.FindRecordById("companies", companyId)
		if err != nil {
			return apis.NewBadRequestError("Entreprise introuvable", nil)
		}
		if company.GetString("ceo") != authRecord.Id && !authRecord.IsSuperuser() {
			return apis.NewForbiddenError("Accès refusé", nil)
		}

		// Trigger lazy calculation
		if graph == nil {
			return apis.NewBadRequestError("Graph economy non initialisé", nil)
		}

		// Trigger Technology Status Update (Lazy Update)
		if err := hooks.UpdateCompanyTechStatus(app, companyId); err != nil {
			app.Logger().Error("[TECH] Failed to update tech status", "err", err)
		}
		producedItems, err := graph.CalculateCompanyInventory(companyId)
		if err != nil {
			app.Logger().Error("[REFRESH] Graph calculation failed", "err", err)
			return apis.NewBadRequestError("Erreur lors du calcul de production", err)
		}

		// Also fetch storage inventories (items buffered in storage nodes)
		storageInvRecords, _ := app.FindRecordsByFilter("inventory",
			fmt.Sprintf("company = '%s' && linked_storage != ''", companyId), "", 0, 0)

		storageInventory := make(map[string]interface{})
		for _, inv := range storageInvRecords {
			storageId := inv.GetString("linked_storage")
			itemId := inv.GetString("item_id")
			quantity := inv.GetFloat("quantity")

			storageInventory[storageId] = map[string]interface{}{
				"item_id":    itemId,
				"quantity":   quantity,
				"storage_id": storageId,
			}
		}

		app.Logger().Debug("[REFRESH] Lazy calculation completed", "company", companyId, "producedItems", len(producedItems), "storageItems", len(storageInventory))

		return c.JSON(200, map[string]interface{}{
			"success":          true,
			"producedItems":    producedItems,
			"storageInventory": storageInventory,
			"message":          fmt.Sprintf("%d type(s) d'items produits, %d stockages", len(producedItems), len(storageInventory)),
		})
	}).Bind(apis.RequireAuth())
}
