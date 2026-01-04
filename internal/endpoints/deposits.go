package endpoints

import (
	"net/http"

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
		maxEmployees := size * 5

		// Count current employees on this deposit
		currentEmployees, _ := app.FindRecordsByFilter(
			"employees",
			"deposit = '"+body.DepositId+"'",
			"", 0, 0,
		)
		if len(currentEmployees) >= maxEmployees {
			return apis.NewBadRequestError("Ce gisement a atteint sa capacité maximale d'employés", nil)
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
}
