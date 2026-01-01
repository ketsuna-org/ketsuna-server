package endpoints

import (
	"fmt"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func registerExplorationEndpoints(app *pocketbase.PocketBase, e *core.ServeEvent) {

	// POST /api/exploration/start - Start a new exploration mission
	e.Router.POST("/api/exploration/start", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Non connecté", nil)
		}

		data := struct {
			TargetResourceId string `json:"targetResourceId"`
		}{}
		if err := c.BindBody(&data); err != nil {
			return apis.NewBadRequestError("Invalid JSON", err)
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

			// Validate CEO
			if company.GetString("ceo") != authRecord.Id {
				return apis.NewForbiddenError("Seul le PDG peut lancer une exploration", nil)
			}

			// Fetch target resource item
			item, err := txApp.FindRecordById("items", data.TargetResourceId)
			if err != nil {
				return apis.NewBadRequestError("Ressource cible introuvable", nil)
			}

			// Verify it is explorable
			if !item.GetBool("is_explorable") {
				return apis.NewBadRequestError("Cet item ne peut pas être exploré", nil)
			}

			// --- Cost Logic ---
			// Base cost could be dynamic. For now, let's say 5000 credits per mission.
			cost := 5000

			// Optional: Check if user has specific tech to reduce cost or allow exploration
			// (Skipped for V1)

			currentBalance := company.GetInt("balance")
			if currentBalance < cost {
				return apis.NewBadRequestError(fmt.Sprintf("Fonds insuffisants. Coût: %d, Solde: %d", cost, currentBalance), nil)
			}

			// Deduct cost
			company.Set("balance", currentBalance-cost)
			if err := txApp.Save(company); err != nil {
				return apis.NewBadRequestError("Erreur lors du paiement", err)
			}

			// Create Exploration Record
			explorationsCollection, err := txApp.FindCollectionByNameOrId("explorations")
			if err != nil {
				return err
			}

			record := core.NewRecord(explorationsCollection)
			record.Set("company", companyId)
			record.Set("target_resource", data.TargetResourceId)
			record.Set("status", "En cours") // Enum: "En cours", "Succès", "Echec"

			// Calculate Duration based on Tech Level
			// Formula: Level * 5 minutes. Default Level 1.
			baseDurationPerLevel := 5 * time.Minute
			level := 1

			// Find technology that unlocks this item (Inverse relation lookup)
			// We look for a technology where 'item_unlocked' contains the target item ID
			tech, err := txApp.FindFirstRecordByFilter(
				"technologies",
				fmt.Sprintf("item_unlocked ~ '%s'", data.TargetResourceId),
			)

			if err == nil && tech != nil {
				reqLevel := tech.GetInt("required_level")
				if reqLevel > 1 {
					level = reqLevel
				}
			}

			duration := time.Duration(level) * baseDurationPerLevel
			record.Set("end_time", time.Now().Add(duration))

			if err := txApp.Save(record); err != nil {
				return apis.NewBadRequestError("Impossible de démarrer l'exploration", err)
			}

			return c.JSON(200, map[string]interface{}{
				"success": true,
				"message": "Exploration lancée !",
				"cost":    cost,
				"endTime": record.GetDateTime("end_time"),
			})
		})
	})

	// GET /api/explorations - List active explorations for the company
	e.Router.GET("/api/explorations", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Non connecté", nil)
		}
		companyId := authRecord.GetString("active_company")

		records, err := app.FindRecordsByFilter(
			"explorations",
			fmt.Sprintf("company = '%s' && status = 'En cours'", companyId),
			"-created",
			100,
			0,
		)
		if err != nil {
			return apis.NewBadRequestError("Erreur list", err)
		}

		// Expand resource info
		for _, r := range records {
			app.ExpandRecord(r, []string{"target_resource"}, nil)
		}

		return c.JSON(200, records)
	})
}
