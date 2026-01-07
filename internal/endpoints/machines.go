package endpoints

import (
	"fmt"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"ketsuna.com/server/internal/gamedata"
)

// registerMachineEndpoints handles /api/machines/* routes
func registerMachineEndpoints(app *pocketbase.PocketBase, e *core.ServeEvent) {

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

		// Optimize: Batch fetch all referenced items to avoid N+1 queries
		machineItemIds := make([]string, 0, len(machines))
		seenItems := make(map[string]bool)
		for _, m := range machines {
			itemId := m.GetString("machine")
			if itemId != "" && !seenItems[itemId] {
				machineItemIds = append(machineItemIds, itemId)
				seenItems[itemId] = true
			}
		}

		// Fetch all relevant items in one query (or chunks if needed, but usually fine for this game scale)
		// Since PocketBase Go API doesn't have a simple FindRecordsByIds for arbitrary list, we use a filter
		// Or simpler: fetch ALL items since they are static definitions essentially
		// But let's verify if we can just fetch all items. Assuming items collection is < 1000 records.
		// A safe approach: If we have IDs, we can build a filter "id = 'id1' || id = 'id2'..." but that's ugly.
		// Actually, fetching all items is probably faster/safer than N+1.
		allItems, err := app.FindRecordsByFilter("items", "", "", 0, 0)
		if err != nil {
			return apis.NewBadRequestError("Erreur récupération items", err)
		}

		itemMap := make(map[string]*core.Record)
		for _, item := range allItems {
			itemMap[item.Id] = item
		}

		// Build map of employees per machine
		employeesPerMachine := make(map[string]int)
		busySet := make(map[string]bool)
		for _, emp := range employees {
			mId := emp.GetString("machine")
			if mId != "" {
				employeesPerMachine[mId]++
				busySet[emp.Id] = true
			}
		}

		// Calculate stats
		totalMachines := len(machines)
		totalMaxEmployees := 0
		currentAssigned := 0
		// busySet already populated above
		machineTypeCount := 0
		stockageTypeCount := 0

		for _, m := range machines {
			// Get max_employee from machine item (via map)
			machineItemId := m.GetString("machine_id") // changed from "machine" to "machine_id" to match schema
			if machineItemId == "" {
				machineItemId = m.GetString("machine") // fallback if schema varies
			}

			maxEmp := 1
			var machineItem *core.Record

			if item, ok := itemMap[machineItemId]; ok {
				machineItem = item
				maxEmp = machineItem.GetInt("max_employee")
				// If 0, check if it has unlimited or specific logic?
				// Usually max_employee 0 means 0 allowed? Or unlimited?
				// Assuming 0 -> 1 for safety unless specified
				if maxEmp <= 0 {
					maxEmp = 1
				}
			}
			totalMaxEmployees += maxEmp

			// Count assigned employees using map
			assigned := employeesPerMachine[m.Id]
			currentAssigned += assigned

			// Count types
			if machineItem != nil {
				itemType := machineItem.GetString("type")
				switch itemType {
				case "Machine":
					machineTypeCount++
				case "Stockage":
					stockageTypeCount++
				}
			}
		}

		// Count available employees (not assigned to any machine)
		availableEmployees := 0
		for _, emp := range employees {
			// Check if busy (machine, deposit, exploration)
			// busySet tracks machine assignment.
			// Need to check deposit/exploration too?
			// Original code just checked busySet which was populated from machine.employees
			// But now busySet comes from employee.machine check.
			// The original logic only counted "busy" if in machine.employees list.
			// So existing logic is preserved if we only use busySet from machine assignment.
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
			"machineTypeCount":   machineTypeCount,
			"stockageTypeCount":  stockageTypeCount,
		})
	})

	// POST /api/machines/assign-employee - Link an employee to a machine
	e.Router.POST("/api/machines/assign-employee", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Non connecté", nil)
		}

		type req struct {
			MachineId  string `json:"machineId"`
			EmployeeId string `json:"employeeId"`
		}
		var data req
		if err := c.BindBody(&data); err != nil {
			return apis.NewBadRequestError("Données invalides", err)
		}

		companyId := authRecord.GetString("active_company")

		// If EmployeeId is empty, we interpret as "unassign" but we need employee ID to unassign strictly?
		// Actually typical flow: Assign X to Y.
		// Unassign is usually "update employee X set machine=''".
		// Let's support "Assign X to Machine Y".
		if data.EmployeeId == "" {
			return apis.NewBadRequestError("EmployeeId requis", nil)
		}

		// Verify Employee
		employee, err := app.FindRecordById("employees", data.EmployeeId)
		if err != nil {
			return apis.NewBadRequestError("Employé introuvable", err)
		}
		if employee.GetString("employer") != companyId {
			return apis.NewForbiddenError("Cet employé ne travaille pas pour vous", nil)
		}

		// Verify Machine
		machine, err := app.FindRecordById("machines", data.MachineId)
		if err != nil {
			return apis.NewBadRequestError("Machine introuvable", err)
		}
		if machine.GetString("company") != companyId {
			return apis.NewForbiddenError("Cette machine ne vous appartient pas", nil)
		}

		// CHECK CAPACITY
		// Count current employees for this machine
		currentEmployees, _ := app.FindRecordsByFilter("employees", fmt.Sprintf("machine = '%s'", data.MachineId), "", 0, 0)

		machineItemId := machine.GetString("machine_id")
		if machineItemId == "" {
			machineItemId = machine.GetString("machine")
		}

		machineItem := gamedata.GetItem(machineItemId)
		if machineItem != nil {
			maxEmp := machineItem.MaxEmployee
			if len(currentEmployees) >= maxEmp {
				return apis.NewBadRequestError(fmt.Sprintf("Machine pleine (%d/%d)", len(currentEmployees), maxEmp), nil)
			}
		}

		// Assign
		employee.Set("machine", data.MachineId)
		// Clear deposit assignment if any (assuming exclusive?)
		// Usually employees are EITHER on machine OR deposit OR exploration.
		employee.Set("deposit", "")
		employee.Set("exploration", "")

		if err := app.Save(employee); err != nil {
			return apis.NewBadRequestError("Erreur lors de l'assignation", err)
		}

		return c.JSON(200, map[string]interface{}{
			"success": true,
			"message": "Employé assigné",
		})
	})

	// POST /api/machines/unassign-employee
	e.Router.POST("/api/machines/unassign-employee", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Non connecté", nil)
		}

		type req struct {
			EmployeeId string `json:"employeeId"`
		}
		var data req
		if err := c.BindBody(&data); err != nil {
			return apis.NewBadRequestError("Données invalides", err)
		}

		employee, err := app.FindRecordById("employees", data.EmployeeId)
		if err != nil {
			return apis.NewBadRequestError("Employé introuvable", err)
		}
		if employee.GetString("employer") != authRecord.GetString("active_company") {
			return apis.NewForbiddenError("Accès refusé", nil)
		}

		employee.Set("machine", "")
		if err := app.Save(employee); err != nil {
			return apis.NewBadRequestError("Erreur lors de la désassignation", err)
		}

		return c.JSON(200, map[string]bool{"success": true})
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

		// Check capacity (Machine = 5 slots)
		size := deposit.GetInt("size")
		if size <= 0 {
			size = 1
		}
		maxCapacity := size * 5

		// Count existing machines (excluding self if already assigned, though here usually pure assignment)
		existingMachines, _ := app.FindRecordsByFilter("machines", fmt.Sprintf("deposit = '%s' && id != '%s'", data.DepositId, data.MachineId), "", 0, 0)
		// Count employees
		employeesOnDeposit, _ := app.FindRecordsByFilter("employees", fmt.Sprintf("deposit = '%s'", data.DepositId), "", 0, 0)

		currentOccupancy := (len(existingMachines) * 5) + len(employeesOnDeposit)
		newOccupancy := currentOccupancy + 5

		if newOccupancy > maxCapacity {
			return apis.NewBadRequestError(fmt.Sprintf("Capacité du gisement insuffisante. Une machine prend 5 places. (%d/%d disponibles)", maxCapacity-currentOccupancy, maxCapacity), nil)
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

	// POST /api/machines/remove - Remove a machine and return it to inventory
	e.Router.POST("/api/machines/remove", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Non connecté", nil)
		}

		type req struct {
			MachineId string `json:"machineId"`
		}
		var data req
		if err := c.BindBody(&data); err != nil {
			return apis.NewBadRequestError("Données invalides", err)
		}

		if data.MachineId == "" {
			return apis.NewBadRequestError("machineId requis", nil)
		}

		machine, err := app.FindRecordById("machines", data.MachineId)
		if err != nil {
			return apis.NewBadRequestError("Machine introuvable", err)
		}

		companyId := authRecord.GetString("active_company")
		if machine.GetString("company") != companyId {
			return apis.NewForbiddenError("Cette machine ne vous appartient pas", nil)
		}

		return app.RunInTransaction(func(txApp core.App) error {
			machineItemId := machine.GetString("machine")

			// 1. Delete the machine record
			if err := txApp.Delete(machine); err != nil {
				return apis.NewBadRequestError("Erreur lors de la suppression", err)
			}

			// 2. Restore the item to inventory
			if machineItemId != "" {
				// Check if inventory entry exists
				inventory, err := txApp.FindFirstRecordByFilter("inventory",
					fmt.Sprintf("company = '%s' && item = '%s'", companyId, machineItemId))

				if err != nil {
					// Create new inventory entry
					invCollection, _ := txApp.FindCollectionByNameOrId("inventory")
					newInv := core.NewRecord(invCollection)
					newInv.Set("company", companyId)
					newInv.Set("item", machineItemId)
					newInv.Set("quantity", 1)
					if err := txApp.Save(newInv); err != nil {
						return apis.NewBadRequestError("Erreur création inventaire", err)
					}
				} else {
					// Increment existing inventory
					currentQty := inventory.GetInt("quantity")
					inventory.Set("quantity", currentQty+1)
					if err := txApp.Save(inventory); err != nil {
						return apis.NewBadRequestError("Erreur mise à jour inventaire", err)
					}
				}
			}

			return c.JSON(200, map[string]interface{}{
				"success": true,
				"message": "Machine retirée et renvoyée au stock",
			})
		})
	})
}
