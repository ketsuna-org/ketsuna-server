package hooks

import (
	"fmt"
	"math"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

func RegisterEndpoints(app *pocketbase.PocketBase, inv *InventoryLogic, eco *EconomyLogic, emp *EmployeeLogic) {
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
			return app.RunInTransaction(func(txApp core.App) error {
				result, err := inv.ProduceItem(txApp, companyId, data.RecipeId, data.Quantity)
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

			return app.RunInTransaction(func(txApp core.App) error {
				company, err := txApp.FindRecordById("companies", companyId)
				if err != nil {
					return apis.NewBadRequestError("Entreprise introuvable", nil)
				}
				if company.GetString("ceo") != authRecord.Id {
					return apis.NewForbiddenError("Accès refusé", nil)
				}

				// Logic
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

				// Check existing
				existing, _ := txApp.FindFirstRecordByFilter("inventory", fmt.Sprintf("company='%s' && item='%s'", companyId, data.ItemId))
				if existing != nil {
					// Update
					curr := existing.GetInt("quantity")
					existing.Set("quantity", curr+data.Quantity)
					if err := txApp.Save(existing); err != nil {
						return apis.NewBadRequestError("Erreur mise à jour inventaire", err)
					}
				} else {
					// Create
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

				// Deduct
				company.Set("balance", current-totalCost)
				if err := txApp.Save(company); err != nil {
					// Revert is automatic in transaction if we return error
					return apis.NewBadRequestError("Erreur sauvegarde entreprise", err)
				}

				app.Logger().Info("[PURCHASE] Company purchased item", "companyId", companyId, "itemId", data.ItemId, "qty", data.Quantity, "totalCost", totalCost)

				// Decrement circulating_supply (Nexa-Bank stock consumed)
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

		// COMPANY ENERGY STATUS
		e.Router.GET("/api/company/energy-status", func(c *core.RequestEvent) error {
			authRecord := c.Auth
			if authRecord == nil {
				return apis.NewUnauthorizedError("Vous devez être connecté.", nil)
			}

			companyId := authRecord.GetString("active_company")
			if companyId == "" {
				return apis.NewBadRequestError("Aucune entreprise active", nil)
			}

			status, err := eco.CalculateEnergyStatus(companyId)
			if err != nil {
				return apis.NewBadRequestError(err.Error(), nil)
			}

			return c.JSON(200, status)
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

			return app.RunInTransaction(func(txApp core.App) error {
				company, err := txApp.FindRecordById("companies", companyId)
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

				if balance < cost {
					return apis.NewBadRequestError(fmt.Sprintf("Fonds insuffisants. Coût: %d€, Solde: %d€", cost, balance), nil)
				}

				company.Set("balance", balance-cost)
				company.Set("level", currentLevel+1)
				if err := txApp.Save(company); err != nil {
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
		})

		// EMPLOYEE HIRE Custom Endpoint (with bulk support)
		e.Router.POST("/api/employees/hire", func(c *core.RequestEvent) error {
			authRecord := c.Auth
			if authRecord == nil {
				return apis.NewUnauthorizedError("Vous devez être connecté.", nil)
			}

			data := struct {
				CompanyId string `json:"companyId" form:"companyId"`
				Quantity  int    `json:"quantity" form:"quantity"`
			}{}
			if err := c.BindBody(&data); err != nil {
				return apis.NewBadRequestError("Corps JSON invalide", err)
			}

			companyId := data.CompanyId
			if companyId == "" {
				return apis.NewBadRequestError("companyId requis", nil)
			}

			quantity := data.Quantity
			if quantity <= 0 {
				quantity = 1
			}
			if quantity > 50 {
				quantity = 50 // Max 50 per batch
			}

			return app.RunInTransaction(func(txApp core.App) error {
				// Validate CEO ownership
				company, err := txApp.FindRecordById("companies", companyId)
				if err != nil {
					return apis.NewBadRequestError("Company introuvable", nil)
				}

				if !authRecord.IsSuperuser() && company.GetString("ceo") != authRecord.Id {
					return apis.NewForbiddenError("Vous n'êtes pas le PDG de cette entreprise", nil)
				}

				// Hire multiple employees
				hiredRecords := []*core.Record{}
				totalCost := 0
				errors := []string{}

				for i := 0; i < quantity; i++ {
					hired, err := emp.HireEmployee(txApp, companyId)
					if err != nil {
						errors = append(errors, err.Error())
						// Stop on first error (usually balance insufficient)
						// If we error here, do we want to commit previous hires?
						// "RunInTransaction" rolls back on error unless we return nil!
						// If we want partial success, we shouldn't use one big transaction, OR we swallow error.
						// BUT for atomicity, "All or Nothing" is safer for "bulk hire".
						// Let's go All or Nothing.
						return apis.NewBadRequestError(err.Error(), nil)
					}
					hiredRecords = append(hiredRecords, hired.Record)
					totalCost += hired.Cost
				}

				if len(hiredRecords) == 0 {
					// Should have been caught by loop error return, but safety check
					return apis.NewBadRequestError("Aucun employé recruté", nil)
				}

				return c.JSON(200, map[string]interface{}{
					"success":    true,
					"message":    fmt.Sprintf("%d employé(s) recruté(s) avec succès", len(hiredRecords)),
					"records":    hiredRecords,
					"totalCost":  totalCost,
					"hiredCount": len(hiredRecords),
					"errors":     errors,
				})
			})
		})

		// TECHNOLOGY UNLOCK Custom Endpoint
		e.Router.POST("/api/company/unlock-tech", func(c *core.RequestEvent) error {
			authRecord := c.Auth
			if authRecord == nil {
				return apis.NewUnauthorizedError("Vous devez être connecté.", nil)
			}

			data := struct {
				CompanyId string `json:"companyId" form:"companyId"`
				TechId    string `json:"techId" form:"techId"`
			}{}
			if err := c.BindBody(&data); err != nil {
				return apis.NewBadRequestError("Corps JSON invalide", err)
			}

			if data.CompanyId == "" || data.TechId == "" {
				return apis.NewBadRequestError("companyId et techId requis", nil)
			}

			return app.RunInTransaction(func(txApp core.App) error {
				company, err := txApp.FindRecordById("companies", data.CompanyId)
				if err != nil {
					return apis.NewBadRequestError("Entreprise introuvable", nil)
				}

				if !authRecord.IsSuperuser() && company.GetString("ceo") != authRecord.Id {
					return apis.NewForbiddenError("Vous n'êtes pas le PDG", nil)
				}

				tech, err := txApp.FindRecordById("technologies", data.TechId)
				if err != nil {
					return apis.NewBadRequestError("Technologie introuvable", nil)
				}

				// Check Requirements
				reqLevel := tech.GetInt("required_level")
				currLevel := company.GetInt("level")
				if currLevel < reqLevel {
					return apis.NewBadRequestError(fmt.Sprintf("Niveau insuffisant. Niveau %d requis (vous êtes niveau %d)", reqLevel, currLevel), nil)
				}

				// Check duplicates
				filter := fmt.Sprintf("company = '%s' && technology = '%s'", data.CompanyId, data.TechId)
				existing, _ := txApp.FindFirstRecordByFilter("company_techs", filter)
				if existing != nil {
					return apis.NewBadRequestError("Cette technologie est déjà acquise", nil)
				}

				// Check Balance
				cost := tech.GetFloat("cost")
				balance := company.GetFloat("balance")
				if balance < cost {
					return apis.NewBadRequestError(fmt.Sprintf("Fonds insuffisants. Requis: %.2f, Actuel: %.2f", cost, balance), nil)
				}

				// 1. Create company_techs record
				collection, err := txApp.FindCollectionByNameOrId("company_techs")
				if err != nil {
					return err
				}
				newTech := core.NewRecord(collection)
				newTech.Set("company", data.CompanyId)
				newTech.Set("technology", data.TechId)
				// Unlock Item?

				if err := txApp.Save(newTech); err != nil {
					return apis.NewBadRequestError(fmt.Sprintf("Erreur lors du déblocage: %v", err), err)
				}

				// 2. Deduct Balance
				company.Set("balance", balance-cost)
				if err := txApp.Save(company); err != nil {
					return apis.NewBadRequestError("Erreur lors du paiement", err)
				}

				return c.JSON(200, map[string]interface{}{
					"success": true,
					"message": fmt.Sprintf("Technologie %s débloquée !", tech.GetString("name")),
					"cost":    cost,
				})
			})
		})

		// EMPLOYEE PREVIEW COST Endpoint
		e.Router.GET("/api/employees/preview-cost", func(c *core.RequestEvent) error {
			// Return average hiring costs based on employee_logic.go constants
			// Common (40%): salary ~26, fee = 26*5 = 130, reserve = 26*30 = 780
			// Rare (30%): salary ~65, fee = 325, reserve = 1950
			// Epic (9%): salary ~130, fee = 650, reserve = 3900
			// Legendary (1%): salary ~260, fee = 1300, reserve = 7800

			// Weighted average: 0.4*130 + 0.3*325 + 0.09*650 + 0.01*1300 = 221
			avgHiringFee := 221
			avgReserve := 1326

			return c.JSON(200, map[string]interface{}{
				"averageHiringFee":     avgHiringFee,
				"averageReserveNeeded": avgReserve,
				"averageTotalRequired": avgHiringFee + avgReserve,
				"maxBulkHire":          50,
				"description":          "Coût moyen estimé pour embaucher un employé",
			})
		})

		// RESERVE Endpoints
		// ------------------

		// DEPOSIT to Reserve
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

			// 1. Check Inventory Ownership & Quantity
			invRecord, err := app.FindFirstRecordByFilter("inventory", fmt.Sprintf("company='%s' && item='%s'", companyId, data.ItemId))
			if err != nil || invRecord == nil {
				return apis.NewBadRequestError("Item non trouvé dans l'inventaire", nil)
			}
			currentQty := invRecord.GetInt("quantity")
			if currentQty < data.Quantity {
				return apis.NewBadRequestError("Quantité insuffisante en inventaire", nil)
			}

			// 2. Check Reserve Capacity
			// Get current used space
			var usedSpace float64
			err = app.DB().Select("SUM(quantity)").From("reserve").Where(dbx.HashExp{"company": companyId}).Row(&usedSpace)
			if err != nil {
				// Ignore error if no rows (means 0 used)
				usedSpace = 0
			}

			level := company.GetInt("level")
			maxCapacity := float64(level * 300)

			if (usedSpace + float64(data.Quantity)) > maxCapacity {
				return apis.NewBadRequestError(fmt.Sprintf("Capacité de réserve dépassée. Max: %.0f, Actuel: %.0f, Requis: +%d", maxCapacity, usedSpace, data.Quantity), nil)
			}

			// 3. EXECUTE TRANSFER (Atomic-ish via sequence of operations)

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
				// Update
				resRecord.Set("quantity", resRecord.GetInt("quantity")+data.Quantity)
				if err := app.Save(resRecord); err != nil {
					return apis.NewBadRequestError("Erreur sauvegarde réserve", err)
				}
			} else {
				// Create
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

		// WITHDRAW from Reserve
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

			// 1. Check Reserve Ownership & Quantity
			resRecord, err := app.FindFirstRecordByFilter("reserve", fmt.Sprintf("company='%s' && item='%s'", companyId, data.ItemId))
			if err != nil || resRecord == nil {
				return apis.NewBadRequestError("Item non trouvé dans la réserve", nil)
			}
			currentQty := resRecord.GetInt("quantity")
			if currentQty < data.Quantity {
				return apis.NewBadRequestError("Quantité insuffisante en réserve", nil)
			}

			// 2. Execute Transfer

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

		// GET Reserve Overview
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

			// Calculate Capacity
			var usedSpace float64
			err = app.DB().Select("SUM(quantity)").From("reserve").Where(dbx.HashExp{"company": companyId}).Row(&usedSpace)
			// error is acceptable if no rows

			level := company.GetInt("level")
			maxCapacity := float64(level * 300)

			return c.JSON(200, map[string]interface{}{
				"used": usedSpace,
				"max":  maxCapacity,
			})
		})

		// HARVEST Endpoints (Manual Mining/Gathering)
		// ------------------

		// GET Harvest Status
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

			// Get current harvest state
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

		// POST Start Harvest
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

			// Check if already harvesting
			if !company.GetDateTime("is_producing").IsZero() {
				return apis.NewBadRequestError("Une récolte est déjà en cours", nil)
			}

			// Verify item is minable
			item, err := app.FindRecordById("items", data.ItemId)
			if err != nil {
				return apis.NewBadRequestError("Item introuvable", nil)
			}

			if !item.GetBool("minable") {
				return apis.NewBadRequestError("Cet item ne peut pas être récolté manuellement", nil)
			}

			// Start harvest
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

		// POST Collect Harvest
		e.Router.POST("/api/harvest/collect", func(c *core.RequestEvent) error {
			authRecord := c.Auth
			if authRecord == nil {
				return apis.NewUnauthorizedError("Non connecté", nil)
			}

			companyId := authRecord.GetString("active_company")
			if companyId == "" {
				return apis.NewBadRequestError("Aucune entreprise active", nil)
			}

			// Run in transaction for Atomicity against Race Conditions
			return app.RunInTransaction(func(txApp core.App) error {
				// 1. Fetch company to verify ownership and initial state Check
				company, err := txApp.FindRecordById("companies", companyId)
				if err != nil {
					return apis.NewBadRequestError("Entreprise introuvable", nil)
				}

				if company.GetString("ceo") != authRecord.Id && !authRecord.IsSuperuser() {
					return apis.NewForbiddenError("Accès refusé", nil)
				}

				// 2. Snapshot Read
				isProducing := company.GetDateTime("is_producing")
				harvestingItemId := company.GetString("item_harvesting")

				if isProducing.IsZero() || harvestingItemId == "" {
					return apis.NewBadRequestError("Aucune récolte en cours (Snapshot)", nil)
				}

				item, err := txApp.FindRecordById("items", harvestingItemId)
				if err != nil {
					return apis.NewBadRequestError("Item introuvable", nil) // Should not happen
				}

				productionTime := item.GetInt("production_time")
				elapsed := time.Since(isProducing.Time()).Seconds()

				if elapsed < float64(productionTime) {
					remaining := float64(productionTime) - elapsed
					return apis.NewBadRequestError(fmt.Sprintf("Récolte pas encore terminée. %.0f secondes restantes", remaining), nil)
				}

				// 3. ATOMIC CLAIM via Conditional Update (Optimistic Locking)
				// We enforce that we are the one setting it to empty, verifying it is NOT empty right now.
				res, err := txApp.DB().NewQuery(`
					UPDATE companies 
					SET is_producing = '', item_harvesting = '' 
					WHERE id = {:id} AND is_producing != '' AND item_harvesting != ''
				`).Bind(dbx.Params{
					"id": company.Id,
				}).Execute()

				if err != nil {
					return apis.NewBadRequestError("Erreur base de données", err)
				}

				rows, _ := res.RowsAffected()
				if rows == 0 {
					// Race condition lost: Another request claimed it first
					return apis.NewBadRequestError("Récolte déjà effectuée par une autre requête", nil)
				}

				// 4. Update Inventory Logic Inline (using txApp for transaction safety)
				// (We cannot use inv.UpdateInventory() because it uses the global app instance, not the transaction)

				// Try Find inventory
				filter := fmt.Sprintf("company = '%s' && item = '%s'", companyId, harvestingItemId)
				invRecord, err := txApp.FindFirstRecordByFilter("inventory", filter)

				if err != nil {
					// Not found -> Create
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
					// Found -> Update
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

		return e.Next()
	})
}
