package hooks

import (
	"fmt"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"ketsuna.com/server/internal/gamedata"
)

func registerCompanyTechHooks(app *pocketbase.PocketBase) {
	app.OnRecordCreateRequest("company_techs").BindFunc(func(e *core.RecordRequestEvent) error {
		return apis.NewForbiddenError("Veuillez utiliser l'API officielle pour débloquer une technologie.", nil)
	})

	app.OnRecordUpdateRequest("company_techs").BindFunc(func(e *core.RecordRequestEvent) error {
		// Allow Admin/Superuser
		if e.Auth != nil && e.Auth.IsSuperuser() {
			return e.Next()
		}
		return apis.NewBadRequestError("Impossible de modifier une technologie déjà achetée", nil)
	})

	app.OnRecordDeleteRequest("company_techs").BindFunc(func(e *core.RecordRequestEvent) error {
		r := e.Record

		// Use technology_id text field instead of technology relation
		techId := r.GetString("technology_id")

		// Use static gamedata for tech name
		techName := gamedata.GetTechnologyName(techId)
		app.Logger().Info("Technologie supprimée.", "technology", techName)

		return e.Next()
	})
}

// UpdateCompanyTechStatus checks for pending technologies that have completed their research time
// and updates them to "completed" status.
func UpdateCompanyTechStatus(app core.App, companyId string) error {
	// Find all pending techs for this company that should be finished
	// Filter: status = 'pending' AND completed_at <= now
	currentTime := time.Now().Format("2006-01-02 15:04:05.000Z")
	filter := fmt.Sprintf("company = '%s' && status = 'pending' && completed_at <= '%s'", companyId, currentTime)

	records, err := app.FindRecordsByFilter("company_techs", filter, "", 0, 0)
	if err != nil {
		return err
	}

	if len(records) == 0 {
		return nil
	}

	for _, record := range records {
		record.Set("status", "completed")
		if err := app.Save(record); err != nil {
			app.Logger().Error("[TECH] Failed to auto-complete research", "id", record.Id, "err", err)
		} else {
			app.Logger().Info("[TECH] Research completed", "tech", record.GetString("technology_id"), "company", companyId)
		}
	}

	return nil
}
