package endpoints

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"ketsuna.com/server/internal/gamedata"
)

// registerGameDataEndpoints registers the /api/gamedata endpoint
func registerGameDataEndpoints(_ *pocketbase.PocketBase, e *core.ServeEvent) {

	// GET /api/gamedata - Returns all static game data (public endpoint)
	e.Router.GET("/api/gamedata", func(c *core.RequestEvent) error {
		// Return all static game data
		response := map[string]interface{}{
			"items":        gamedata.GetAllItems(),
			"recipes":      gamedata.GetAllRecipes(),
			"technologies": gamedata.GetAllTechnologies(),
		}

		// Set cache headers - data rarely changes
		c.Response.Header().Set("Cache-Control", "public, max-age=0")

		return c.JSON(200, response)
	})

	// GET /api/gamedata/items - Returns only items
	e.Router.GET("/api/gamedata/items", func(c *core.RequestEvent) error {
		c.Response.Header().Set("Cache-Control", "public, max-age=0")
		return c.JSON(200, gamedata.GetAllItems())
	})

	// GET /api/gamedata/recipes - Returns only recipes
	e.Router.GET("/api/gamedata/recipes", func(c *core.RequestEvent) error {
		c.Response.Header().Set("Cache-Control", "public, max-age=0")
		return c.JSON(200, gamedata.GetAllRecipes())
	})

	// GET /api/gamedata/technologies - Returns only technologies
	e.Router.GET("/api/gamedata/technologies", func(c *core.RequestEvent) error {
		c.Response.Header().Set("Cache-Control", "public, max-age=0")
		return c.JSON(200, gamedata.GetAllTechnologies())
	})
}
