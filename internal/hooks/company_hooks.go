package hooks

import (
	"fmt"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func registerCompanyHooks(app *pocketbase.PocketBase) {
	// On create: set defaults
	app.OnRecordCreateRequest("companies").BindFunc(func(e *core.RecordRequestEvent) error {
		r := e.Record
		if e.Auth != nil && r.GetString("ceo") == "" {
			r.Set("ceo", e.Auth.Id)
		}

		if r.GetString("is_npc") != "true" {
			r.Set("balance", 1000) // Starting balance for new players
		} else if r.GetString("balance") == "" {
			r.Set("balance", 0)
		}

		r.Set("level", 1)

		r.Set("payroll_daily_cost", 0)

		return e.Next()
	})

	// Prevent deletion when related records exist
	app.OnRecordDeleteRequest("companies").BindFunc(func(e *core.RecordRequestEvent) error {
		company := e.Record
		companyId := company.Id

		employees, _ := e.App.FindRecordsByFilter("employees", fmt.Sprintf("employer = \"%s\"", companyId), "-created", 1, 0)
		if len(employees) > 0 {
			return apis.NewBadRequestError("Impossible de supprimer une entreprise qui a encore des employés", nil)
		}

		inv, _ := e.App.FindRecordsByFilter("inventory", fmt.Sprintf("company = \"%s\"", companyId), "-created", 1, 0)
		if len(inv) > 0 {
			return apis.NewBadRequestError("Impossible de supprimer une entreprise qui a encore du stock", nil)
		}

		// simple check for shareholders/stocks presence
		hs1, _ := e.App.FindRecordsByFilter("shareholders", fmt.Sprintf("holder_company = \"%s\"", companyId), "-created", 1, 0)
		hs2, _ := e.App.FindRecordsByFilter("shareholders", fmt.Sprintf("stock.company = \"%s\"", companyId), "-created", 1, 0)
		if len(hs1) > 0 || len(hs2) > 0 {
			return apis.NewBadRequestError("Impossible de supprimer une entreprise liée à des actions", nil)
		}

		return e.Next()
	})

	// After create: attach to owner's owned_companies and create CEO employee
	app.OnRecordAfterCreateSuccess("companies").BindFunc(func(e *core.RecordEvent) error {
		company := e.Record
		companyId := company.Id
		companyName := company.GetString("name")
		ceoId := company.GetString("ceo")

		// Create CEO employee for this company
		EnsureCEOOnCompanyCreation(app, companyId, companyName)

		// Create default Wood Deposit
		EnsureWoodDepositOnCompanyCreation(app, companyId)

		if ceoId == "" {
			return nil
		}

		user, err := e.App.FindRecordById("users", ceoId)
		if err != nil {
			e.App.Logger().Error("Erreur mise à jour owned_companies", "error", err)
			return nil
		}

		// append company id to owned_companies
		user.Set("owned_companies+", []string{company.Id})
		if user.GetString("active_company") == "" {
			user.Set("active_company", company.Id)
		}
		if err := e.App.Save(user); err != nil {
			e.App.Logger().Error("Erreur mise à jour owned_companies (save)", "error", err)
		}
		return nil
	})
}
