package hooks

import (
	"fmt"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func registerEmployeeHooks(app *pocketbase.PocketBase) {
	// Create: generate random stats and validate company balance -> MOVED TO ENDPOINT
	// Logic now in employee_logic.go and endpoints.go

	// Protect some fields on update
	app.OnRecordUpdateRequest("employees").BindFunc(func(e *core.RecordRequestEvent) error {
		isSuper := false
		if info, err := e.RequestInfo(); err == nil && info.HasSuperuserAuth() {
			isSuper = true
		}
		if !isSuper {
			original := e.Record.Original()
			protected := []string{"rarity", "efficiency", "employer"}
			for _, f := range protected {
				if e.Record.GetString(f) != original.GetString(f) {
					e.Record.Set(f, original.GetString(f))
				}
			}
		}

		// Check machine assignment conflict (deposits are no longer used for mining)
		newMachine := e.Record.GetString("machine")
		if newMachine != "" {
			// Ensure not assigned to exploration
			explorationId := e.Record.GetString("exploration")
			if explorationId != "" {
				return apis.NewBadRequestError("Cet employé est assigné à une exploration. Retirez-le avant de l'assigner à une machine.", nil)
			}
		}

		// salary increase check
		oldSalary := e.Record.Original().GetInt("salary")
		newSalary := e.Record.GetInt("salary")
		if newSalary > oldSalary*3/2 { // > old * 1.5
			employerId := e.Record.GetString("employer")
			if employerId != "" {
				company, err := e.App.FindRecordById("companies", employerId)
				if err == nil {
					balance := company.GetInt("balance")
					increase := newSalary - oldSalary
					required := increase * 30
					if balance < required {
						return apis.NewBadRequestError(fmt.Sprintf("Balance insuffisante pour cette augmentation. %d€ requis", required), nil)
					}
				}
			}
		}

		return e.Next()
	})
	// Fire (Delete): Decrement company employee_count
	app.OnRecordDeleteRequest("employees").BindFunc(func(e *core.RecordRequestEvent) error {
		// Run the delete first
		if err := e.Next(); err != nil {
			return err
		}

		employerId := e.Record.GetString("employer")
		if employerId != "" {
			company, err := app.FindRecordById("companies", employerId)
			if err == nil {
				count := company.GetInt("employee_count")
				if count > 0 {
					company.Set("employee_count", count-1)
					// We use SaveNoValidate to avoid triggering other hooks validation if possible/needed,
					// or just standard Save.
					app.Save(company)
				}
			}
		}
		return nil
	})
}
