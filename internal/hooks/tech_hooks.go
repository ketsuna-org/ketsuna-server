package hooks

import (
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
