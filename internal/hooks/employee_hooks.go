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

		return nil
	})
}
