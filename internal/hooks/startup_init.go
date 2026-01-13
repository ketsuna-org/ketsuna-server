package hooks

import (
	"fmt"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// InitializeCompaniesOnStartup ensures all companies have:
// 1. A wood deposit (1M quantity, level 3) if they don't have any deposits
// 2. A CEO employee (PDG) if they don't have one
func InitializeCompaniesOnStartup(app *pocketbase.PocketBase) {
	companies, err := app.FindRecordsByFilter("companies", "", "", 0, 0)
	if err != nil {
		app.Logger().Error("[STARTUP] Failed to fetch companies", "err", err)
		return
	}

	// Use hardcoded "wood" resource ID (from gamedata)
	woodItemId := "wood"

	for _, company := range companies {
		companyId := company.Id
		companyName := company.GetString("name")

		// 1. Check and create wood deposit if needed
		deposits, _ := app.FindRecordsByFilter("deposits", fmt.Sprintf("company = '%s'", companyId), "", 1, 0)
		if len(deposits) == 0 {
			createWoodDeposit(app, companyId, woodItemId)
			app.Logger().Info("[STARTUP] Created wood deposit", "company", companyName)
		}

		// 2. Check and create CEO if needed
		ceoEmployees, _ := app.FindRecordsByFilter("employees", fmt.Sprintf("employer = '%s' && poste = 'PDG'", companyId), "", 1, 0)
		if len(ceoEmployees) == 0 {
			createCEOEmployee(app, companyId, companyName)
			app.Logger().Info("[STARTUP] Created CEO employee", "company", companyName)
		}

		// 3. Check and create starter forestry machine if needed
		EnsureForestryMachineOnCompanyCreation(app, companyId)
	}

	app.Logger().Info("[STARTUP] Company initialization complete", "companies", len(companies))
}

// createWoodDeposit creates a wood deposit with 1M quantity and level 3
func createWoodDeposit(app *pocketbase.PocketBase, companyId string, woodItemId string) {
	depositsCollection, err := app.FindCollectionByNameOrId("deposits")
	if err != nil {
		app.Logger().Error("[STARTUP] Failed to find deposits collection", "err", err)
		return
	}

	deposit := core.NewRecord(depositsCollection)
	deposit.Set("company", companyId)
	deposit.Set("ressource_id", woodItemId) // Use ressource_id (string), not ressource
	deposit.Set("quantity", 1000000)        // 1 million
	deposit.Set("size", 3)                  // Level 3

	if err := app.Save(deposit); err != nil {
		app.Logger().Error("[STARTUP] Failed to create wood deposit", "err", err)
	}
}

// createCEOEmployee creates a legendary CEO employee with balanced stats
func createCEOEmployee(app *pocketbase.PocketBase, companyId string, companyName string) {
	employeesCollection, err := app.FindCollectionByNameOrId("employees")
	if err != nil {
		return
	}

	employee := core.NewRecord(employeesCollection)
	employee.Set("employer", companyId)
	employee.Set("name", fmt.Sprintf("PDG de %s", companyName))
	employee.Set("poste", "PDG")
	employee.Set("salary", 0)           // CEO doesn't need salary
	employee.Set("mining", 3)           // Can mine
	employee.Set("exploration_luck", 5) // Good at exploration
	employee.Set("maintenance", 3)      // Can maintain

	if err := app.Save(employee); err != nil {
		app.Logger().Error("[STARTUP] Failed to create CEO employee", "err", err)
	}
}

// EnsureCEOOnCompanyCreation is called when a company is created to auto-add a CEO
func EnsureCEOOnCompanyCreation(app *pocketbase.PocketBase, companyId string, companyName string) {
	createCEOEmployee(app, companyId, companyName)
	app.Logger().Info("[COMPANY] Auto-created CEO for new company", "company", companyName)
}

// EnsureWoodDepositOnCompanyCreation is called when a company is created to auto-add a wood deposit
func EnsureWoodDepositOnCompanyCreation(app *pocketbase.PocketBase, companyId string) {
	// Use hardcoded "wood" resource ID (from gamedata)
	createWoodDeposit(app, companyId, "wood")
	app.Logger().Info("[COMPANY] Auto-created Wood Deposit for new company", "company", companyId)
}

// EnsureForestryMachineOnCompanyCreation is called when a company is created to auto-add a starter forestry machine
func EnsureForestryMachineOnCompanyCreation(app *pocketbase.PocketBase, companyId string) {
	// Avoid duplicates if hooks rerun
	existing, err := app.FindRecordsByFilter("machines", fmt.Sprintf("company = '%s' && machine_id = 'forestry_machine'", companyId), "", 1, 0)
	if err != nil {
		app.Logger().Error("[COMPANY] Failed to check forestry machine", "company", companyId, "err", err)
		return
	}
	if len(existing) > 0 {
		return
	}

	collection, err := app.FindCollectionByNameOrId("machines")
	if err != nil {
		app.Logger().Error("[COMPANY] Failed to find machines collection", "err", err)
		return
	}

	machine := core.NewRecord(collection)
	machine.Set("company", companyId)
	machine.Set("machine_id", "forestry_machine")
	machine.Set("placed", false)

	if err := app.Save(machine); err != nil {
		app.Logger().Error("[COMPANY] Failed to create forestry machine", "company", companyId, "err", err)
		return
	}

	app.Logger().Info("[COMPANY] Auto-created Forestry Machine for new company", "company", companyId)
}

// MigrateTechStatusOnStartup fixes existing company_techs records that don't have a status
// Sets them to "completed" since they were unlocked before the timed system
func MigrateTechStatusOnStartup(app *pocketbase.PocketBase) {
	// Find all company_techs records without a status
	records, err := app.FindRecordsByFilter("company_techs", "status = '' || status = null", "", 0, 0)
	if err != nil {
		app.Logger().Error("[STARTUP] Failed to fetch company_techs for migration", "err", err)
		return
	}

	if len(records) == 0 {
		app.Logger().Info("[STARTUP] No company_techs records to migrate")
		return
	}

	migratedCount := 0
	for _, record := range records {
		record.Set("status", "completed")
		if err := app.Save(record); err != nil {
			app.Logger().Error("[STARTUP] Failed to migrate company_tech", "id", record.Id, "err", err)
		} else {
			migratedCount++
		}
	}

	app.Logger().Info("[STARTUP] Migrated company_techs to completed status", "count", migratedCount)
}

// CleanupDuplicateDepositEdges removes excess edges where a machine has multiple deposit inputs
// It keeps the most recently created edge and deletes the others
func CleanupDuplicateDepositEdges(app *pocketbase.PocketBase) {
	// Find all edges that connect a deposit to a machine
	edges, err := app.FindRecordsByFilter("edge_relation", "input_type='deposit' && output_type='machine'", "-created", 0, 0)
	if err != nil {
		app.Logger().Error("[CLEANUP] Failed to fetch edges", "err", err)
		return
	}

	machineEdges := make(map[string][]*core.Record)
	for _, edge := range edges {
		machineId := edge.GetString("output_id")
		machineEdges[machineId] = append(machineEdges[machineId], edge)
	}

	deletedCount := 0
	for machineId, machineEdgeList := range machineEdges {
		if len(machineEdgeList) > 1 {
			// Sort by created desc (already done by query but just ensuring logic)
			// machineEdgeList[0] is the newest, keep it. Delete the rest.
			keep := machineEdgeList[0]
			toDelete := machineEdgeList[1:]

			app.Logger().Warn("[CLEANUP] Machine has multiple deposits", "machineId", machineId, "count", len(machineEdgeList), "keeping", keep.Id)

			for _, edge := range toDelete {
				if err := app.Delete(edge); err != nil {
					app.Logger().Error("[CLEANUP] Failed to delete duplicate edge", "edgeId", edge.Id, "err", err)
				} else {
					deletedCount++
				}
			}
		}
	}

	if deletedCount > 0 {
		app.Logger().Info("[CLEANUP] Removed duplicate deposit edges", "count", deletedCount)
	}
}
