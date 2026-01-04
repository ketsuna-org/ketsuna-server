package hooks

import (
	"fmt"
	"math/rand"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

type EmployeeLogic struct {
	app *pocketbase.PocketBase
}

func NewEmployeeLogic(app *pocketbase.PocketBase) *EmployeeLogic {
	return &EmployeeLogic{app: app}
}

type HiredEmployee struct {
	Record   *core.Record `json:"record"`
	Cost     int          `json:"cost"`
	NewSaldo int          `json:"newSaldo"`
}

func (el *EmployeeLogic) HireEmployee(app core.App, companyId string) (*HiredEmployee, error) {
	// 1. Generate Stats
	first := []string{"Jean", "Pierre", "Paul", "Jacques", "Marie", "Sophie", "Lucie", "Camille", "Thomas", "Nicolas", "Julien", "Antoine", "Lucas", "Emma", "Léa", "Chloé", "Manon", "Alex", "Maxime", "Léo", "Sarah", "Julie", "Hugo", "Gabriel", "Arthur"}
	last := []string{"Dupont", "Durand", "Martin", "Bernard", "Petit", "Robert", "Richard", "Simon", "Michel", "Lefebvre", "Moreau", "Laurent", "Garcia", "Roux", "David", "Bertrand", "Garnier", "Lambert", "Faure", "Rousseau", "Blanc", "Guerin", "Boyer", "Chevalier", "Mathieu"}
	// Use exact SELECT values from pb_schema.json (excluding PDG which is only for CEOs)
	postes := []string{"Manutentionnaire", "Opérateur", "Ouvrier", "Mineur", "Explorateur"}

	name := fmt.Sprintf("%s %s", first[rand.Intn(len(first))], last[rand.Intn(len(last))])
	poste := postes[rand.Intn(len(postes))]

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
	efficiencyFormatted := fmt.Sprintf("%.2f", eff)

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

	// 2. Calculate Costs
	hiringFee := salary * 5
	requiredReserve := salary * 30
	totalRequired := hiringFee + requiredReserve

	// 3. Transaction
	company, err := app.FindRecordById("companies", companyId)
	if err != nil {
		return nil, fmt.Errorf("company not found")
	}

	balance := company.GetInt("balance")
	if balance < totalRequired {
		return nil, fmt.Errorf("balance insuffisante. %d€ requis (Frais: %d + Réserve: %d)", totalRequired, hiringFee, requiredReserve)
	}

	// 4. Create Employee Record
	collection, err := app.FindCollectionByNameOrId("employees")
	if err != nil {
		return nil, err
	}

	record := core.NewRecord(collection)
	record.Set("employer", companyId)
	record.Set("name", name)
	record.Set("poste", poste)
	record.Set("rarity", rarity)
	record.Set("efficiency", efficiencyFormatted)
	record.Set("salary", salary)

	// New stats - random values based on rarity
	baseStat := rarity + 1                                // 1-4 base depending on rarity
	record.Set("mining", baseStat+rand.Intn(3))           // 1-6
	record.Set("exploration_luck", baseStat+rand.Intn(3)) // 1-6
	record.Set("energy", 50+rand.Intn(50))                // 50-100
	record.Set("maintenance", baseStat+rand.Intn(3))      // 1-6

	if err := app.Save(record); err != nil {
		return nil, fmt.Errorf("failed to create employee record: %v", err)
	}

	// 5. Deduct Balance & Increment Count
	company.Set("balance", balance-hiringFee)
	company.Set("employee_count", company.GetInt("employee_count")+1)
	if err := app.Save(company); err != nil {
		// Rollback employee creation if money deduction fails?
		// ideally yes, but for now let's just error
		app.Delete(record)
		return nil, fmt.Errorf("failed to update company balance: %v", err)
	}

	return &HiredEmployee{
		Record:   record,
		Cost:     hiringFee,
		NewSaldo: balance - hiringFee,
	}, nil
}
