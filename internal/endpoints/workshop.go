package endpoints

import (
	"ketsuna.com/server/internal/hooks"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// registerWorkshopEndpoints handles /api/workshop/* routes
func registerWorkshopEndpoints(app *pocketbase.PocketBase, e *core.ServeEvent, inv *hooks.InventoryLogic) {

	// POST /api/workshop/produce - Manual crafting
	e.Router.POST("/api/workshop/produce", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Vous devez être connecté.", nil)
		}

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

		companyId := authRecord.GetString("active_company")
		if companyId == "" {
			return apis.NewBadRequestError("Aucune entreprise active pour cet utilisateur", nil)
		}

		company, err := app.FindRecordById("companies", companyId)
		if err != nil {
			return apis.NewBadRequestError("Entreprise introuvable", nil)
		}
		if company.GetString("ceo") != authRecord.Id && !authRecord.IsSuperuser() {
			return apis.NewForbiddenError("Seul le PDG peut lancer une production manuelle", nil)
		}

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

	// POST /api/machines/auto-assign-deposits
	e.Router.POST("/api/machines/auto-assign-deposits", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Vous devez être connecté.", nil)
		}

		companyId := authRecord.GetString("active_company")
		if companyId == "" {
			return apis.NewBadRequestError("Aucune entreprise active", nil)
		}

		// Security check: is user CEO?
		company, err := app.FindRecordById("companies", companyId)
		if err != nil {
			return apis.NewBadRequestError("Entreprise introuvable", nil)
		}
		if company.GetString("ceo") != authRecord.Id && !authRecord.IsSuperuser() {
			return apis.NewForbiddenError("Seul le PDG peut gérer les machines", nil)
		}

		return app.RunInTransaction(func(txApp core.App) error {
			// AutoAssignDeposits has been removed - deposits are no longer used for mining.
			// Deposit-to-machine connections now use edge_relation exclusively.
			// No automatic assignment needed.
			return c.JSON(200, map[string]interface{}{
				"success":       true,
				"assignedCount": 0,
				"message":       "Assignation automatique n'est plus disponible. Utilisez les edges pour connecter les machines aux gisements.",
			})
		})
	})
}
