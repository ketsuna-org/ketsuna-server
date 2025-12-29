package hooks

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func registerRecipeHooks(app *pocketbase.PocketBase) {
	// recipes are admin-only for create/update/delete
	app.OnRecordCreateRequest("recipes").BindFunc(func(e *core.RecordRequestEvent) error {
		info, err := e.RequestInfo()
		if err != nil || !info.HasSuperuserAuth() {
			return apis.NewBadRequestError("Seuls les administrateurs peuvent créer des recettes", nil)
		}
		return nil
	})

	app.OnRecordUpdateRequest("recipes").BindFunc(func(e *core.RecordRequestEvent) error {
		info, err := e.RequestInfo()
		if err != nil || !info.HasSuperuserAuth() {
			return apis.NewBadRequestError("Seuls les administrateurs peuvent modifier des recettes", nil)
		}
		return nil
	})

	app.OnRecordDeleteRequest("recipes").BindFunc(func(e *core.RecordRequestEvent) error {
		info, err := e.RequestInfo()
		if err != nil || !info.HasSuperuserAuth() {
			return apis.NewBadRequestError("Seuls les administrateurs peuvent supprimer des recettes", nil)
		}
		return nil
	})
}
