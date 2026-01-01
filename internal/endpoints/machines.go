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
}
