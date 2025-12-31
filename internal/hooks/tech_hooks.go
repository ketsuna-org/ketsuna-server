package hooks

import (
	"fmt"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func registerCompanyTechHooks(app *pocketbase.PocketBase) {
	app.OnRecordCreateRequest("company_techs").BindFunc(func(e *core.RecordRequestEvent) error {
		r := e.Record
		companyId := r.GetString("company")
		techId := r.GetString("technology")

		if companyId == "" || techId == "" {
			return apis.NewBadRequestError("Company et Technology sont requis", nil)
		}

		company, err := app.FindRecordById("companies", companyId)
		if err != nil {
			return apis.NewBadRequestError("Company introuvable", nil)
		}
		tech, err := app.FindRecordById("technologies", techId)
		if err != nil {
			return apis.NewBadRequestError("Technology introuvable", nil)
		}

		reqLevel := tech.GetInt("required_level")

		currLevel := company.GetInt("level")

		if currLevel < reqLevel {
			return apis.NewBadRequestError(fmt.Sprintf("Niveau insuffisant. Niveau %d requis (vous êtes niveau %d)", reqLevel, currLevel), nil)
		}

		// Check duplicate
		filter := fmt.Sprintf("company = '%s' && technology = '%s'", companyId, techId)
		existing, _ := app.FindFirstRecordByFilter("company_techs", filter)
		if existing != nil {
			return apis.NewBadRequestError("Cette technologie est déjà acquise", nil)
		}

		if err := app.Save(company); err != nil {
			return err
		}

		return e.Next()
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
