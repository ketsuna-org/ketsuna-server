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
}
