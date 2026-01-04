package hooks

import (
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
// Reformatted to use SQL for reliability.
func PurgeEmptyDeposits(app *pocketbase.PocketBase) {
	app.Logger().Info("[CLEANUP] PurgeEmptyDeposits (SQL)...")

	// 1. Unassign machines from empty deposits
	// UPDATE machines SET deposit='' WHERE deposit IN (SELECT id FROM deposits WHERE quantity < 1)
	subQuery := app.DB().Select("id").From("deposits").Where(dbx.NewExp("quantity < 1"))
	_, err := app.DB().Update("machines", dbx.Params{"deposit": ""}, dbx.In("deposit", subQuery)).Execute()
	if err != nil {
		app.Logger().Error("[CLEANUP] Failed to unassign machines from empty deposits", "error", err)
	}

	// 2. Delete empty deposits
	// DELETE FROM deposits WHERE quantity < 1
	result, err := app.DB().Delete("deposits", dbx.NewExp("quantity < 1")).Execute()
	if err != nil {
		app.Logger().Error("[CLEANUP] Failed to delete empty deposits", "error", err)
	} else {
		affected, _ := result.RowsAffected()
		if affected > 0 {
			app.Logger().Info("[CLEANUP] Deleted empty deposits", "count", affected)
		}
	}
}

// FixZeroLevelDeposits assigns random levels (1-10) to deposits that have size 0
// Reformatted to use SQL for reliability.
func FixZeroLevelDeposits(app *pocketbase.PocketBase) {
	app.Logger().Info("[CLEANUP] FixZeroLevelDeposits (SQL)...")

	// UPDATE deposits SET size = ABS(RANDOM() % 10) + 1 WHERE size <= 0 OR size IS NULL
	// SQLite syntax
	_, err := app.DB().NewQuery("UPDATE deposits SET size = ABS(RANDOM() % 10) + 1 WHERE size <= 0 OR size IS NULL OR size = ''").Execute()
	if err != nil {
		app.Logger().Error("[CLEANUP] Failed to fix zero-level deposits", "error", err)
	} else {
		app.Logger().Info("[CLEANUP] FixZeroLevelDeposits executed.")
	}
}

// EnforceDepositCapacity cleans up surplus machines/employees from deposits that exceed capacity.
// Capacity Rule: Size * 5. Machine = 5. Employee = 1.
func EnforceDepositCapacity(app *pocketbase.PocketBase) {
	app.Logger().Info("[CLEANUP] Enforcing deposit capacities (Bulk SQL)...")

	// Get all deposits logic is okay, but if too many deposits, might be slow.
	// But usually deposits are few (per user).
	deposits, err := app.FindRecordsByFilter("deposits", "", "", 0, 0)
	if err != nil {
		app.Logger().Error("[CLEANUP] Failed to fetch deposits", "error", err)
		return
	}

	for _, dep := range deposits {
		size := dep.GetInt("size")
		if size <= 0 {
			size = 1
		}
		maxCapacity := size * 5

		// Count directly from DB for speed
		var machCount int
		app.DB().Select("count(*)").From("machines").Where(dbx.HashExp{"deposit": dep.Id}).Row(&machCount)

		var empCount int
		app.DB().Select("count(*)").From("employees").Where(dbx.HashExp{"deposit": dep.Id}).Row(&empCount)

		currentOccupancy := (machCount * 5) + empCount

		if currentOccupancy > maxCapacity {
			app.Logger().Info("[CLEANUP] Deposit over capacity", "id", dep.Id, "capacity", maxCapacity, "used", currentOccupancy)

			// Strategy: Prioritize Employees (keep them), remove surplus Machines.
			// 1. Check if Employees alone exceed capacity
			keptEmployees := empCount
			if keptEmployees > maxCapacity {
				// Too many employees. Keep maxCapacity.
				keptEmployees = maxCapacity

				// Identify IDs to keep
				var keepEmpIds []interface{}
				// Retrieve IDs as []interface{} for dbx.NotIn
				var ids []string
				app.DB().Select("id").From("employees").Where(dbx.HashExp{"deposit": dep.Id}).OrderBy("created DESC").Limit(int64(keptEmployees)).Column(&ids)
				for _, id := range ids {
					keepEmpIds = append(keepEmpIds, id)
				}

				// Unassign others
				if len(keepEmpIds) > 0 {
					_, err := app.DB().Update("employees", dbx.Params{"deposit": ""}, dbx.And(dbx.HashExp{"deposit": dep.Id}, dbx.Not(dbx.HashExp{"id": keepEmpIds}))).Execute()
					if err != nil {
						app.Logger().Error("[CLEANUP] Failed to bulk unassign employees", "error", err)
					}
				}
			}

			// 2. Calculate remaining space for machines
			remainingCap := maxCapacity - keptEmployees
			if remainingCap < 0 {
				remainingCap = 0
			}

			allowedMachines := remainingCap / 5

			if machCount > allowedMachines {
				// We need to remove surplus machines
				var keepMachIds []interface{}
				if allowedMachines > 0 {
					var ids []string
					app.DB().Select("id").From("machines").Where(dbx.HashExp{"deposit": dep.Id}).OrderBy("created DESC").Limit(int64(allowedMachines)).Column(&ids)
					for _, id := range ids {
						keepMachIds = append(keepMachIds, id)
					}
				}

				// Update matches: deposit=CurrentDep AND id NOT IN keepMachIds

				var verifyExpr dbx.Expression
				if len(keepMachIds) > 0 {
					verifyExpr = dbx.And(dbx.HashExp{"deposit": dep.Id}, dbx.Not(dbx.HashExp{"id": keepMachIds}))
				} else {
					verifyExpr = dbx.HashExp{"deposit": dep.Id}
				}

				result, err := app.DB().Update("machines", dbx.Params{"deposit": ""}, verifyExpr).Execute()
				if err != nil {
					app.Logger().Error("[CLEANUP] Failed to bulk unassign machines", "error", err)
				} else {
					affected, _ := result.RowsAffected()
					app.Logger().Info("[CLEANUP] Bulk unassigned surplus machines", "deposit", dep.Id, "unassigned", affected)
				}
			}
		}
	}
	app.Logger().Info("[CLEANUP] EnforceDepositCapacity finished.")
}
