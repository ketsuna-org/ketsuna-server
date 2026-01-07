package endpoints

import (
	"net/http"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func registerDepositEndpoints(app *pocketbase.PocketBase, e *core.ServeEvent) {
	// POST /api/deposits/assign-employee - Assign an employee to a deposit
	e.Router.POST("/api/deposits/assign-employee", func(re *core.RequestEvent) error {
		var body struct {
			DepositId  string `json:"depositId"`
			EmployeeId string `json:"employeeId"`
		}
		if err := re.BindBody(&body); err != nil {
			return apis.NewBadRequestError("Invalid request body", err)
		}

		authRecord := re.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Vous devez être connecté", nil)
		}

		// Verify deposit exists and belongs to user's company
		deposit, err := app.FindRecordById("deposits", body.DepositId)
		if err != nil {
			return apis.NewNotFoundError("Gisement non trouvé", nil)
		}

		companyId := deposit.GetString("company")
		company, err := app.FindRecordById("companies", companyId)
		if err != nil {
			return apis.NewNotFoundError("Entreprise non trouvée", nil)
		}

		if company.GetString("ceo") != authRecord.Id {
			return apis.NewForbiddenError("Vous n'êtes pas le CEO de cette entreprise", nil)
		}

		// Check deposit capacity (size * 5)
		size := deposit.GetInt("size")
		if size <= 0 {
			size = 1
		}
		maxCapacity := size * 5

		// Count current employees on this deposit
		currentEmployees, _ := app.FindRecordsByFilter(
			"employees",
			"deposit = '"+body.DepositId+"'",
			"", 0, 0,
		)

		// Count machines on this deposit (Each machine counts as 5 employees)
		currentMachines, _ := app.FindRecordsByFilter("machines", "deposit = '"+body.DepositId+"'", "", 0, 0)
		machineOccupancy := len(currentMachines) * 5

		if len(currentEmployees)+machineOccupancy >= maxCapacity {
			return apis.NewBadRequestError("Ce gisement a atteint sa capacité maximale (Machines + Employés)", nil)
		}

		// Verify employee exists and belongs to company
		employee, err := app.FindRecordById("employees", body.EmployeeId)
		if err != nil {
			return apis.NewNotFoundError("Employé non trouvé", nil)
		}

		if employee.GetString("employer") != companyId {
			return apis.NewForbiddenError("Cet employé n'appartient pas à votre entreprise", nil)
		}

		// Check if employee is already assigned
		if employee.GetString("deposit") != "" {
			return apis.NewBadRequestError("Cet employé est déjà assigné à un gisement", nil)
		}
		if employee.GetString("exploration") != "" {
			return apis.NewBadRequestError("Cet employé est en exploration", nil)
		}

		// Check if employee is assigned to a machine
		machineAssignments, _ := app.FindRecordsByFilter(
			"machines",
			"employees ~ '"+body.EmployeeId+"'",
			"", 1, 0,
		)
		if len(machineAssignments) > 0 {
			return apis.NewBadRequestError("Cet employé est assigné à une machine", nil)
		}

		// Assign employee to deposit
		employee.Set("deposit", body.DepositId)
		if err := app.Save(employee); err != nil {
			return apis.NewBadRequestError("Erreur lors de l'assignation", err)
		}

		// Initialize last_harvest_at if not set (starts the mining timer)
		if deposit.GetDateTime("last_harvest_at").Time().IsZero() {
			deposit.Set("last_harvest_at", time.Now())
			app.Save(deposit)
		}

		return re.JSON(http.StatusOK, map[string]any{
			"success": true,
			"message": "Employé assigné au gisement",
		})
	}).Bind(apis.RequireAuth())

	// POST /api/deposits/unassign-employee - Unassign an employee from a deposit
	e.Router.POST("/api/deposits/unassign-employee", func(re *core.RequestEvent) error {
		var body struct {
			EmployeeId string `json:"employeeId"`
		}
		if err := re.BindBody(&body); err != nil {
			return apis.NewBadRequestError("Invalid request body", err)
		}

		authRecord := re.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Vous devez être connecté", nil)
		}

		// Verify employee exists
		employee, err := app.FindRecordById("employees", body.EmployeeId)
		if err != nil {
			return apis.NewNotFoundError("Employé non trouvé", nil)
		}

		// Verify ownership
		companyId := employee.GetString("employer")
		company, err := app.FindRecordById("companies", companyId)
		if err != nil {
			return apis.NewNotFoundError("Entreprise non trouvée", nil)
		}

		if company.GetString("ceo") != authRecord.Id {
			return apis.NewForbiddenError("Vous n'êtes pas le CEO de cette entreprise", nil)
		}

		// Unassign
		employee.Set("deposit", "")
		if err := app.Save(employee); err != nil {
			return apis.NewBadRequestError("Erreur lors de la désassignation", err)
		}

		return re.JSON(http.StatusOK, map[string]any{
			"success": true,
			"message": "Employé retiré du gisement",
		})
	}).Bind(apis.RequireAuth())

	// POST /api/deposits/harvest - Transfer harvested resources to company inventory
	e.Router.POST("/api/deposits/harvest", func(re *core.RequestEvent) error {
		var body struct {
			DepositId string `json:"depositId"`
		}
		if err := re.BindBody(&body); err != nil {
			return apis.NewBadRequestError("Invalid request body", err)
		}

		authRecord := re.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Vous devez être connecté", nil)
		}

		// Verify deposit exists and belongs to user's company
		deposit, err := app.FindRecordById("deposits", body.DepositId)
		if err != nil {
			return apis.NewNotFoundError("Gisement non trouvé", nil)
		}

		companyId := deposit.GetString("company")
		company, err := app.FindRecordById("companies", companyId)
		if err != nil {
			return apis.NewNotFoundError("Entreprise non trouvée", nil)
		}

		if company.GetString("ceo") != authRecord.Id {
			return apis.NewForbiddenError("Vous n'êtes pas le CEO de cette entreprise", nil)
		}

		// Get harvested amount
		harvested := deposit.GetFloat("harvested")
		if harvested <= 0 {
			return apis.NewBadRequestError("Aucune ressource à récolter", nil)
		}

		// Check for "ressource_id" (new schema) then fallback to "ressource" (old schema)
		resourceId := deposit.GetString("ressource_id")
		if resourceId == "" {
			resourceId = deposit.GetString("ressource")
		}

		if resourceId == "" {
			return apis.NewBadRequestError("Type de ressource invalide", nil)
		}

		// Find or create inventory record
		// Note: Inventory schema likely uses "item_id" based on graph_traversal.go usage
		inventory, err := app.FindFirstRecordByFilter(
			"inventory",
			"company = '"+companyId+"' && item_id = '"+resourceId+"'",
		)
		if err != nil {
			// Create new inventory record
			inventoryCollection, _ := app.FindCollectionByNameOrId("inventory")
			inventory = core.NewRecord(inventoryCollection)
			inventory.Set("company", companyId)
			inventory.Set("item_id", resourceId)
			inventory.Set("quantity", harvested)
		} else {
			// Update existing
			currentQty := inventory.GetFloat("quantity")
			inventory.Set("quantity", currentQty+harvested)
		}

		if err := app.Save(inventory); err != nil {
			return apis.NewBadRequestError("Erreur lors du transfert", err)
		}

		// Reset harvested to 0
		deposit.Set("harvested", 0)
		if err := app.Save(deposit); err != nil {
			return apis.NewBadRequestError("Erreur lors de la réinitialisation", err)
		}

		return re.JSON(http.StatusOK, map[string]any{
			"success":   true,
			"message":   "Ressources récoltées",
			"harvested": harvested,
		})
	}).Bind(apis.RequireAuth())
}
