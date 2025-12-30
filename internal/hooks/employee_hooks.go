package hooks

import (
	"fmt"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"math/rand"
)

func registerEmployeeHooks(app *pocketbase.PocketBase) {
	// Create: generate random stats and validate company balance
	app.OnRecordCreateRequest("employees").BindFunc(func(e *core.RecordRequestEvent) error {
		app.Logger().Info("[Hooks] Employee create request received", "auth", e.Auth)
		r := e.Record

		// employer either provided or from auth.active_company
		employer := r.GetString("employer")
		if employer == "" && e.Auth != nil {
			employer = e.Auth.GetString("active_company")
			r.Set("employer", employer)
		}

		if employer == "" {
			app.Logger().Error("[Hooks] Employee creation failed: No employer found")
			return apis.NewBadRequestError("L'employeur (employer) est requis", nil)
		}
		app.Logger().Info("[Hooks] Employee creation - Employer determined", "employerId", employer)

		// generate
		first := []string{"Jean", "Pierre", "Paul", "Jacques", "Marie", "Sophie", "Lucie", "Camille", "Thomas", "Nicolas", "Julien", "Antoine", "Lucas", "Emma", "Léa", "Chloé", "Manon", "Alex", "Maxime", "Léo", "Sarah", "Julie", "Hugo", "Gabriel", "Arthur"}
		last := []string{"Dupont", "Durand", "Martin", "Bernard", "Petit", "Robert", "Richard", "Simon", "Michel", "Lefebvre", "Moreau", "Laurent", "Garcia", "Roux", "David", "Bertrand", "Garnier", "Lambert", "Faure", "Rousseau", "Blanc", "Guerin", "Boyer", "Chevalier", "Mathieu"}
		postes := []string{"Ouvrier", "Technicien", "Ingénieur", "Superviseur", "Manutentionnaire", "Opérateur", "Analyste", "Logisticien", "Contremaître", "Directeur"}

		r.Set("name", fmt.Sprintf("%s %s", first[rand.Intn(len(first))], last[rand.Intn(len(last))]))
		r.Set("poste", postes[rand.Intn(len(postes))])

		randVal := rand.Float64()
		rarity := 0
		switch {
		case randVal > 0.99:
			rarity = 3
		case randVal > 0.90:
			rarity = 2
		case randVal > 0.60:
			rarity = 1
		}
		r.Set("rarity", rarity)

		var efficiencyBase float64
		switch rarity {
		case 3:
			efficiencyBase = 2.0
		case 2:
			efficiencyBase = 1.5
		case 1:
			efficiencyBase = 1.25
		default:
			efficiencyBase = 1.05
		}
		eff := efficiencyBase * (0.9 + rand.Float64()*0.2)
		r.Set("efficiency", fmt.Sprintf("%.2f", eff))

		var salaryBase int
		switch rarity {
		case 3:
			salaryBase = 260
		case 2:
			salaryBase = 130
		case 1:
			salaryBase = 65
		default:
			salaryBase = 26
		}
		salary := int(float64(salaryBase) * (0.9 + rand.Float64()*0.2))
		r.Set("salary", salary)

		// validate company balance
		company, err := e.App.FindRecordById("companies", employer)
		if err != nil {
			return apis.NewBadRequestError("Company introuvable ou erreur de validation", nil)
		}

		// Validate CEO ownership
		if e.Auth != nil && !e.Auth.IsSuperuser() && company.GetString("ceo") != e.Auth.Id {
			app.Logger().Error("[Hooks] creating employee forbidden: user is not CEO", "user", e.Auth.Id, "ceo", company.GetString("ceo"))
			return apis.NewForbiddenError("Vous n'êtes pas le PDG de cette entreprise", nil)
		}

		hiringFee := salary * 5
		requiredReserve := salary * 30
		totalRequired := hiringFee + requiredReserve
		balance := company.GetInt("balance")
		if balance < totalRequired {
            app.Logger().Error("[Hooks] Employee creation failed: Insufficient balance", "balance", balance, "required", totalRequired)
			return apis.NewBadRequestError(fmt.Sprintf("Balance insuffisante. %d€ requis", totalRequired), nil)
		}
        app.Logger().Info("[Hooks] Employee validation success, proceeding to create", "name", r.GetString("name"), "salary", salary)

		return nil
	})

	// After create: deduct fee
	app.OnRecordAfterCreateSuccess("employees").BindFunc(func(e *core.RecordEvent) error {
		record := e.Record
		employer := record.GetString("employer")
		if employer == "" {
			return nil
		}
		company, err := e.App.FindRecordById("companies", employer)
		if err != nil {
			e.App.Logger().Error("[Hooks] Erreur après création d'employé (company not found)", "error", err)
			return nil
		}
		salary := record.GetInt("salary")
		hiringFee := salary * 5
		current := company.GetInt("balance")
		company.Set("balance", current-hiringFee)
		if err := e.App.Save(company); err != nil {
			e.App.Logger().Error("[Hooks] Erreur save company after hire", "error", err)
		}
		e.App.Logger().Info("[Hooks] Frais de recrutement déduits", "amount", hiringFee, "company", company.GetString("name"))
		return nil
	})

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
