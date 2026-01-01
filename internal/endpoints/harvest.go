package endpoints

import (
	"fmt"
	"time"

	"ketsuna.com/server/internal/hooks"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// registerHarvestEndpoints handles /api/harvest/* routes
func registerHarvestEndpoints(app *pocketbase.PocketBase, e *core.ServeEvent, inv *hooks.InventoryLogic) {

	// GET /api/harvest/status - Get current harvest status
	e.Router.GET("/api/harvest/status", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Non connecté", nil)
		}

		companyId := authRecord.GetString("active_company")
		if companyId == "" {
			return apis.NewBadRequestError("Aucune entreprise active", nil)
		}

		company, err := app.FindRecordById("companies", companyId)
		if err != nil {
			return apis.NewBadRequestError("Entreprise introuvable", nil)
		}

		isProducing := company.GetDateTime("is_producing")
		harvestingItemId := company.GetString("item_harvesting")

		var currentHarvest map[string]interface{} = nil
		var remainingSeconds float64 = 0

		if !isProducing.IsZero() && harvestingItemId != "" {
			item, err := app.FindRecordById("items", harvestingItemId)
			if err == nil {
				productionTime := item.GetInt("production_time")
				elapsed := time.Since(isProducing.Time()).Seconds()
				remainingSeconds = float64(productionTime) - elapsed
				if remainingSeconds < 0 {
					remainingSeconds = 0
				}

				currentHarvest = map[string]interface{}{
					"itemId":           harvestingItemId,
					"itemName":         item.GetString("name"),
					"startedAt":        isProducing.Time(),
					"productionTime":   productionTime,
					"remainingSeconds": remainingSeconds,
					"isComplete":       remainingSeconds <= 0,
				}
			}
		}

		// Get minable items
		minableItems, _ := app.FindRecordsByFilter("items", "minable = true", "", 0, 0)
		var minableList []map[string]interface{}
		for _, item := range minableItems {
			minableList = append(minableList, map[string]interface{}{
				"id":             item.Id,
				"name":           item.GetString("name"),
				"type":           item.GetString("type"),
				"productionTime": item.GetInt("production_time"),
				"basePrice":      item.GetFloat("base_price"),
			})
		}

		return c.JSON(200, map[string]interface{}{
			"currentHarvest": currentHarvest,
			"minableItems":   minableList,
		})
	})

	// POST /api/harvest/start - Start harvesting an item
	e.Router.POST("/api/harvest/start", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Non connecté", nil)
		}

		data := struct {
			ItemId string `json:"itemId" form:"itemId"`
		}{}
		if err := c.BindBody(&data); err != nil {
			return apis.NewBadRequestError("Corps invalide", err)
		}

		if data.ItemId == "" {
			return apis.NewBadRequestError("itemId requis", nil)
		}

		companyId := authRecord.GetString("active_company")
		if companyId == "" {
			return apis.NewBadRequestError("Aucune entreprise active", nil)
		}

		company, err := app.FindRecordById("companies", companyId)
		if err != nil {
			return apis.NewBadRequestError("Entreprise introuvable", nil)
		}

		if company.GetString("ceo") != authRecord.Id && !authRecord.IsSuperuser() {
			return apis.NewForbiddenError("Accès refusé", nil)
		}

		if !company.GetDateTime("is_producing").IsZero() {
			return apis.NewBadRequestError("Une récolte est déjà en cours", nil)
		}

		item, err := app.FindRecordById("items", data.ItemId)
		if err != nil {
			return apis.NewBadRequestError("Item introuvable", nil)
		}

		if !item.GetBool("minable") {
			return apis.NewBadRequestError("Cet item ne peut pas être récolté manuellement", nil)
		}

		company.Set("is_producing", types.NowDateTime())
		company.Set("item_harvesting", data.ItemId)
		if err := app.Save(company); err != nil {
			return apis.NewBadRequestError("Erreur sauvegarde", err)
		}

		return c.JSON(200, map[string]interface{}{
			"success":        true,
			"message":        fmt.Sprintf("Récolte de %s démarrée", item.GetString("name")),
			"productionTime": item.GetInt("production_time"),
		})
	})

	// POST /api/harvest/collect - Collect completed harvest
	e.Router.POST("/api/harvest/collect", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Non connecté", nil)
		}

		companyId := authRecord.GetString("active_company")
		if companyId == "" {
			return apis.NewBadRequestError("Aucune entreprise active", nil)
		}

		return app.RunInTransaction(func(txApp core.App) error {
			company, err := txApp.FindRecordById("companies", companyId)
			if err != nil {
				return apis.NewBadRequestError("Entreprise introuvable", nil)
			}

			if company.GetString("ceo") != authRecord.Id && !authRecord.IsSuperuser() {
				return apis.NewForbiddenError("Accès refusé", nil)
			}

			isProducing := company.GetDateTime("is_producing")
			harvestingItemId := company.GetString("item_harvesting")

			if isProducing.IsZero() || harvestingItemId == "" {
				return apis.NewBadRequestError("Aucune récolte en cours", nil)
			}

			item, err := txApp.FindRecordById("items", harvestingItemId)
			if err != nil {
				return apis.NewBadRequestError("Item introuvable", nil)
			}

			productionTime := item.GetInt("production_time")
			elapsed := time.Since(isProducing.Time()).Seconds()

			if elapsed < float64(productionTime) {
				remaining := float64(productionTime) - elapsed
				return apis.NewBadRequestError(fmt.Sprintf("Récolte pas encore terminée. %.0f secondes restantes", remaining), nil)
			}

			// Atomic claim via conditional update
			res, err := txApp.DB().NewQuery(`
				UPDATE companies 
				SET is_producing = '', item_harvesting = '' 
				WHERE id = {:id} AND is_producing != '' AND item_harvesting != ''
			`).Bind(map[string]interface{}{
				"id": company.Id,
			}).Execute()

			if err != nil {
				return apis.NewBadRequestError("Erreur base de données", err)
			}

			rows, _ := res.RowsAffected()
			if rows == 0 {
				return apis.NewBadRequestError("Récolte déjà effectuée par une autre requête", nil)
			}

			// Update Inventory
			filter := fmt.Sprintf("company = '%s' && item = '%s'", companyId, harvestingItemId)
			invRecord, err := txApp.FindFirstRecordByFilter("inventory", filter)

			if err != nil {
				invColl, err := txApp.FindCollectionByNameOrId("inventory")
				if err != nil {
					return err
				}
				newInv := core.NewRecord(invColl)
				newInv.Set("company", companyId)
				newInv.Set("item", harvestingItemId)
				newInv.Set("quantity", 1)
				if err := txApp.Save(newInv); err != nil {
					return apis.NewBadRequestError("Erreur création inventaire", err)
				}
			} else {
				qty := invRecord.GetInt("quantity")
				invRecord.Set("quantity", qty+1)
				if err := txApp.Save(invRecord); err != nil {
					return apis.NewBadRequestError("Erreur mise à jour inventaire", err)
				}
			}

			return c.JSON(200, map[string]interface{}{
				"success":  true,
				"message":  fmt.Sprintf("1x %s collecté!", item.GetString("name")),
				"itemName": item.GetString("name"),
				"quantity": 1,
			})
		})
	})
}
