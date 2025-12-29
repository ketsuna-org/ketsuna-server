package hooks

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/pocketbase/pocketbase/tools/types"
)

// ProcessCompanyEconomy handles machine production for a single company
func (l *EconomyLogic) ProcessCompanyEconomy(companyId string) error {
	_, err := l.app.FindRecordById("companies", companyId)
	if err != nil {
		return err
	}

	// Fetch assigned machines
	assignedMachines, err := l.app.FindRecordsByFilter(
		"machines",
		fmt.Sprintf("company = '%s'", companyId),
		"",
		0,
		0,
	)
	if err != nil {
		return err // Or just return nil to skip if no machines? JS didn't fail hard.
	}

	if len(assignedMachines) == 0 {
		return nil
	}

	// Pre-fetch employees for efficiency calculation to avoid N+1 queries ideally,
	// but for now we follow the logic pattern. JS code fetched all employees of company once.
	employees, err := l.app.FindRecordsByFilter(
		"employees",
		fmt.Sprintf("employer = '%s'", companyId),
		"",
		0,
		0,
	)
	if err != nil {
		return err
	}

	for _, assignment := range assignedMachines {
		machineItemId := assignment.GetString("machine")
		if machineItemId == "" {
			continue
		}

		machineItem, err := l.app.FindRecordById("items", machineItemId)
		if err != nil {
			continue
		}

		// Calculate Efficiency
		assignedEmpIds := assignment.GetStringSlice("employees")
		totalEfficiency := 0.0

		for _, empId := range assignedEmpIds {
			for _, emp := range employees {
				if emp.Id == empId {
					totalEfficiency += emp.GetFloat("efficiency")
				}
			}
		}

		if totalEfficiency <= 0 {
			continue
		}

		// Calc Output Quantity
		productQty := float64(machineItem.GetInt("product_quantity"))
		if productQty == 0 {
			productQty = 1
		}

		finalQty := int(math.Floor(productQty * totalEfficiency))
		if finalQty < 1 {
			finalQty = 1
		}

		// If 0, skip production
		if finalQty == 0 {
			continue
		}

		recipeId := machineItem.GetString("use_recipe")
		productId := machineItem.GetString("product")

		if recipeId != "" {
			// --- RECIPE PRODUCTION ---
			recipe, err := l.app.FindRecordById("recipes", recipeId)
			if err != nil {
				continue
			}
			productionTime := recipe.GetInt("production_time")

			if productionTime > 60 {
				// Long production
				startedAt := assignment.GetDateTime("production_started_at")

				if startedAt.IsZero() {
					// Start Production
					_, err := l.inventory.ConsumeInputs(companyId, recipeId, finalQty)
					if err == nil {
						assignment.Set("production_started_at", types.NowDateTime())
						l.app.Save(assignment)
						log.Printf("[ECONOMY] Machine %s: Production Started.", machineItem.GetString("name"))
					}
				} else {
					// Check if finished
					elapsed := time.Since(startedAt.Time()).Seconds()
					if elapsed >= float64(productionTime) {
						err := l.inventory.CompleteProduction(companyId, recipeId, finalQty)
						if err == nil {
							// Reset
							assignment.Set("production_started_at", types.DateTime{}) // Zero
							l.app.Save(assignment)
							log.Printf("[ECONOMY] Machine %s: Production Terminée (+%d).", machineItem.GetString("name"), finalQty)
						} else {
							log.Printf("[ECONOMY] Machine %s: Erreur fin production: %v", machineItem.GetString("name"), err)
						}
					}
				}
			} else {
				// Short Production (Immediate)
				_, err := l.inventory.ProduceItem(companyId, recipeId, finalQty)
				if err == nil {
					log.Printf("[ECONOMY] Machine %s (Rapid): +%d", machineItem.GetString("name"), finalQty)
				}
			}

		} else if productId != "" {
			// --- PASSIVE PRODUCTION ---
			err := l.inventory.UpdateInventory(companyId, productId, finalQty)
			if err == nil {
				log.Printf("[ECONOMY] Machine %s (Passive): +%d", machineItem.GetString("name"), finalQty)
			}
		}
	}

	return nil
}

func (l *EconomyLogic) UpdateMarketPrices() {
	items, _ := l.app.FindRecordsByFilter("items", "", "", 0, 0)
	for _, item := range items {
		basePrice := item.GetFloat("base_price")
		volatility := item.GetFloat("volatility")
		if volatility == 0 {
			volatility = 0.1
		}

		variation := (rand.Float64() - 0.5) * 2 * volatility
		newPrice := basePrice * (1 + variation)

		if newPrice < 0.01 {
			newPrice = 0.01
		}

		item.Set("base_price", math.Round(newPrice*100)/100)
		l.app.Save(item)
	}
}

func (l *EconomyLogic) UpdateStockPrices() {
	stocks, _ := l.app.FindRecordsByFilter("stocks", "", "", 0, 0)
	for _, stock := range stocks {
		currentPrice := stock.GetFloat("share_price")
		if currentPrice == 0 {
			currentPrice = 10
		}

		change := (rand.Float64() - 0.5) * 0.1
		newPrice := currentPrice * (1 + change)

		if newPrice < 0.1 {
			newPrice = 0.1
		}

		stock.Set("share_price", newPrice)
		l.app.Save(stock)
	}
}

func (l *EconomyLogic) DeductDailyPayroll() {
	companies, _ := l.app.FindRecordsByFilter("companies", "", "", 0, 0)
	for _, company := range companies {
		employees, _ := l.app.FindRecordsByFilter("employees", fmt.Sprintf("employer = '%s'", company.Id), "", 0, 0)
		machines, _ := l.app.FindRecordsByFilter("machines", fmt.Sprintf("company = '%s'", company.Id), "", 0, 0)

		monthlyCost := 0.0

		for _, emp := range employees {
			monthlyCost += float64(emp.GetInt("salary") * 30)
		}

		monthlyCost += float64(len(machines) * 7)

		currentBalance := company.GetFloat("balance")
		newBalance := currentBalance - monthlyCost
		if newBalance < 0 {
			newBalance = 0
		}

		company.Set("balance", newBalance)
		l.app.Save(company)

		if monthlyCost > 0 {
			log.Printf("[PAYROLL] %s: -%.2f€ (Salaires: %d emp, Machines: %d)", company.GetString("name"), monthlyCost, len(employees), len(machines))
		}
	}
}
