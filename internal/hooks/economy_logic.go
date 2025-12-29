package hooks

import (
	"github.com/pocketbase/pocketbase"
)

type EconomyLogic struct {
	app       *pocketbase.PocketBase
	inventory *InventoryLogic
}

func NewEconomyLogic(app *pocketbase.PocketBase, inv *InventoryLogic) *EconomyLogic {
	return &EconomyLogic{
		app:       app,
		inventory: inv,
	}
}
