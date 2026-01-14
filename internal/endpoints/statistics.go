package endpoints

import (
	"fmt"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"ketsuna.com/server/internal/gamedata"
)

// registerStatisticsEndpoints handles /api/company/statistics
func registerStatisticsEndpoints(app *pocketbase.PocketBase, e *core.ServeEvent) {

	// GET /api/company/statistics?period=1m|10m|1h|10h|24h|all
	e.Router.GET("/api/company/statistics", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Vous devez être connecté.", nil)
		}

		companyId := authRecord.GetString("active_company")
		if companyId == "" {
			return apis.NewBadRequestError("Aucune entreprise active", nil)
		}

		// Parse period parameter
		period := c.Request.URL.Query().Get("period")
		if period == "" {
			period = "10m"
		}

		var duration time.Duration
		allPeriod := false
		switch period {
		case "1m":
			duration = 1 * time.Minute
		case "10m":
			duration = 10 * time.Minute
		case "1h":
			duration = 1 * time.Hour
		case "10h":
			duration = 10 * time.Hour
		case "24h":
			duration = 24 * time.Hour
		case "all":
			allPeriod = true
		default:
			duration = 10 * time.Minute
		}

		var records []*core.Record
		var err error

		if allPeriod {
			filter := fmt.Sprintf("company = '%s'", companyId)

			// Fetch all records in batches to avoid missing older history
			const batchSize = 500
			for offset := 0; ; offset += batchSize {
				batch, batchErr := app.FindRecordsByFilter("company_statistics", filter, "-created", batchSize, offset)
				if batchErr != nil {
					return apis.NewBadRequestError("Erreur lors de la récupération des statistiques", batchErr)
				}

				records = append(records, batch...)
				if len(batch) < batchSize {
					break
				}
			}
		} else {
			cutoffTime := time.Now().Add(-duration)
			cutoffStr := cutoffTime.UTC().Format("2006-01-02 15:04:05.000Z")

			filter := fmt.Sprintf("company = '%s' && created >= '%s'", companyId, cutoffStr)
			records, err = app.FindRecordsByFilter("company_statistics", filter, "-created", 1000, 0)
			if err != nil {
				return apis.NewBadRequestError("Erreur lors de la récupération des statistiques", err)
			}
		}

		// Aggregate by item_id and event_type
		productionMap := make(map[string]float64)
		consumptionMap := make(map[string]float64)
		moneyIn := 0.0
		moneyOut := 0.0

		for _, record := range records {
			eventType := record.GetString("event_type")
			itemId := record.GetString("item_id")
			quantity := record.GetFloat("quantity")

			switch eventType {
			case "production":
				productionMap[itemId] += quantity
			case "consumption":
				consumptionMap[itemId] += quantity
			case "money_in":
				moneyIn += quantity
			case "money_out":
				moneyOut += quantity
			}
		}

		// Calculate rates per minute
		durationMinutes := duration.Minutes()
		if allPeriod {
			durationMinutes = 1
			if len(records) > 0 {
				oldest := records[len(records)-1].GetDateTime("created")
				span := time.Since(oldest.Time()).Minutes()
				if span > 0 {
					durationMinutes = span
				}
			}
		}
		if durationMinutes <= 0 {
			durationMinutes = 1
		}

		// Build production array
		production := make([]map[string]interface{}, 0)
		for itemId, qty := range productionMap {
			item := gamedata.GetItem(itemId)
			name := itemId
			if item != nil {
				name = item.Name
			}
			production = append(production, map[string]interface{}{
				"item_id":         itemId,
				"name":            name,
				"quantity":        qty,
				"rate_per_minute": qty / durationMinutes,
			})
		}

		// Build consumption array
		consumption := make([]map[string]interface{}, 0)
		for itemId, qty := range consumptionMap {
			item := gamedata.GetItem(itemId)
			name := itemId
			if item != nil {
				name = item.Name
			}
			consumption = append(consumption, map[string]interface{}{
				"item_id":         itemId,
				"name":            name,
				"quantity":        qty,
				"rate_per_minute": qty / durationMinutes,
			})
		}

		return c.JSON(200, map[string]interface{}{
			"success":     true,
			"period":      period,
			"production":  production,
			"consumption": consumption,
			"money": map[string]interface{}{
				"income":   moneyIn,
				"expenses": moneyOut,
				"net":      moneyIn - moneyOut,
			},
		})
	})
}
