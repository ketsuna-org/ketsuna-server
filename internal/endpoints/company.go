package endpoints

import (
	"fmt"
	"math"
	"time"

	"ketsuna.com/server/internal/gamedata"
	"ketsuna.com/server/internal/hooks"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// registerCompanyEndpoints handles /api/company/* routes
func registerCompanyEndpoints(app *pocketbase.PocketBase, e *core.ServeEvent, eco *hooks.EconomyLogic, graph *hooks.GraphEconomy) {

	// POST /api/company/finance - Get company financial breakdown
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

	// POST /api/company/levelup - Level up a company
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

	// POST /api/company/unlock-tech - Unlock a technology for company
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

			tech := gamedata.GetTechnology(data.TechId)

			// Check Requirements
			reqLevel := tech.RequiredLevel
			currLevel := company.GetInt("level")
			if currLevel < reqLevel {
				return apis.NewBadRequestError(fmt.Sprintf("Niveau insuffisant. Niveau %d requis (vous êtes niveau %d)", reqLevel, currLevel), nil)
			}

			// Check duplicates or in-progress
			filter := fmt.Sprintf("company = '%s' && technology_id = '%s'", data.CompanyId, data.TechId)
			existing, _ := txApp.FindFirstRecordByFilter("company_techs", filter)
			if existing != nil {
				status := existing.GetString("status")
				if status == "completed" {
					return apis.NewBadRequestError("Cette technologie est déjà acquise", nil)
				}
				if status == "pending" {
					return apis.NewBadRequestError("Cette technologie est déjà en cours de recherche", nil)
				}
			}

			// Check Balance
			cost := tech.Cost
			balance := company.GetFloat("balance")
			if balance < cost {
				return apis.NewBadRequestError(fmt.Sprintf("Fonds insuffisants. Requis: %.2f, Actuel: %.2f", cost, balance), nil)
			}

			// Check Required Items (inventory for regular items, machines for machine items)
			for _, reqItem := range tech.RequiredItems {
				itemDef := gamedata.GetItem(reqItem.ItemID)
				itemName := gamedata.GetItemName(reqItem.ItemID)

				if itemDef != nil && itemDef.Type == gamedata.ItemTypeMachine {
					// Check in machines collection
					machineFilter := fmt.Sprintf("company = '%s' && machine_id = '%s'", data.CompanyId, reqItem.ItemID)
					machines, _ := txApp.FindRecordsByFilter("machines", machineFilter, "", 0, 0)
					if len(machines) < reqItem.Quantity {
						return apis.NewBadRequestError(fmt.Sprintf("Machines requises manquantes: %dx %s (vous avez %d)", reqItem.Quantity, itemName, len(machines)), nil)
					}
				} else {
					// Check in inventory
					invFilter := fmt.Sprintf("company = '%s' && item_id = '%s'", data.CompanyId, reqItem.ItemID)
					invRecord, _ := txApp.FindFirstRecordByFilter("inventory", invFilter)
					if invRecord == nil {
						return apis.NewBadRequestError(fmt.Sprintf("Item requis manquant: %dx %s", reqItem.Quantity, itemName), nil)
					}
					qty := invRecord.GetFloat("quantity")
					if qty < float64(reqItem.Quantity) {
						return apis.NewBadRequestError(fmt.Sprintf("Quantité insuffisante: %dx %s (vous avez %.0f)", reqItem.Quantity, itemName, qty), nil)
					}
				}
			}

			// Consume Required Items
			for _, reqItem := range tech.RequiredItems {
				itemDef := gamedata.GetItem(reqItem.ItemID)

				if itemDef != nil && itemDef.Type == gamedata.ItemTypeMachine {
					// Delete machines from machines collection
					machineFilter := fmt.Sprintf("company = '%s' && machine_id = '%s'", data.CompanyId, reqItem.ItemID)
					machines, _ := txApp.FindRecordsByFilter("machines", machineFilter, "", reqItem.Quantity, 0)
					for _, machine := range machines {
						txApp.Delete(machine)
					}
				} else {
					// Deduct from inventory
					invFilter := fmt.Sprintf("company = '%s' && item_id = '%s'", data.CompanyId, reqItem.ItemID)
					invRecord, _ := txApp.FindFirstRecordByFilter("inventory", invFilter)
					if invRecord != nil {
						newQty := invRecord.GetFloat("quantity") - float64(reqItem.Quantity)
						invRecord.Set("quantity", newQty)
						txApp.Save(invRecord)
					}
				}
			}

			// Create company_techs record
			collection, err := txApp.FindCollectionByNameOrId("company_techs")
			if err != nil {
				return err
			}
			newTech := core.NewRecord(collection)
			newTech.Set("company", data.CompanyId)
			newTech.Set("technology_id", data.TechId)

			// Handle timed unlock
			if tech.UnlockTime > 0 {
				newTech.Set("status", "pending")
				completedAt := time.Now().Add(time.Duration(tech.UnlockTime) * time.Second)
				newTech.Set("completed_at", completedAt)

				if err := txApp.Save(newTech); err != nil {
					return apis.NewBadRequestError(fmt.Sprintf("Erreur lors du déblocage: %v", err), err)
				}

				// Deduct Balance
				company.Set("balance", balance-cost)
				if err := txApp.Save(company); err != nil {
					return apis.NewBadRequestError("Erreur lors du paiement", err)
				}

				return c.JSON(200, map[string]interface{}{
					"success":      true,
					"message":      fmt.Sprintf("Recherche de %s lancée!", tech.Name),
					"status":       "pending",
					"unlock_time":  tech.UnlockTime,
					"completed_at": completedAt,
					"cost":         cost,
				})
			}

			// Instant unlock
			newTech.Set("status", "completed")
			newTech.Set("completed_at", time.Now())

			if err := txApp.Save(newTech); err != nil {
				return apis.NewBadRequestError(fmt.Sprintf("Erreur lors du déblocage: %v", err), err)
			}

			// Deduct Balance
			company.Set("balance", balance-cost)
			if err := txApp.Save(company); err != nil {
				return apis.NewBadRequestError("Erreur lors du paiement", err)
			}

			return c.JSON(200, map[string]interface{}{
				"success": true,
				"message": fmt.Sprintf("Technologie %s débloquée!", tech.Name),
				"status":  "completed",
				"cost":    cost,
			})
		})
	})
}
