package endpoints

import (
	"fmt"

	"ketsuna.com/server/internal/hooks"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// registerEmployeesEndpoints handles /api/employees/* routes
func registerEmployeesEndpoints(app *pocketbase.PocketBase, e *core.ServeEvent, emp *hooks.EmployeeLogic) {

	// POST /api/employees/hire - Hire employees (with bulk support)
	e.Router.POST("/api/employees/hire", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Vous devez être connecté.", nil)
		}

		data := struct {
			CompanyId string `json:"companyId" form:"companyId"`
			Quantity  int    `json:"quantity" form:"quantity"`
		}{}
		if err := c.BindBody(&data); err != nil {
			return apis.NewBadRequestError("Corps JSON invalide", err)
		}

		companyId := data.CompanyId
		if companyId == "" {
			return apis.NewBadRequestError("companyId requis", nil)
		}

		quantity := data.Quantity
		if quantity <= 0 {
			quantity = 1
		}
		if quantity > 50 {
			quantity = 50 // Max 50 per batch
		}

		return app.RunInTransaction(func(txApp core.App) error {
			company, err := txApp.FindRecordById("companies", companyId)
			if err != nil {
				return apis.NewBadRequestError("Company introuvable", nil)
			}

			if !authRecord.IsSuperuser() && company.GetString("ceo") != authRecord.Id {
				return apis.NewForbiddenError("Vous n'êtes pas le PDG de cette entreprise", nil)
			}

			hiredRecords := []*core.Record{}
			totalCost := 0
			errors := []string{}

			for i := 0; i < quantity; i++ {
				hired, err := emp.HireEmployee(txApp, companyId)
				if err != nil {
					errors = append(errors, err.Error())
					return apis.NewBadRequestError(err.Error(), nil)
				}
				hiredRecords = append(hiredRecords, hired.Record)
				totalCost += hired.Cost
			}

			if len(hiredRecords) == 0 {
				return apis.NewBadRequestError("Aucun employé recruté", nil)
			}

			return c.JSON(200, map[string]interface{}{
				"success":    true,
				"message":    fmt.Sprintf("%d employé(s) recruté(s) avec succès", len(hiredRecords)),
				"records":    hiredRecords,
				"totalCost":  totalCost,
				"hiredCount": len(hiredRecords),
				"errors":     errors,
			})
		})
	})

	// GET /api/employees/preview-cost - Get average hiring cost estimate
	e.Router.GET("/api/employees/preview-cost", func(c *core.RequestEvent) error {
		// Return average hiring costs based on employee_logic.go constants
		avgHiringFee := 221
		avgReserve := 1326

		return c.JSON(200, map[string]interface{}{
			"averageHiringFee":     avgHiringFee,
			"averageReserveNeeded": avgReserve,
			"averageTotalRequired": avgHiringFee + avgReserve,
			"maxBulkHire":          50,
			"description":          "Coût moyen estimé pour embaucher un employé",
		})
	})

	// GET /api/employees/available - Get employees not assigned to anything (Machine, Deposit, Exploration)
	e.Router.GET("/api/employees/available", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Non connecté", nil)
		}

		companyId := authRecord.GetString("active_company")
		if companyId == "" {
			return apis.NewBadRequestError("Aucune entreprise active", nil)
		}

		// 1. Fetch all employees for this company that look free (deposit=empty && exploration=empty)
		// We still need to check Machine assignments which are on the Machine record
		employees, err := app.FindRecordsByFilter("employees", fmt.Sprintf("employer = '%s' && deposit = '' && exploration = '' && machine = ''", companyId), "-mining", 500, 0)
		if err != nil {
			return apis.NewBadRequestError("Erreur lors de la récupération des employés", err)
		}

		// Simplified: Since we now filter by machine='', deposit='', exploration='',
		// the returned employees are already available.
		availableEmployees := employees

		return c.JSON(200, map[string]interface{}{
			"items":      availableEmployees,
			"totalItems": len(availableEmployees),
		})
	})

}
