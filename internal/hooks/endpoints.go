package hooks

import (
	"fmt"
	"math"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func RegisterEndpoints(app *pocketbase.PocketBase, inv *InventoryLogic, eco *EconomyLogic) {
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {

		// WORKSHOP Custom Endpoints
		e.Router.POST("/api/workshop/produce", func(c *core.RequestEvent) error {
			authRecord := c.Auth
			if authRecord == nil {
				return apis.NewUnauthorizedError("Vous devez être connecté.", nil)
			}

			// Parse Body
			data := struct {
				RecipeId string `json:"recipeId" form:"recipeId"`
				Quantity int    `json:"quantity" form:"quantity"`
			}{}

			if err := c.BindBody(&data); err != nil {
				return apis.NewBadRequestError("Corps JSON invalide", err)
			}

			if data.RecipeId == "" || data.Quantity <= 0 {
				return apis.NewBadRequestError("Paramètres manquants: recipeId et quantité > 0 requis", nil)
			}

			// Get Active Company
			companyId := authRecord.GetString("active_company")
			if companyId == "" {
				return apis.NewBadRequestError("Aucune entreprise active pour cet utilisateur", nil)
			}

			// Verify CEO
			company, err := app.FindRecordById("companies", companyId)
			if err != nil {
				return apis.NewBadRequestError("Entreprise introuvable", nil)
			}
			if company.GetString("ceo") != authRecord.Id && !authRecord.IsSuperuser() {
				return apis.NewForbiddenError("Seul le PDG peut lancer une production manuelle", nil)
			}

			// Logic
			result, err := inv.ProduceItem(companyId, data.RecipeId, data.Quantity)
			if err != nil {
				app.Logger().Error("[WORKSHOP] Erreur production", "error", err)
				return apis.NewBadRequestError(err.Error(), nil)
			}

			return c.JSON(200, map[string]interface{}{
				"success":        true,
				"message":        "Production réussie",
				"produced":       result.Produced,
				"itemName":       result.ItemName,
				"xpGained":       result.XpGained,
				"productionTime": result.ProductionTime,
			})

		})

		// INVENTORY Custom Endpoints
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

			// Get Active Company
			companyId := authRecord.GetString("active_company")
			if companyId == "" {
				return apis.NewBadRequestError("Aucune entreprise active pour cet utilisateur", nil)
			}
			company, err := app.FindRecordById("companies", companyId)
			if err != nil {
				return apis.NewBadRequestError("Entreprise introuvable", nil)
			}
			if company.GetString("ceo") != authRecord.Id {
				return apis.NewForbiddenError("Accès refusé", nil)
			}

			// Logic
			result, err := inv.SellInventory(companyId, data.ItemId, data.Quantity)
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

		// INVENTORY PURCHASE
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
			company, err := app.FindRecordById("companies", companyId)
			if err != nil {
				return apis.NewBadRequestError("Entreprise introuvable", nil)
			}
			if company.GetString("ceo") != authRecord.Id && !authRecord.IsSuperuser() {
				return apis.NewForbiddenError("Seul le PDG peut acheter", nil)
			}

			item, err := app.FindRecordById("items", data.ItemId)
			if err != nil {
				return apis.NewBadRequestError("Item introuvable", nil)
			}

			itemPrice := item.GetInt("base_price")
			totalCost := itemPrice * data.Quantity
			current := company.GetInt("balance")
			if current < totalCost {
				return apis.NewBadRequestError(fmt.Sprintf("Fonds insuffisants. Coût: %d€, Solde: %d€", totalCost, current), nil)
			}

			// Check existing
			existing, _ := app.FindFirstRecordByFilter("inventory", fmt.Sprintf("company='%s' && item='%s'", companyId, data.ItemId))
			if existing != nil {
				// Update
				curr := existing.GetInt("quantity")
				existing.Set("quantity", curr+data.Quantity)
				if err := app.Save(existing); err != nil {
					return apis.NewBadRequestError("Erreur mise à jour inventaire", err)
				}
			} else {
				// Create
				collection, err := app.FindCollectionByNameOrId("inventory")
				if err != nil {
					return apis.NewBadRequestError("Erreur collection", err)
				}
				newRecord := core.NewRecord(collection)
				newRecord.Set("company", companyId)
				newRecord.Set("item", data.ItemId)
				newRecord.Set("quantity", data.Quantity)
				if err := app.Save(newRecord); err != nil {
					return apis.NewBadRequestError("Erreur création inventaire", err)
				}
				existing = newRecord
			}

			// Deduct
			company.Set("balance", current-totalCost)
			if err := app.Save(company); err != nil {
				// Revert
				if existing.GetInt("quantity") == data.Quantity {
					// Was create, delete
					app.Delete(existing)
				} else {
					// Was update, revert
					existing.Set("quantity", existing.GetInt("quantity")-data.Quantity)
					app.Save(existing)
				}
				return apis.NewBadRequestError("Erreur sauvegarde entreprise", err)
			}

			app.Logger().Info("[PURCHASE] Company purchased item", "companyId", companyId, "itemId", data.ItemId, "qty", data.Quantity, "totalCost", totalCost)

			return c.JSON(200, map[string]interface{}{
				"success": true,
				"message": "Achat réussi",
				"record":  existing,
				"cost":    totalCost,
			})
		})

		// COMPANY FINANCE (ported from JS)
		e.Router.POST("/api/company/finance", func(c *core.RequestEvent) error {
			authRecord := c.Auth
			if authRecord == nil {
				return apis.NewUnauthorizedError("Vous devez être connecté.", nil)
			}

			data := struct {
				CompanyId string `json:"companyId" form:"companyId"`
			}{}
			if err := c.BindBody(&data); err != nil {
				return apis.NewBadRequestError("Corps JSON invalide", err)
			}

			companyId := data.CompanyId
			if companyId == "" {
				companyId = authRecord.GetString("active_company")
				if companyId == "" {
					return apis.NewBadRequestError("Aucune entreprise active", nil)
				}
			}

			breakdown, err := eco.CalculateCompanyFinance(companyId)
			if err != nil {
				return apis.NewBadRequestError(err.Error(), nil)
			}

			return c.JSON(200, breakdown)
		})

		// COMPANY LEVELUP
		e.Router.POST("/api/company/levelup", func(c *core.RequestEvent) error {
			authRecord := c.Auth
			if authRecord == nil {
				return apis.NewUnauthorizedError("Vous devez être connecté.", nil)
			}

			data := struct {
				CompanyId string `json:"companyId" form:"companyId"`
			}{}
			if err := c.BindBody(&data); err != nil {
				return apis.NewBadRequestError("Corps JSON invalide", err)
			}

			companyId := data.CompanyId
			if companyId == "" {
				return apis.NewBadRequestError("companyId requis", nil)
			}

			company, err := app.FindRecordById("companies", companyId)
			if err != nil {
				return apis.NewBadRequestError("Entreprise introuvable", nil)
			}

			// Verify CEO
			if company.GetString("ceo") != authRecord.Id && !authRecord.IsSuperuser() {
				return apis.NewForbiddenError("Seul le PDG peut faire level up l'entreprise", nil)
			}

			currentLevel := company.GetInt("level")
			cost := int(math.Floor(1000 * math.Pow(float64(currentLevel), 1.5)))
			repReq := currentLevel * 10

			balance := company.GetInt("balance")
			reputation := company.GetInt("reputation")

			if balance < cost {
				return apis.NewBadRequestError(fmt.Sprintf("Fonds insuffisants. Coût: %d€, Solde: %d€", cost, balance), nil)
			}
			if reputation < repReq {
				return apis.NewBadRequestError(fmt.Sprintf("Réputation insuffisante. Requis: %d, Actuelle: %d", repReq, reputation), nil)
			}

			company.Set("balance", balance-cost)
			company.Set("level", currentLevel+1)
			if err := app.Save(company); err != nil {
				return apis.NewBadRequestError("Erreur lors de la sauvegarde", err)
			}

			return c.JSON(200, map[string]interface{}{
				"success":  true,
				"message":  "Entreprise level up réussie",
				"newLevel": currentLevel + 1,
				"cost":     cost,
				"repReq":   repReq,
			})
		})

		return e.Next()
	})
}
