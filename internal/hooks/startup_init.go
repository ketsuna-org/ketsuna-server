package hooks

import (
	"fmt"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// InitializeCompaniesOnStartup ensures all companies have:
// 1. A wood deposit (1M quantity, level 3) if they don't have any deposits
// 2. A CEO employee (PDG, legendary, all stats = 1) if they don't have one
func InitializeCompaniesOnStartup(app *pocketbase.PocketBase) {
	companies, err := app.FindRecordsByFilter("companies", "", "", 0, 0)
	if err != nil {
		app.Logger().Error("[STARTUP] Failed to fetch companies", "err", err)
		return
	}

	// Find the "Bois" item for wood deposits
	woodItems, err := app.FindRecordsByFilter("items", "name ~ 'Bois'", "", 1, 0)
	var woodItemId string
	if err == nil && len(woodItems) > 0 {
		woodItemId = woodItems[0].Id
	} else {
		app.Logger().Warn("[STARTUP] Could not find 'Bois' item, skipping deposit creation")
	}

	for _, company := range companies {
		companyId := company.Id
		companyName := company.GetString("name")

		// 1. Check and create wood deposit if needed
		if woodItemId != "" {
			deposits, _ := app.FindRecordsByFilter("deposits", fmt.Sprintf("company = '%s'", companyId), "", 1, 0)
			if len(deposits) == 0 {
				createWoodDeposit(app, companyId, woodItemId)
				app.Logger().Info("[STARTUP] Created wood deposit", "company", companyName)
			}
		}

		// 2. Check and create CEO if needed
		ceoEmployees, _ := app.FindRecordsByFilter("employees", fmt.Sprintf("employer = '%s' && poste = 'PDG'", companyId), "", 1, 0)
		if len(ceoEmployees) == 0 {
			createCEOEmployee(app, companyId, companyName)
			app.Logger().Info("[STARTUP] Created CEO employee", "company", companyName)
		}
	}

	app.Logger().Info("[STARTUP] Company initialization complete", "companies", len(companies))
}

// createWoodDeposit creates a wood deposit with 1M quantity and level 3
func createWoodDeposit(app *pocketbase.PocketBase, companyId string, woodItemId string) {
	depositsCollection, err := app.FindCollectionByNameOrId("deposits")
	if err != nil {
		return
	}

	deposit := core.NewRecord(depositsCollection)
	deposit.Set("company", companyId)
	deposit.Set("ressource", woodItemId)
	deposit.Set("quantity", 1000000) // 1 million
	deposit.Set("size", 3)           // Level 3

	if err := app.Save(deposit); err != nil {
		app.Logger().Error("[STARTUP] Failed to create wood deposit", "err", err)
	}
}

// createCEOEmployee creates a legendary CEO employee with all stats = 1
func createCEOEmployee(app *pocketbase.PocketBase, companyId string, companyName string) {
	employeesCollection, err := app.FindCollectionByNameOrId("employees")
	if err != nil {
		return
	}

	employee := core.NewRecord(employeesCollection)
	employee.Set("employer", companyId)
	employee.Set("name", fmt.Sprintf("PDG de %s", companyName))
	employee.Set("poste", "PDG")
	employee.Set("rarity", 5)       // Legendary
	employee.Set("salary", 0)       // CEO doesn't need salary
	employee.Set("efficiency", 100) // Max efficiency
	employee.Set("mining", 1)       // Stats start at 1
	employee.Set("exploration_luck", 1)
	employee.Set("energy", 100)
	employee.Set("maintenance", 1)

	if err := app.Save(employee); err != nil {
		app.Logger().Error("[STARTUP] Failed to create CEO employee", "err", err)
	}
}

// EnsureCEOOnCompanyCreation is called when a company is created to auto-add a CEO
func EnsureCEOOnCompanyCreation(app *pocketbase.PocketBase, companyId string, companyName string) {
	createCEOEmployee(app, companyId, companyName)
	app.Logger().Info("[COMPANY] Auto-created CEO for new company", "company", companyName)
}
