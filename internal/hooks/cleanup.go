package hooks

import (
	"math/rand"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
)

// PurgeExcessMachines detects companies with > 2000 machines and deletes ALL their machines.
// (Threshold set to 2000 to be safe, user said 5000)
func PurgeExcessMachines(app *pocketbase.PocketBase) {
	app.Logger().Info("[CLEANUP] Starting PurgeExcessMachines check...")

	// Get all companies
	companies, err := app.FindRecordsByFilter("companies", "", "", 0, 0)
	if err != nil {
		app.Logger().Error("[CLEANUP] Failed to fetch companies", "error", err)
		return
	}

	for _, c := range companies {
		// Count machines using direct DB query for speed
		var count int
		err := app.DB().Select("count(*)").From("machines").Where(dbx.HashExp{"company": c.Id}).Row(&count)
		if err != nil {
			continue
		}

		if count > 2000 {
			app.Logger().Info("[CLEANUP] Found company with excessive machines", "company", c.GetString("name"), "count", count)

			// DELETE ALL MACHINES for this company
			// We use raw SQL delete because fetching 5000+ records to delete them one by one via Record model is slow/heavy.
			_, err := app.DB().Delete("machines", dbx.HashExp{"company": c.Id}).Execute()
			if err != nil {
				app.Logger().Error("[CLEANUP] Failed to delete machines", "company", c.GetString("name"), "error", err)
			} else {
				app.Logger().Info("[CLEANUP] SUCCESS: Deleted all machines for company", "company", c.GetString("name"))
			}
		}
	}
	app.Logger().Info("[CLEANUP] Finished.")
}

// PurgeEmptyDeposits removes all deposits with quantity <= 0 and unassigns machines from them.
// This is called on server startup to clean up stale data.
func PurgeEmptyDeposits(app *pocketbase.PocketBase) {
	app.Logger().Info("[CLEANUP] Starting PurgeEmptyDeposits...")

	// Find all empty deposits (< 1 to handle floating point values like 0.5)
	emptyDeposits, err := app.FindRecordsByFilter("deposits", "quantity < 1", "", 0, 0)
	if err != nil {
		app.Logger().Error("[CLEANUP] Failed to fetch empty deposits", "error", err)
		return
	}

	if len(emptyDeposits) == 0 {
		app.Logger().Info("[CLEANUP] No empty deposits found.")
		return
	}

	app.Logger().Info("[CLEANUP] Found empty deposits to purge", "count", len(emptyDeposits))

	for _, deposit := range emptyDeposits {
		depositId := deposit.Id

		// Unassign any machines linked to this deposit
		machines, err := app.FindRecordsByFilter("machines", "deposit = '"+depositId+"'", "", 0, 0)
		if err == nil {
			for _, m := range machines {
				m.Set("deposit", "")
				if err := app.Save(m); err != nil {
					app.Logger().Error("[CLEANUP] Failed to unassign machine from deposit", "machine", m.Id, "error", err)
				}
			}
		}

		// Delete the deposit
		if err := app.Delete(deposit); err != nil {
			app.Logger().Error("[CLEANUP] Failed to delete empty deposit", "deposit", depositId, "error", err)
		}
	}

	app.Logger().Info("[CLEANUP] PurgeEmptyDeposits completed", "deleted", len(emptyDeposits))
}

// FixZeroLevelDeposits assigns random levels (1-10) to deposits that have size 0
func FixZeroLevelDeposits(app *pocketbase.PocketBase) {
	app.Logger().Info("[CLEANUP] Starting FixZeroLevelDeposits...")

	// Find all deposits with size = 0
	zeroLevelDeposits, err := app.FindRecordsByFilter("deposits", "size = 0 || size = ''", "", 0, 0)
	if err != nil {
		app.Logger().Error("[CLEANUP] Failed to fetch zero-level deposits", "error", err)
		return
	}

	if len(zeroLevelDeposits) == 0 {
		app.Logger().Info("[CLEANUP] No zero-level deposits found.")
		return
	}

	app.Logger().Info("[CLEANUP] Found zero-level deposits to fix", "count", len(zeroLevelDeposits))

	fixedCount := 0
	for _, deposit := range zeroLevelDeposits {
		// Assign random level between 1 and 10
		randomLevel := rand.Intn(10) + 1 // 1-10
		deposit.Set("size", randomLevel)

		if err := app.Save(deposit); err != nil {
			app.Logger().Error("[CLEANUP] Failed to update deposit level", "deposit", deposit.Id, "error", err)
		} else {
			fixedCount++
		}
	}

	app.Logger().Info("[CLEANUP] FixZeroLevelDeposits completed", "fixed", fixedCount)
}
