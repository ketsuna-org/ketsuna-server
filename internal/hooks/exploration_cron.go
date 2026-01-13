package hooks

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func RegisterExplorationCron(app *pocketbase.PocketBase) {
	// Run every minute
	app.Cron().Add("exploration_resolution", "* * * * *", func() {
		ResolveExplorations(app)
	})
}

func ResolveExplorations(app *pocketbase.PocketBase) {
	// Find all explorations "En cours" where end_time <= Now
	// Note: PB filter for Date can be tricky with timezone. We compare UTC.
	now := time.Now().UTC().Format("2006-01-02 15:04:05.000Z")

	records, err := app.FindRecordsByFilter(
		"explorations",
		fmt.Sprintf("status = 'En cours' && end_time <= '%s'", now),
		"",
		100, // Process batch of 100
		0,
	)

	if err != nil {
		app.Logger().Error("Error fetching explorations", "err", err)
		return
	}

	for _, exploration := range records {
		processExplorationResult(app, exploration)
	}
}

func processExplorationResult(app *pocketbase.PocketBase, exploration *core.Record) {
	// 50% chance of success for now
	// TODO: Adjust based on tech or company stats
	successRate := 0.40
	roll := rand.Float64()

	isSuccess := roll <= successRate

	if isSuccess {
		// Create Deposit
		companyId := exploration.GetString("company")
		resourceId := exploration.GetString("target_resource_id")
		distance := exploration.GetFloat("distance")

		// Default distance if not set
		if distance <= 0 {
			distance = 10
		}

		// Calculate Quantity based on distance and size
		// Formula: A 10km patch yields 1000 to 10000 resources based on size (1-10)
		// So: quantity = (distance / 10) * size * 1000
		// Size 1 at 10km = 1000, Size 10 at 10km = 10000
		// Size 5 at 20km = 2 * 5 * 1000 = 10000
		size := 1 + rand.Intn(10) // Random level 1-10
		baseMultiplier := distance / 10.0
		quantity := math.Floor(baseMultiplier * float64(size) * 1000)

		// Ensure minimum quantity of 100
		if quantity < 100 {
			quantity = 100
		}

		// Create record
		depositsCollection, _ := app.FindCollectionByNameOrId("deposits")
		deposit := core.NewRecord(depositsCollection)
		deposit.Set("company", companyId)
		deposit.Set("ressource_id", resourceId)
		deposit.Set("quantity", quantity)
		deposit.Set("size", size)
		deposit.Set("distance", distance) // Store distance on deposit for reference

		if err := app.Save(deposit); err != nil {
			app.Logger().Error("Failed to create deposit", "err", err)
			// Don't mark exploration as success if deposit failed?
			// Let's mark as failed for safety or retry?
			// We'll mark as Failed to avoid infinite loop
			exploration.Set("status", "Echec")
		} else {
			exploration.Set("status", "Succès")
			// Create notification message (optional)
			createNotification(app, companyId, "Exploration réussie !", fmt.Sprintf("Gisement découvert à %.0f km : %.0f unités (Niveau %d)", distance, quantity, size))
		}
	} else {
		exploration.Set("status", "Echec")
		companyId := exploration.GetString("company")
		createNotification(app, companyId, "Exploration échouée", "L'équipe n'a rien trouvé sur ce secteur.")
	}

	app.Save(exploration)
}

func createNotification(app *pocketbase.PocketBase, companyId string, title string, content string) {
	// Simple wrapper to create a message if messages collection exists and is linked to company (via user)
	// For now, we log it. Real notification system needs to find User ID from Company ID.

	company, err := app.FindRecordById("companies", companyId)
	if err != nil {
		return
	}
	userId := company.GetString("ceo")

	msgs, _ := app.FindCollectionByNameOrId("notifications")
	if msgs != nil {
		msg := core.NewRecord(msgs)
		msg.Set("user", userId)
		msg.Set("title", title)
		msg.Set("content", content)
		msg.Set("is_read", false)
		app.Save(msg)
	}
}
