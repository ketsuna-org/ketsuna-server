package hooks

import (
	"github.com/pocketbase/pocketbase/core"
)

// RecordCompanyStatistic records a financial or resource statistic for a company
// eventType options: "production", "consumption", "money_in", "money_out"
func RecordCompanyStatistic(app core.App, companyId, itemId, eventType string, quantity float64) {
	if quantity <= 0 || companyId == "" {
		return
	}

	collection, err := app.FindCollectionByNameOrId("company_statistics")
	if err != nil {
		if app.Logger() != nil {
			app.Logger().Error("[STATS] Failed to find company_statistics collection", "err", err)
		}
		return
	}

	record := core.NewRecord(collection)
	record.Set("company", companyId)
	record.Set("item_id", itemId)
	record.Set("event_type", eventType)
	record.Set("quantity", quantity)

	if err := app.Save(record); err != nil {
		if app.Logger() != nil {
			app.Logger().Error("[STATS] Failed to save statistic", "err", err)
		}
	}
}
