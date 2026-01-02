package hooks

import (
	"fmt"
	"math/rand"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func registerMachineHooks(app *pocketbase.PocketBase, inv *InventoryLogic) {
	app.OnRecordCreateRequest("machines").BindFunc(func(e *core.RecordRequestEvent) error {
		record := e.Record
		companyId := record.GetString("company")
		machineItemId := record.GetString("machine")
		employeeIds := record.GetStringSlice("employees")

		if companyId == "" || machineItemId == "" {
			return apis.NewBadRequestError("ID de compagnie ou de machine manquant.", nil)
		}

		// 1. Validate employees not assigned elsewhere
		if len(employeeIds) > 0 {
			for _, empId := range employeeIds {
				found, err := app.FindRecordsByFilter("machines", fmt.Sprintf("employees ~ '%s'", empId), "", 1, 0)
				if err == nil && len(found) > 0 {
					return apis.NewBadRequestError("Un ou plusieurs employés sont déjà assignés à une autre machine.", nil)
				}
			}
		}

		// 2. Check max_employee limit
		machineItem, err := app.FindRecordById("items", machineItemId)
		if err == nil {
			maxEmp := machineItem.GetInt("max_employee")
			if maxEmp > 0 && len(employeeIds) > maxEmp {
				return apis.NewBadRequestError(fmt.Sprintf("Cette machine ne peut accueillir que %d employé(s) maximum.", maxEmp), nil)
			}
		}

		// 3. Check Inventory Stock
		if !inv.HasEnoughItems(app, companyId, machineItemId, 1) {
			return apis.NewBadRequestError("Vous n'avez pas cette machine en stock dans votre inventaire.", nil)
		}

		// 4. Deduct from Inventory
		if err := inv.UpdateInventory(app, companyId, machineItemId, -1); err != nil {
			return apis.NewBadRequestError(fmt.Sprintf("Erreur lors de la mise à jour de l'inventaire: %v", err), nil)
		}

		app.Logger().Info("[MACHINES] Machine assigned", "machineId", machineItemId, "companyId", companyId)
		return e.Next()
	})

	app.OnRecordUpdateRequest("machines").BindFunc(func(e *core.RecordRequestEvent) error {
		record := e.Record
		machineItemId := record.GetString("machine")
		employeeIds := record.GetStringSlice("employees")

		// Check max_employee limit
		if machineItemId != "" {
			machineItem, err := app.FindRecordById("items", machineItemId)
			if err == nil {
				maxEmp := machineItem.GetInt("max_employee")
				if maxEmp > 0 && len(employeeIds) > maxEmp {
					return apis.NewBadRequestError(fmt.Sprintf("Cette machine ne peut accueillir que %d employé(s) maximum. Vous essayez d'en assigner %d.", maxEmp, len(employeeIds)), nil)
				}
			}
		}

		if len(employeeIds) > 0 {
			e.App.Logger().Info("[MACHINES] Validating update", "machineId", record.Id, "newEmployeeList", employeeIds)
			for _, empId := range employeeIds {
				found, err := app.FindRecordsByFilter("machines", fmt.Sprintf("employees ~ '%s'", empId), "", 10, 0)
				if err == nil {
					e.App.Logger().Info("[MACHINES] Checked employee", "empId", empId, "foundInMachines", len(found))
					for _, m := range found {
						e.App.Logger().Info("[MACHINES] comparing", "foundId", m.Id, "currentId", record.Id)
						if m.Id != record.Id {
							app.Logger().Error("[MACHINES] Validation failed: Employee already busy", "empId", empId, "otherMachine", m.Id)
							return apis.NewBadRequestError("Un ou plusieurs employés sont déjà assignés à une autre machine.", nil)
						}
					}
				} else {
					e.App.Logger().Error("[MACHINES] Error checking filter", "error", err)
				}
			}
		} else {
			e.App.Logger().Info("[MACHINES] Clearing all employees", "machineId", record.Id)
		}
		return e.Next()
	})

	app.OnRecordDeleteRequest("machines").BindFunc(func(e *core.RecordRequestEvent) error {
		record := e.Record
		companyId := record.GetString("company")
		machineItemId := record.GetString("machine")

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
		machineItemId := machine.GetString("machine")
		employeeIds := machine.GetStringSlice("employees")

		if machineItemId == "" || len(employeeIds) == 0 {
			continue
		}

		machineItem, err := app.FindRecordById("items", machineItemId)
		if err != nil {
			continue
		}

		maxEmp := machineItem.GetInt("max_employee")
		if maxEmp <= 0 {
			continue // No limit defined
		}

		if len(employeeIds) > maxEmp {
			excess := len(employeeIds) - maxEmp
			app.Logger().Warn("[FIX] Machine has too many employees",
				"machineId", machine.Id,
				"machineName", machineItem.GetString("name"),
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
	// Note: We might want to filter by richness to assign best ones first, but let's just get all > 0
	deposits, err := app.FindRecordsByFilter("deposits", fmt.Sprintf("company = '%s' && quantity > 0", companyId), "-richness", 0, 0)
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

		machineItemId := m.GetString("machine")
		machineItem, err := app.FindRecordById("items", machineItemId)
		if err != nil {
			continue
		}
		
		// Check if machine output is "is_explorable" (meaning it needs a deposit)
		// The machine item itself doesn't have "is_explorable", its PRODUCT does.
		// Wait, the schema says:
		// items -> product (relation)
		// items -> is_explorable (boolean). 
		// Usually a "Drill" produces "Iron Ore". "Iron Ore" is explorable.
		
		productId := machineItem.GetString("product")
		if productId == "" {
			continue
		}
		
		productItem, err := app.FindRecordById("items", productId)
		if err != nil {
			continue
		}
		
		if !productItem.GetBool("is_explorable") {
			continue // Not a mining machine
		}

		// Find best deposit for this resource
		var bestDeposit *core.Record
		for _, d := range deposits {
			if d.GetString("ressource") == productId {
				bestDeposit = d
				break // Deposits are sorted by richness desc, so the first match is the best
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
