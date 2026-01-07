package main

import (
	"log"
	"os"

	"ketsuna.com/server/internal/endpoints"
	"ketsuna.com/server/internal/hooks"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func main() {

	app := pocketbase.New()

	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		// serves static files from the provided public dir (if exists)
		se.Router.GET("/{path...}", apis.Static(os.DirFS("./pb_public"), false))

		return se.Next()
	})

	// Get the logic handlers from hooks
	inv, eco, emp, graph := hooks.GetLogicHandlers(app)

	// Register PocketBase hooks (companies, employees, inventory, recipes, cron)
	hooks.RegisterHooks(app, inv, eco, emp, graph)

	// Register custom API endpoints (separated for better organization)
	endpoints.RegisterAll(app, inv, eco, emp, graph)

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
