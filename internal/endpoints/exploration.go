package endpoints

import (
	"fmt"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func registerExplorationEndpoints(app *pocketbase.PocketBase, e *core.ServeEvent) {

	// POST /api/exploration/start - Start a new exploration mission
	e.Router.POST("/api/exploration/start", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Non connecté", nil)
		}

		data := struct {
			EmployeeId string `json:"employeeId"`
			ResourceId string `json:"resourceId"`
		}{}
		// Support old/new field names just in case, though frontend sends employeeId and resourceId
		if err := c.BindBody(&data); err != nil {
			return apis.NewBadRequestError("Invalid JSON", err)
		}

		if data.EmployeeId == "" {
			return apis.NewBadRequestError("employeeId requis", nil)
		}
		if data.ResourceId == "" {
			return apis.NewBadRequestError("resourceId requis", nil)
		}

		return app.RunInTransaction(func(txApp core.App) error {
			// 1. Fetch Employee
			employee, err := txApp.FindRecordById("employees", data.EmployeeId)
			if err != nil {
				return apis.NewBadRequestError("Employé introuvable", nil)
			}

			// 2. Check Ownership
			companyId := employee.GetString("employer")
			company, err := txApp.FindRecordById("companies", companyId)
			if err != nil {
				return apis.NewBadRequestError("Entreprise introuvable", nil)
			}

			if !authRecord.IsSuperuser() {
				if company.GetString("ceo") != authRecord.Id {
					return apis.NewForbiddenError("Cet employé ne vous appartient pas", nil)
				}
			}

			// 3. Check Eligibility (Must be Explorateur and Idle)
			if employee.GetString("poste") != "Explorateur" {
				return apis.NewBadRequestError("Seul un Explorateur peut partir en mission", nil)
			}
			if employee.GetString("deposit") != "" || employee.GetString("exploration") != "" {
				return apis.NewBadRequestError("Cet employé est déjà occupé", nil)
			}

			// 4. Create Exploration Record
			exCollection, err := txApp.FindCollectionByNameOrId("explorations")
			if err != nil {
				return err
			}

			exRecord := core.NewRecord(exCollection)
			exRecord.Set("company", companyId)
			exRecord.Set("employee", data.EmployeeId)
			exRecord.Set("target_resource_id", data.ResourceId)
			exRecord.Set("status", "En cours")

			// Set expiration based on level logic (kept from old implementation but simplified)
			// Or just set to now + fixed time for now
			duration := 15 * time.Minute // 15 mins
			exRecord.Set("end_time", time.Now().Add(duration))

			if err := txApp.Save(exRecord); err != nil {
				return fmt.Errorf("failed to create exploration: %w", err)
			}

			// 5. Update Employee Status
			employee.Set("exploration", exRecord.Id)
			if err := txApp.Save(employee); err != nil {
				return fmt.Errorf("failed to update employee status: %w", err)
			}

			return c.JSON(200, map[string]interface{}{
				"success":       true,
				"message":       "Mission d'exploration lancée",
				"explorationId": exRecord.Id,
				"endTime":       exRecord.GetDateTime("end_time"),
			})
		})
	})

	// GET /api/explorations - List recent explorations for the company
	e.Router.GET("/api/explorations", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Non connecté", nil)
		}
		companyId := authRecord.GetString("active_company")

		records, err := app.FindRecordsByFilter(
			"explorations",
			fmt.Sprintf("company = '%s'", companyId),
			"-created",
			100,
			0,
		)
		if err != nil {
			return apis.NewBadRequestError("Erreur list", err)
		}

		// Expand resource info and employee info
		for _, r := range records {
			app.ExpandRecord(r, []string{"target_resource", "employee"}, nil)
		}

		return c.JSON(200, records)
	})

	// POST /api/exploration/acknowledge - Acknowledge a mission result and free the employee
	e.Router.POST("/api/exploration/acknowledge", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Non connecté", nil)
		}

		data := struct {
			ExplorationId string `json:"explorationId"`
		}{}
		if err := c.BindBody(&data); err != nil {
			return apis.NewBadRequestError("Invalid JSON", err)
		}

		if data.ExplorationId == "" {
			return apis.NewBadRequestError("explorationId requis", nil)
		}

		return app.RunInTransaction(func(txApp core.App) error {
			// 1. Fetch Exploration
			exploration, err := txApp.FindRecordById("explorations", data.ExplorationId)
			if err != nil {
				return apis.NewBadRequestError("Exploration introuvable", nil)
			}

			// 2. Check Ownership
			companyId := exploration.GetString("company")
			company, err := txApp.FindRecordById("companies", companyId)
			if err != nil {
				return apis.NewBadRequestError("Entreprise introuvable", nil)
			}
			if !authRecord.IsSuperuser() && company.GetString("ceo") != authRecord.Id {
				return apis.NewForbiddenError("Cette mission ne vous appartient pas", nil)
			}

			// 3. Check status (must be resolved)
			status := exploration.GetString("status")
			if status == "En cours" {
				return apis.NewBadRequestError("La mission est encore en cours", nil)
			}

			// 4. Free Employee
			employeeId := exploration.GetString("employee")
			if employeeId != "" {
				employee, err := txApp.FindRecordById("employees", employeeId)
				if err == nil {
					employee.Set("exploration", "")
					if err := txApp.Save(employee); err != nil {
						app.Logger().Error("Failed to clear employee exploration", "id", employeeId, "error", err)
					}
				}
			}

			// 5. Delete or Resolve the exploration record?
			// Let's delete it to keep DB clean, or we could add a 'resolved' bool field.
			// User said "failed exploration are not shown", showing them AFTER failure but BEFORE acknowledge is the goal.
			// Once acknowledged, they should disappear from the modal.
			if err := txApp.Delete(exploration); err != nil {
				return fmt.Errorf("failed to delete exploration: %w", err)
			}

			return c.JSON(200, map[string]interface{}{
				"success": true,
				"message": "Mission acquittée",
			})
		})
	})
}
