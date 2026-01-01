package endpoints

import (
	"fmt"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// registerMachineEndpoints handles /api/machines/* routes
func registerMachineEndpoints(app *pocketbase.PocketBase, e *core.ServeEvent) {

	// POST /api/machines/auto-assign - Auto-assign available employees to machines
	e.Router.POST("/api/machines/auto-assign", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Non connecté", nil)
		}

		companyId := authRecord.GetString("active_company")
		if companyId == "" {
			return apis.NewBadRequestError("Aucune entreprise active", nil)
		}

		company, err := app.FindRecordById("companies", companyId)
		if err != nil {
			return apis.NewBadRequestError("Entreprise introuvable", nil)
		}
		if company.GetString("ceo") != authRecord.Id && !authRecord.IsSuperuser() {
			return apis.NewForbiddenError("Accès refusé", nil)
		}

		return app.RunInTransaction(func(txApp core.App) error {
			// Get all machines for the company
			machines, err := txApp.FindRecordsByFilter("machines", fmt.Sprintf("company = '%s'", companyId), "", 0, 0)
			if err != nil {
				return apis.NewBadRequestError("Erreur récupération machines", err)
			}

			// Get all employees for the company (sorted by efficiency)
			employees, err := txApp.FindRecordsByFilter("employees", fmt.Sprintf("employer = '%s'", companyId), "-efficiency", 0, 0)
			if err != nil {
				return apis.NewBadRequestError("Erreur récupération employés", err)
			}

			// Build set of already-busy employee IDs
			busySet := make(map[string]bool)
			for _, m := range machines {
				empIds := m.GetStringSlice("employees")
				for _, id := range empIds {
					busySet[id] = true
				}
			}

			// Build list of available employees
			var availableEmpIds []string
			for _, emp := range employees {
				if !busySet[emp.Id] {
					availableEmpIds = append(availableEmpIds, emp.Id)
				}
			}

			if len(availableEmpIds) == 0 {
				return c.JSON(200, map[string]interface{}{
					"success":       true,
					"message":       "Aucun employé disponible à assigner",
					"assignedCount": 0,
				})
			}

			// Sort machines: prioritize those with 0 employees
			type machineSlot struct {
				machine     *core.Record
				maxEmp      int
				currentEmp  int
				slotsNeeded int
			}
			var machinesNeedingEmp []machineSlot

			for _, m := range machines {
				machineItemId := m.GetString("machine")
				maxEmp := 1
				if machineItemId != "" {
					machineItem, err := txApp.FindRecordById("items", machineItemId)
					if err == nil {
						maxEmp = machineItem.GetInt("max_employee")
						if maxEmp <= 0 {
							maxEmp = 1
						}
					}
				}

				currentEmp := len(m.GetStringSlice("employees"))
				slotsNeeded := maxEmp - currentEmp

				if slotsNeeded > 0 {
					machinesNeedingEmp = append(machinesNeedingEmp, machineSlot{
						machine:     m,
						maxEmp:      maxEmp,
						currentEmp:  currentEmp,
						slotsNeeded: slotsNeeded,
					})
				}
			}

			// Sort: empty machines first
			for i := 0; i < len(machinesNeedingEmp)-1; i++ {
				for j := i + 1; j < len(machinesNeedingEmp); j++ {
					iEmpty := machinesNeedingEmp[i].currentEmp == 0
					jEmpty := machinesNeedingEmp[j].currentEmp == 0
					if jEmpty && !iEmpty {
						machinesNeedingEmp[i], machinesNeedingEmp[j] = machinesNeedingEmp[j], machinesNeedingEmp[i]
					}
				}
			}

			// Assign employees to machines
			assignedCount := 0
			availableIndex := 0

			for _, ms := range machinesNeedingEmp {
				if availableIndex >= len(availableEmpIds) {
					break
				}

				currentEmployees := ms.machine.GetStringSlice("employees")
				toAssignCount := ms.slotsNeeded
				if toAssignCount > len(availableEmpIds)-availableIndex {
					toAssignCount = len(availableEmpIds) - availableIndex
				}

				newEmps := availableEmpIds[availableIndex : availableIndex+toAssignCount]
				availableIndex += toAssignCount

				updatedList := append(currentEmployees, newEmps...)
				ms.machine.Set("employees", updatedList)
				if err := txApp.Save(ms.machine); err != nil {
					app.Logger().Error("[AUTO-ASSIGN] Failed to save machine", "machineId", ms.machine.Id, "error", err)
					continue
				}

				assignedCount += toAssignCount
			}

			return c.JSON(200, map[string]interface{}{
				"success":       true,
				"message":       fmt.Sprintf("%d employé(s) assigné(s) automatiquement", assignedCount),
				"assignedCount": assignedCount,
			})
		})
	})

	// GET /api/machines/stats - Get accurate machine stats (total counts across ALL machines)
	e.Router.GET("/api/machines/stats", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Non connecté", nil)
		}

		companyId := authRecord.GetString("active_company")
		if companyId == "" {
			return apis.NewBadRequestError("Aucune entreprise active", nil)
		}

		// Get ALL machines for the company (no pagination)
		machines, err := app.FindRecordsByFilter("machines", fmt.Sprintf("company = '%s'", companyId), "", 0, 0)
		if err != nil {
			return apis.NewBadRequestError("Erreur récupération machines", err)
		}

		// Get ALL employees for the company
		employees, err := app.FindRecordsByFilter("employees", fmt.Sprintf("employer = '%s'", companyId), "", 0, 0)
		if err != nil {
			return apis.NewBadRequestError("Erreur récupération employés", err)
		}

		// Calculate stats
		totalMachines := len(machines)
		totalMaxEmployees := 0
		currentAssigned := 0
		busySet := make(map[string]bool)

		for _, m := range machines {
			// Get max_employee from machine item
			machineItemId := m.GetString("machine")
			maxEmp := 1
			if machineItemId != "" {
				machineItem, err := app.FindRecordById("items", machineItemId)
				if err == nil {
					maxEmp = machineItem.GetInt("max_employee")
					if maxEmp <= 0 {
						maxEmp = 1
					}
				}
			}
			totalMaxEmployees += maxEmp

			// Count assigned employees
			empIds := m.GetStringSlice("employees")
			currentAssigned += len(empIds)
			for _, id := range empIds {
				busySet[id] = true
			}
		}

		// Count available employees (not assigned to any machine)
		availableEmployees := 0
		for _, emp := range employees {
			if !busySet[emp.Id] {
				availableEmployees++
			}
		}

		missingEmployees := totalMaxEmployees - currentAssigned
		if missingEmployees < 0 {
			missingEmployees = 0
		}

		return c.JSON(200, map[string]interface{}{
			"totalMachines":      totalMachines,
			"totalMaxEmployees":  totalMaxEmployees,
			"currentAssigned":    currentAssigned,
			"missingEmployees":   missingEmployees,
			"availableEmployees": availableEmployees,
			"totalEmployees":     len(employees),
		})
	})
	// POST /api/machines/assign-deposit - Link a machine to a deposit
	e.Router.POST("/api/machines/assign-deposit", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Non connecté", nil)
		}

		type req struct {
			MachineId string `json:"machineId"`
			DepositId string `json:"depositId"`
		}
		var data req
		if err := c.BindBody(&data); err != nil {
			return apis.NewBadRequestError("Données invalides", err)
		}

		machine, err := app.FindRecordById("machines", data.MachineId)
		if err != nil {
			return apis.NewBadRequestError("Machine introuvable", err)
		}
		if machine.GetString("company") != authRecord.GetString("active_company") {
			return apis.NewForbiddenError("Cette machine ne vous appartient pas", nil)
		}

		// If DepositId is empty, it means unassign
		if data.DepositId == "" {
			machine.Set("deposit", "")
			if err := app.Save(machine); err != nil {
				return apis.NewBadRequestError("Erreur lors de la désassignation", err)
			}
			return c.JSON(200, map[string]bool{"success": true})
		}

		deposit, err := app.FindRecordById("deposits", data.DepositId)
		if err != nil {
			return apis.NewBadRequestError("Gisement introuvable", err)
		}
		if deposit.GetString("company") != authRecord.GetString("active_company") {
			return apis.NewForbiddenError("Ce gisement ne vous appartient pas", nil)
		}

		// Verify compatibility
		machineItem, err := app.FindRecordById("items", machine.GetString("machine"))
		if err != nil {
			return apis.NewBadRequestError("Type de machine inconnu", err)
		}

		productItemId := machineItem.GetString("product")
		depositResourceId := deposit.GetString("ressource") // Note spelling 'ressource' in schema

		if productItemId != depositResourceId {
			// Get names to be friendly
			prodItem, _ := app.FindRecordById("items", productItemId)
			depItem, _ := app.FindRecordById("items", depositResourceId)
			prodName := "?"
			depName := "?"
			if prodItem != nil {
				prodName = prodItem.GetString("name")
			}
			if depItem != nil {
				depName = depItem.GetString("name")
			}

			return apis.NewBadRequestError(fmt.Sprintf("Incompatible : La machine extrait du %s mais le gisement est du %s", prodName, depName), nil)
		}

		// Assign
		machine.Set("deposit", data.DepositId)
		if err := app.Save(machine); err != nil {
			return apis.NewBadRequestError("Erreur sauvegarde machine", err)
		}

		return c.JSON(200, map[string]bool{"success": true})
	})
}
