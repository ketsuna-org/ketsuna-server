package hooks

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"ketsuna.com/server/internal/gamedata"
)

func registerMachineHooks(app *pocketbase.PocketBase, inv *InventoryLogic, graph *GraphEconomy) {
	// Hook for Lazy Evaluation on View
	app.OnRecordViewRequest("machines").BindFunc(func(e *core.RecordRequestEvent) error {
		if graph != nil {
			graph.CalculateNodeFlow(e.Record.Id, "machine")
		}
		return e.Next()
	})

	// Hook for Lazy Evaluation on List
	// REMOVED: Optimizing to avoid N+1 queries.
	// GraphEconomy handles individual nodes updates via UI interaction or Global Tick.
	// Listing all machines should purely be a DB fetch.
	/*
		app.OnRecordsListRequest("machines").BindFunc(func(e *core.RecordsListRequestEvent) error {
			if graph != nil {
				for _, rec := range e.Records {
					graph.CalculateNodeFlow(rec.Id, "machine")
				}
			}
			return e.Next()
		})
	*/

	app.OnRecordCreateRequest("machines").BindFunc(func(e *core.RecordRequestEvent) error {
		record := e.Record
		companyId := record.GetString("company")
		machineItemId := record.GetString("machine_id")

		depositId := record.GetString("deposit")

		if companyId == "" || machineItemId == "" {
			return apis.NewBadRequestError("ID de compagnie ou de machine manquant.", nil)
		}

		// 1. Validate employees not assigned elsewhere
		// REMOVED: Employees are now assigned via employee record update, not machine record.
		// The endpoints/machines.go handles assignment validation.

		// 2. Check max_employee limit using static gamedata
		// REMOVED: Same reason.

		// 3. VALIDATE DEPOSIT CAPACITY (if assigning to deposit)
		if depositId != "" {
			deposit, err := app.FindRecordById("deposits", depositId)
			if err != nil {
				return apis.NewBadRequestError("Gisement introuvable.", nil)
			}

			size := deposit.GetInt("size")
			if size <= 0 {
				size = 1
			}

			maxMachines := gamedata.GetMaxMachinesForDeposit(size)

			// Count current machines on this deposit
			currentMachines, _ := app.FindRecordsByFilter("machines",
				fmt.Sprintf("deposit = '%s'", depositId), "", 0, 0)

			if len(currentMachines) >= maxMachines {
				return apis.NewBadRequestError(fmt.Sprintf(
					"Ce gisement (taille %d) ne peut accueillir que %d machine(s). Déjà %d assignée(s).",
					size, maxMachines, len(currentMachines)), nil)
			}
		}

		// 4. SET INITIAL DURABILITY AND START PRODUCTION (skip for storage)
		// Storage items don't have durability or production, they just store
		machineItem := gamedata.GetItem(machineItemId)
		if machineItem != nil && machineItem.Type != gamedata.ItemTypeStockage {
			// Initialize production timestamp so machines start producing immediately
			record.Set("production_started_at", time.Now())
		}

		// 5. ATOMIC: Check and Deduct Inventory using transaction
		// This prevents race conditions when multiple machines are created simultaneously
		var deductErr error
		txErr := app.RunInTransaction(func(txApp core.App) error {
			// Re-fetch inventory inside transaction for fresh data (using item_id)
			inventory, err := txApp.FindFirstRecordByFilter("inventory",
				fmt.Sprintf("company = '%s' && item_id = '%s'", companyId, machineItemId))
			if err != nil {
				deductErr = apis.NewBadRequestError("Vous n'avez pas cette machine en stock dans votre inventaire.", nil)
				return deductErr
			}

			currentQty := inventory.GetInt("quantity")
			if currentQty < 1 {
				deductErr = apis.NewBadRequestError("Stock insuffisant: vous n'avez plus cette machine en inventaire.", nil)
				return deductErr
			}

			// Deduct within transaction
			inventory.Set("quantity", currentQty-1)
			if err := txApp.Save(inventory); err != nil {
				deductErr = apis.NewBadRequestError("Erreur lors de la mise à jour de l'inventaire.", nil)
				return deductErr
			}

			app.Logger().Info("[MACHINES] Machine assigned (tx)", "machineId", machineItemId, "companyId", companyId, "remaining", currentQty-1)
			return nil
		})

		if txErr != nil || deductErr != nil {
			if deductErr != nil {
				return deductErr
			}
			return apis.NewBadRequestError("Erreur de transaction", txErr)
		}

		return e.Next()
	})

	// Hook to auto-start production when machine is placed
	app.OnRecordAfterUpdateSuccess("machines").BindFunc(func(e *core.RecordEvent) error {
		record := e.Record

		// Check if machine was just placed (placed changed to true)
		if record.GetBool("placed") {
			// Check if production_started_at is not set
			startedAt := record.GetDateTime("production_started_at")
			if startedAt.Time().IsZero() {
				// Skip storage items - they don't need production
				machineItemId := record.GetString("machine_id")
				machineItem := gamedata.GetItem(machineItemId)

				if machineItem != nil && machineItem.Type != gamedata.ItemTypeStockage {
					// Initialize production timestamp
					record.Set("production_started_at", time.Now())
					if err := app.Save(record); err != nil {
						app.Logger().Error("[MACHINES] Failed to auto-start production", "err", err)
					} else {
						app.Logger().Info("[MACHINES] Auto-started production for placed machine", "machineId", record.Id)
					}
				}
			}
		}

		return e.Next()
	})

	// REMOVED: OnRecordUpdateRequest validation for employees.
	// Since machines no longer have 'employees' field, this hook serves no purpose for employee validation.
	// If other updates need validation, add them here.
	// app.OnRecordUpdateRequest("machines").BindFunc(...)

	app.OnRecordDeleteRequest("machines").BindFunc(func(e *core.RecordRequestEvent) error {
		record := e.Record
		companyId := record.GetString("company")
		machineItemId := record.GetString("machine_id")

		if companyId != "" && machineItemId != "" {
			if err := inv.UpdateInventory(app, companyId, machineItemId, 1); err != nil {
				app.Logger().Error("[MACHINES] Erreur remise en stock", "error", err)
			} else {
				app.Logger().Info("[MACHINES] Assignation supprimée. Machine renvoyée au stock", "machineId", machineItemId)
			}
		}
		return e.Next()
	})
}

// EnforceMaxEmployees corrects machines that have more employees than max_employee allows
// This is a data correction function to fix past bugs
func EnforceMaxEmployees(app *pocketbase.PocketBase) {
	machines, err := app.FindRecordsByFilter("machines", "", "", 0, 0)
	if err != nil {
		app.Logger().Error("[FIX] Failed to fetch machines", "error", err)
		return
	}

	fixedCount := 0
	for _, machine := range machines {
		machineItemId := machine.GetString("machine_id")
		employeeIds := machine.GetStringSlice("employees")

		if machineItemId == "" || len(employeeIds) == 0 {
			continue
		}

		// Use static gamedata for machine info
		machineItem := gamedata.GetItem(machineItemId)
		if machineItem == nil {
			continue
		}

		maxEmp := machineItem.MaxEmployee
		if maxEmp <= 0 {
			continue // No limit defined
		}

		if len(employeeIds) > maxEmp {
			excess := len(employeeIds) - maxEmp
			app.Logger().Warn("[FIX] Machine has too many employees",
				"machineId", machine.Id,
				"machineName", machineItem.Name,
				"current", len(employeeIds),
				"max", maxEmp,
				"excess", excess)

			// Shuffle to randomize which employees get unassigned
			shuffled := make([]string, len(employeeIds))
			copy(shuffled, employeeIds)
			rand.Shuffle(len(shuffled), func(i, j int) {
				shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
			})

			// Keep only maxEmp employees
			machine.Set("employees", shuffled[:maxEmp])
			if err := app.Save(machine); err != nil {
				app.Logger().Error("[FIX] Failed to save corrected machine", "error", err)
			} else {
				fixedCount++
				app.Logger().Info("[FIX] Corrected machine employee count",
					"machineId", machine.Id,
					"removed", excess,
					"kept", maxEmp)
			}
		}
	}

	if fixedCount > 0 {
		app.Logger().Info("[FIX] EnforceMaxEmployees completed", "machinesFixed", fixedCount)
	}
}

// AutoAssignDeposits tries to assign available deposits to compatible machines
func AutoAssignDeposits(app core.App, companyId string) (int, error) {
	// 1. Fetch Company Machines
	machines, err := app.FindRecordsByFilter("machines", fmt.Sprintf("company = '%s'", companyId), "", 0, 0)
	if err != nil {
		return 0, err
	}

	// 2. Fetch Company Deposits (quantity > 0)
	// We want deposits that are not depleted.
	// Note: We sort by size (level) to assign best ones first
	deposits, err := app.FindRecordsByFilter("deposits", fmt.Sprintf("company = '%s' && quantity > 0", companyId), "-size", 0, 0)
	if err != nil {
		return 0, err
	}

	if len(deposits) == 0 {
		return 0, nil // No deposits to assign
	}

	assignedCount := 0

	// Track assigned deposits to avoid double assignment in this run
	// (though usually a deposit can support multiple machines? The schema doesn't strictly say unique,
	// but typically mining machines match 1-1 or N-1.
	// The current logic in MachineAssignment.svelte implies one deposit per machine logic (machine.expand.deposit).
	// But can a deposit be used by multiple machines?
	// The UI handles one deposit per machine.
	// Let's assume a deposit can be used by multiple machines unless specified otherwise,
	// BUT for "auto-assign" it's safer to not over-subscribe if we want to be "smart".
	// However, standard games usually allow multiple miners on one node.
	// Validating against the "Deposit" schema: there is no "assigned_machines" field.
	// So multiple machines CAN reference the same deposit.
	//
	// Strategy:
	// iterate machines -> if machine is miner and has no deposit -> find best compatible deposit -> assign.

	// Pre-load items to check if they are explorable/minable
	// To avoid N+1 queries, we could fetch all items involved, but for now loop is acceptable for typical counts.

	for _, m := range machines {
		// Skip if already has a deposit assigned
		if m.GetString("deposit") != "" {
			continue
		}

		machineItemId := m.GetString("machine_id")
		machineItem := gamedata.GetItem(machineItemId)
		if machineItem == nil {
			continue
		}

		// Check if machine output is "is_explorable" (meaning it needs a deposit)
		// The machine's product is what we check for explorable status

		productId := machineItem.Product
		if productId == "" {
			continue
		}

		// Use static gamedata for product item
		productItem := gamedata.GetItem(productId)
		if productItem == nil {
			continue
		}

		if !productItem.IsExplorable {
			continue // Not a mining machine
		}

		// Find best deposit for this resource
		var bestDeposit *core.Record
		for _, d := range deposits {
			if d.GetString("ressource") == productId {
				bestDeposit = d
				break // Deposits are sorted by size desc, so the first match is the best
			}
		}

		if bestDeposit != nil {
			m.Set("deposit", bestDeposit.Id)
			if err := app.Save(m); err != nil {
				app.Logger().Error("[AUTO-ASSIGN] Failed to save machine", "id", m.Id, "error", err)
			} else {
				assignedCount++
			}
		}
	}

	return assignedCount, nil
}
