package hooks

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// package-local RNG (preferred over global rand.Seed as of Go 1.20)
var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

// RegisterHooks registers a subset of game hooks (companies, employees, inventory, recipes)
// and starts a simple economy ticker (cron-like) to simulate the JS hooks behavior.
func RegisterHooks(app *pocketbase.PocketBase) {
	invLogic := NewInventoryLogic(app)
	ecoLogic := NewEconomyLogic(app, invLogic)

	registerCompanyHooks(app)
	registerEmployeeHooks(app)
	registerInventoryHooks(app)
	registerRecipeHooks(app)
	registerMachineHooks(app, invLogic)
	RegisterEndpoints(app, invLogic)
	startEconomyTicker(app, ecoLogic)
}

func registerCompanyHooks(app *pocketbase.PocketBase) {
	// On create: set defaults
	app.OnRecordCreateRequest("companies").BindFunc(func(e *core.RecordRequestEvent) error {
		r := e.Record
		if e.Auth != nil && r.GetString("ceo") == "" {
			r.Set("ceo", e.Auth.Id)
		}

		if r.GetString("is_npc") != "true" {
			r.Set("balance", 10000)
		} else if r.GetString("balance") == "" {
			r.Set("balance", 10000)
		}

		r.Set("level", 1)
		r.Set("tech_points", 0)

		if r.GetString("reputation") == "" {
			r.Set("reputation", 50)
		}

		r.Set("payroll_daily_cost", 0)

		return nil
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

		return nil
	})

	// After create: attach to owner's owned_companies
	app.OnRecordAfterCreateSuccess("companies").BindFunc(func(e *core.RecordEvent) error {
		company := e.Record
		ceoId := company.GetString("ceo")
		if ceoId == "" {
			return nil
		}

		user, err := e.App.FindRecordById("users", ceoId)
		if err != nil {
			log.Println("Erreur mise à jour owned_companies:", err)
			return nil
		}

		// append company id to owned_companies
		user.Set("owned_companies+", []string{company.Id})
		if user.GetString("active_company") == "" {
			user.Set("active_company", company.Id)
		}
		if err := e.App.Save(user); err != nil {
			log.Println("Erreur mise à jour owned_companies (save):", err)
		}
		return nil
	})
}

func registerEmployeeHooks(app *pocketbase.PocketBase) {
	// Create: generate random stats and validate company balance
	app.OnRecordCreateRequest("employees").BindFunc(func(e *core.RecordRequestEvent) error {
		r := e.Record

		// employer either provided or from auth.active_company
		employer := r.GetString("employer")
		if employer == "" && e.Auth != nil {
			employer = e.Auth.GetString("active_company")
			r.Set("employer", employer)
		}

		if employer == "" {
			return apis.NewBadRequestError("L'employeur (employer) est requis", nil)
		}

		// generate
		first := []string{"Jean", "Pierre", "Paul", "Jacques", "Marie", "Sophie", "Lucie", "Camille", "Thomas", "Nicolas"}
		last := []string{"Dupont", "Durand", "Martin", "Bernard", "Petit", "Robert", "Richard"}
		postes := []string{"Ouvrier", "Technicien", "Ingénieur", "Superviseur", "Directeur"}

		r.Set("name", fmt.Sprintf("%s %s", first[rng.Intn(len(first))], last[rng.Intn(len(last))]))
		r.Set("poste", postes[rng.Intn(len(postes))])

		randVal := rng.Float64()
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
		eff := efficiencyBase * (0.9 + rng.Float64()*0.2)
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
		salary := int(float64(salaryBase) * (0.9 + rng.Float64()*0.2))
		r.Set("salary", salary)

		// validate company balance
		company, err := e.App.FindRecordById("companies", employer)
		if err != nil {
			return apis.NewBadRequestError("Company introuvable ou erreur de validation", nil)
		}

		hiringFee := salary * 5
		requiredReserve := salary * 30
		totalRequired := hiringFee + requiredReserve
		balance := company.GetInt("balance")
		if balance < totalRequired {
			return apis.NewBadRequestError(fmt.Sprintf("Balance insuffisante. %d€ requis", totalRequired), nil)
		}

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
			log.Println("[Hooks] Erreur après création d'employé (company not found):", err)
			return nil
		}
		salary := record.GetInt("salary")
		hiringFee := salary * 5
		current := company.GetInt("balance")
		company.Set("balance", current-hiringFee)
		if err := e.App.Save(company); err != nil {
			log.Println("[Hooks] Erreur save company after hire:", err)
		}
		log.Printf("[Hooks] Frais de recrutement déduits (-%d€) pour %s\n", hiringFee, company.GetString("name"))
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

func registerInventoryHooks(app *pocketbase.PocketBase) {
	app.OnRecordCreateRequest("inventory").BindFunc(func(e *core.RecordRequestEvent) error {
		r := e.Record
		companyId := r.GetString("company")
		itemId := r.GetString("item")
		qty := r.GetInt("quantity")
		if companyId == "" || itemId == "" {
			return apis.NewBadRequestError("Company et Item sont requis", nil)
		}
		if qty <= 0 {
			return apis.NewBadRequestError("La quantité doit être supérieure à 0", nil)
		}

		company, err := e.App.FindRecordById("companies", companyId)
		if err != nil {
			return apis.NewBadRequestError("Company introuvable", nil)
		}
		item, err := e.App.FindRecordById("items", itemId)
		if err != nil {
			return apis.NewBadRequestError("Item introuvable", nil)
		}

		// If auth is CEO -> purchase
		if e.Auth != nil && e.Auth.Id == company.GetString("ceo") {
			itemPrice := item.GetInt("base_price")
			totalCost := itemPrice * qty
			current := company.GetInt("balance")
			if current < totalCost {
				return apis.NewBadRequestError(fmt.Sprintf("Fonds insuffisants. Coût: %d€, Solde: %d€", totalCost, current), nil)
			}
			company.Set("balance", current-totalCost)
			if err := e.App.Save(company); err != nil {
				log.Println("[PURCHASE] erreur save company:", err)
			}
			log.Printf("[PURCHASE] Company %s purchased item %s x%d for %d€\n", companyId, itemId, qty, totalCost)
		}

		// If inventory exists, update quantity and return an error informing about it
		existing, _ := e.App.FindFirstRecordByFilter("inventory", fmt.Sprintf("company='%s' && item='%s'", companyId, itemId))
		if existing != nil {
			curr := existing.GetInt("quantity")
			existing.Set("quantity", curr+qty)
			if err := e.App.Save(existing); err != nil {
				log.Println("Erreur mise à jour inventaire:", err)
			}
			return apis.NewBadRequestError(fmt.Sprintf("Item déjà en inventaire. Quantité mise à jour: %d", curr+qty), nil)
		}

		return nil
	})

	app.OnRecordUpdateRequest("inventory").BindFunc(func(e *core.RecordRequestEvent) error {
		rec := e.Record
		orig := rec.Original()
		if rec.GetString("item") != orig.GetString("item") || rec.GetString("company") != orig.GetString("company") {
			return apis.NewBadRequestError("Action illégale : Impossible de transférer cet inventaire.", nil)
		}

		company, err := e.App.FindRecordById("companies", rec.GetString("company"))
		if err != nil {
			return apis.NewBadRequestError("Company introuvable", nil)
		}
		companyAuthId := company.GetString("ceo")
		if e.Auth == nil || e.Auth.Id != companyAuthId {
			return apis.NewForbiddenError("Seul le CEO peut modifier l'inventaire", nil)
		}

		oldQ := orig.GetInt("quantity")
		newQ := rec.GetInt("quantity")
		if newQ > oldQ {
			added := newQ - oldQ
			item, _ := e.App.FindRecordById("items", rec.GetString("item"))
			price := item.GetInt("base_price")
			totalCost := price * added
			bal := company.GetInt("balance")
			if bal < totalCost {
				return apis.NewBadRequestError(fmt.Sprintf("Fonds insuffisants pour acheter %d items. Coût: %d€, Solde: %d€", added, totalCost, bal), nil)
			}
			company.Set("balance", bal-totalCost)
			if err := e.App.Save(company); err != nil {
				log.Println("[PURCHASE-UPDATE] erreur save company:", err)
			}
			log.Printf("[PURCHASE-UPDATE] Company %s purchased %d more items for %d€\n", company.Id, added, totalCost)
		}

		if newQ < 0 {
			return apis.NewBadRequestError("La quantité ne peut pas être négative", nil)
		}

		return nil
	})

	app.OnRecordAfterUpdateSuccess("inventory").BindFunc(func(e *core.RecordEvent) error {
		if e.Record.GetInt("quantity") == 0 {
			if err := e.App.Delete(e.Record); err != nil {
				log.Println("Erreur lors de la suppression de l'inventaire:", err)
			}
		}
		return nil
	})
}

func registerRecipeHooks(app *pocketbase.PocketBase) {
	// recipes are admin-only for create/update/delete
	app.OnRecordCreateRequest("recipes").BindFunc(func(e *core.RecordRequestEvent) error {
		info, err := e.RequestInfo()
		if err != nil || !info.HasSuperuserAuth() {
			return apis.NewBadRequestError("Seuls les administrateurs peuvent créer des recettes", nil)
		}
		return nil
	})

	app.OnRecordUpdateRequest("recipes").BindFunc(func(e *core.RecordRequestEvent) error {
		info, err := e.RequestInfo()
		if err != nil || !info.HasSuperuserAuth() {
			return apis.NewBadRequestError("Seuls les administrateurs peuvent modifier des recettes", nil)
		}
		return nil
	})

	app.OnRecordDeleteRequest("recipes").BindFunc(func(e *core.RecordRequestEvent) error {
		info, err := e.RequestInfo()
		if err != nil || !info.HasSuperuserAuth() {
			return apis.NewBadRequestError("Seuls les administrateurs peuvent supprimer des recettes", nil)
		}
		return nil
	})
}

func registerMachineHooks(app *pocketbase.PocketBase, inv *InventoryLogic) {
	app.OnRecordCreateRequest("machines").BindFunc(func(e *core.RecordRequestEvent) error {
		record := e.Record
		companyId := record.GetString("company")
		machineItemId := record.GetString("machine")
		employeeIds := record.GetStringSlice("employees")

		if companyId == "" || machineItemId == "" {
			return apis.NewBadRequestError("ID de compagnie ou de machine manquant.", nil)
		}

		// 1. Validate employees not assigned elsewhere
		if len(employeeIds) > 0 {
			// Find other machines with these employees
			// filter usage: employees ~ 'id' || employees ~ 'id2'
			// This is complex to build in simple string, loop
			for _, empId := range employeeIds {
				found, err := app.FindRecordsByFilter("machines", fmt.Sprintf("employees ~ '%s'", empId), "", 1, 0)
				if err == nil && len(found) > 0 {
					return apis.NewBadRequestError("Un ou plusieurs employés sont déjà assignés à une autre machine.", nil)
				}
			}
		}

		// 2. Check Inventory Stock
		if !inv.HasEnoughItems(companyId, machineItemId, 1) {
			return apis.NewBadRequestError("Vous n'avez pas cette machine en stock dans votre inventaire.", nil)
		}

		// 3. Deduct from Inventory
		if err := inv.UpdateInventory(companyId, machineItemId, -1); err != nil {
			return apis.NewBadRequestError(fmt.Sprintf("Erreur lors de la mise à jour de l'inventaire: %v", err), nil)
		}
		
		log.Printf("[MACHINES] Machine %s assignée pour company %s. Stock déduit.\n", machineItemId, companyId)
		return nil
	})

	app.OnRecordUpdateRequest("machines").BindFunc(func(e *core.RecordRequestEvent) error {
		record := e.Record
		employeeIds := record.GetStringSlice("employees")
		
		if len(employeeIds) > 0 {
			for _, empId := range employeeIds {
				// exclude current machine
				filter := fmt.Sprintf("id != '%s' && employees ~ '%s'", record.Id, empId)
				found, err := app.FindRecordsByFilter("machines", filter, "", 1, 0)
				if err == nil && len(found) > 0 {
					return apis.NewBadRequestError("Un ou plusieurs employés sont déjà assignés à une autre machine.", nil)
				}
			}
		}
		return nil
	})

	app.OnRecordDeleteRequest("machines").BindFunc(func(e *core.RecordRequestEvent) error {
		record := e.Record
		companyId := record.GetString("company")
		machineItemId := record.GetString("machine")

		if companyId != "" && machineItemId != "" {
			if err := inv.UpdateInventory(companyId, machineItemId, 1); err != nil {
				log.Printf("[MACHINES] Erreur remise en stock: %v\n", err)
			} else {
				log.Printf("[MACHINES] Assignation supprimée. Machine %s renvoyée au stock.\n", machineItemId)
			}
		}
		return nil
	})
}

func startEconomyTicker(app *pocketbase.PocketBase, eco *EconomyLogic) {
	// start a minute ticker to simulate economy cron
	go func() {
		t := time.NewTicker(1 * time.Minute)
		defer t.Stop()
		for range t.C {
			log.Println("[CRON] Economy tick triggered at", time.Now().UTC().Format(time.RFC3339))
			companies, err := app.FindRecordsByFilter("companies", "", "", 0, 0)
			if err != nil {
				log.Println("[CRON] cannot list companies:", err)
				continue
			}
			
			// Process Machine Production
			for _, c := range companies {
				if err := eco.ProcessCompanyEconomy(c.Id); err != nil {
					log.Printf("[CRON] Error processing company %s: %v", c.Id, err)
				}
			}
			
			// Update Stock Prices (Every tick?) -> JS did it every tick
			eco.UpdateStockPrices()
		}
	}()

	// daily market price update ticker (every 24h)
	go func() {
		t := time.NewTicker(24 * time.Hour)
		defer t.Stop()
		for range t.C {
			log.Println("[CRON] Market price update triggered at", time.Now().UTC().Format(time.RFC3339))
			eco.UpdateMarketPrices()
			eco.DeductDailyPayroll()
		}
	}()
}
