package endpoints

import (
	"fmt"
	"math"

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

	// GET /api/company/energy-status - Get current energy production/consumption
	e.Router.GET("/api/company/energy-status", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Vous devez être connecté.", nil)
		}

		companyId := authRecord.GetString("active_company")
		if companyId == "" {
			return apis.NewBadRequestError("Aucune entreprise active", nil)
		}

		// Trigger Game Loop (Lazy Update)
		if graph != nil {
			if _, err := graph.CalculateCompanyInventory(companyId); err != nil {
				app.Logger().Error("[ENERGY] Failed to update game loop", "err", err)
			}
		}

		status, err := eco.CalculateEnergyStatus(companyId)
		if err != nil {
			return apis.NewBadRequestError(err.Error(), nil)
		}

		return c.JSON(200, status)
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

			// Check duplicates
			filter := fmt.Sprintf("company = '%s' && technology = '%s'", data.CompanyId, data.TechId)
			existing, _ := txApp.FindFirstRecordByFilter("company_techs", filter)
			if existing != nil {
				return apis.NewBadRequestError("Cette technologie est déjà acquise", nil)
			}

			// Check Balance
			cost := tech.Cost
			balance := company.GetFloat("balance")
			if balance < cost {
				return apis.NewBadRequestError(fmt.Sprintf("Fonds insuffisants. Requis: %.2f, Actuel: %.2f", cost, balance), nil)
			}

			// Create company_techs record
			collection, err := txApp.FindCollectionByNameOrId("company_techs")
			if err != nil {
				return err
			}
			newTech := core.NewRecord(collection)
			newTech.Set("company", data.CompanyId)
			newTech.Set("technology", data.TechId)

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
				"message": fmt.Sprintf("Technologie %s débloquée !", tech.Name),
				"cost":    cost,
			})
		})
	})
}
