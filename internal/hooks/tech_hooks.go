package hooks

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
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

		techId := r.GetString("technology")

		tech, err := app.FindRecordById("technologies", techId)
		if err != nil {
			return nil
		}

		app.Logger().Info("Technologie supprimée.", "technology", tech.GetString("name"))
		return e.Next()
	})
}
