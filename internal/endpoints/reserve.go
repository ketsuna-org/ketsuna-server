package endpoints

import (
	"fmt"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// registerReserveEndpoints handles /api/reserve/* routes
func registerReserveEndpoints(app *pocketbase.PocketBase, e *core.ServeEvent) {

	// POST /api/reserve/deposit - Deposit items to reserve
	e.Router.POST("/api/reserve/deposit", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Non connecté", nil)
		}

		data := struct {
			ItemId   string `json:"itemId" form:"itemId"`
			Quantity int    `json:"quantity" form:"quantity"`
		}{}
		if err := c.BindBody(&data); err != nil {
			return apis.NewBadRequestError("Corps invalide", err)
		}

		if data.ItemId == "" || data.Quantity <= 0 {
			return apis.NewBadRequestError("ItemId et Quantity (>0) requis", nil)
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

		// Check Inventory Ownership & Quantity
		invRecord, err := app.FindFirstRecordByFilter("inventory", fmt.Sprintf("company='%s' && item='%s'", companyId, data.ItemId))
		if err != nil || invRecord == nil {
			return apis.NewBadRequestError("Item non trouvé dans l'inventaire", nil)
		}
		currentQty := invRecord.GetInt("quantity")
		if currentQty < data.Quantity {
			return apis.NewBadRequestError("Quantité insuffisante en inventaire", nil)
		}

		// Check Reserve Capacity
		var usedSpace float64
		err = app.DB().Select("SUM(quantity)").From("reserve").Where(dbx.HashExp{"company": companyId}).Row(&usedSpace)
		if err != nil {
			usedSpace = 0
		}

		level := company.GetInt("level")
		maxCapacity := float64(level * 300)

		if (usedSpace + float64(data.Quantity)) > maxCapacity {
			return apis.NewBadRequestError(fmt.Sprintf("Capacité de réserve dépassée. Max: %.0f, Actuel: %.0f, Requis: +%d", maxCapacity, usedSpace, data.Quantity), nil)
		}

		// Decrement Inventory
		if currentQty == data.Quantity {
			if err := app.Delete(invRecord); err != nil {
				return apis.NewBadRequestError("Erreur suppression inventaire", err)
			}
		} else {
			invRecord.Set("quantity", currentQty-data.Quantity)
			if err := app.Save(invRecord); err != nil {
				return apis.NewBadRequestError("Erreur mise à jour inventaire", err)
			}
		}

		// Increment/Create Reserve
		resRecord, err := app.FindFirstRecordByFilter("reserve", fmt.Sprintf("company='%s' && item='%s'", companyId, data.ItemId))
		if err == nil && resRecord != nil {
			resRecord.Set("quantity", resRecord.GetInt("quantity")+data.Quantity)
			if err := app.Save(resRecord); err != nil {
				return apis.NewBadRequestError("Erreur sauvegarde réserve", err)
			}
		} else {
			col, _ := app.FindCollectionByNameOrId("reserve")
			newRes := core.NewRecord(col)
			newRes.Set("company", companyId)
			newRes.Set("item", data.ItemId)
			newRes.Set("quantity", data.Quantity)
			if err := app.Save(newRes); err != nil {
				return apis.NewBadRequestError("Erreur création réserve", err)
			}
		}

		return c.JSON(200, map[string]interface{}{
			"success": true,
			"message": "Dépot réussi",
			"used":    usedSpace + float64(data.Quantity),
			"max":     maxCapacity,
		})
	})

	// POST /api/reserve/withdraw - Withdraw items from reserve
	e.Router.POST("/api/reserve/withdraw", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Non connecté", nil)
		}

		data := struct {
			ItemId   string `json:"itemId" form:"itemId"`
			Quantity int    `json:"quantity" form:"quantity"`
		}{}
		if err := c.BindBody(&data); err != nil {
			return apis.NewBadRequestError("Corps invalide", err)
		}

		if data.ItemId == "" || data.Quantity <= 0 {
			return apis.NewBadRequestError("ItemId et Quantity (>0) requis", nil)
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

		// Check Reserve Ownership & Quantity
		resRecord, err := app.FindFirstRecordByFilter("reserve", fmt.Sprintf("company='%s' && item='%s'", companyId, data.ItemId))
		if err != nil || resRecord == nil {
			return apis.NewBadRequestError("Item non trouvé dans la réserve", nil)
		}
		currentQty := resRecord.GetInt("quantity")
		if currentQty < data.Quantity {
			return apis.NewBadRequestError("Quantité insuffisante en réserve", nil)
		}

		// Decrement Reserve
		if currentQty == data.Quantity {
			if err := app.Delete(resRecord); err != nil {
				return apis.NewBadRequestError("Erreur suppression réserve", err)
			}
		} else {
			resRecord.Set("quantity", currentQty-data.Quantity)
			if err := app.Save(resRecord); err != nil {
				return apis.NewBadRequestError("Erreur mise à jour réserve", err)
			}
		}

		// Increment/Create Inventory
		invRecord, err := app.FindFirstRecordByFilter("inventory", fmt.Sprintf("company='%s' && item='%s'", companyId, data.ItemId))
		if err == nil && invRecord != nil {
			invRecord.Set("quantity", invRecord.GetInt("quantity")+data.Quantity)
			app.Save(invRecord)
		} else {
			col, _ := app.FindCollectionByNameOrId("inventory")
			newInv := core.NewRecord(col)
			newInv.Set("company", companyId)
			newInv.Set("item", data.ItemId)
			newInv.Set("quantity", data.Quantity)
			app.Save(newInv)
		}

		return c.JSON(200, map[string]interface{}{
			"success": true,
			"message": "Retrait réussi",
		})
	})

	// GET /api/reserve/overview - Get reserve capacity overview
	e.Router.GET("/api/reserve/overview", func(c *core.RequestEvent) error {
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

		var usedSpace float64
		app.DB().Select("SUM(quantity)").From("reserve").Where(dbx.HashExp{"company": companyId}).Row(&usedSpace)

		level := company.GetInt("level")
		maxCapacity := float64(level * 300)

		return c.JSON(200, map[string]interface{}{
			"used": usedSpace,
			"max":  maxCapacity,
		})
	})
}
