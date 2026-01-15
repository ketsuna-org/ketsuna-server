package endpoints

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func registerDepositEndpoints(app *pocketbase.PocketBase, e *core.ServeEvent) {
	// NOTE: Deposit employee assignment endpoints have been removed.
	// Employees are no longer used in the mining system - only for explorations.
	// Deposits are now pure sources for mining machines via edges.
}
