package hooks

import (
	"fmt"
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
			// Check technology requirement first
			hasTech, techName := l.inventory.HasRequiredTechnology(companyId, recipeId)
			if !hasTech {
				l.app.Logger().Info("[ECONOMY] Machine blocked: missing technology",
					"machine", machineItem.GetString("name"),
					"tech", techName)
				continue
			}

			recipe, err := l.app.FindRecordById("recipes", recipeId)
			if err != nil {
				continue
			}
			productionTime := recipe.GetInt("production_time")

			if productionTime > 0 {
				// Timed production (Any duration > 0)
				startedAt := assignment.GetDateTime("production_started_at")

				if startedAt.IsZero() {
					// Start Production
					_, err := l.inventory.ConsumeInputs(companyId, recipeId, finalQty)
					if err == nil {
						assignment.Set("production_started_at", types.NowDateTime())
						l.app.Save(assignment)
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
						} else {
							l.app.Logger().Error("[ECONOMY] Machine Production Error", "machine", machineItem.GetString("name"), "error", err)
						}
					}
				}
			} else {
				// Short Production (Immediate) - Only if production_time is 0
				_, _ = l.inventory.ProduceItem(companyId, recipeId, finalQty)
			}

		} else if productId != "" {
			// --- PASSIVE PRODUCTION ---
			productionTime := machineItem.GetInt("production_time")

			if productionTime > 0 {
				// Timed Passive Production (Even for short times, as requested)
				startedAt := assignment.GetDateTime("production_started_at")

				if startedAt.IsZero() {
					// Start Production
					assignment.Set("production_started_at", types.NowDateTime())
					l.app.Save(assignment)
				} else {
					// Check if finished
					elapsed := time.Since(startedAt.Time()).Seconds()
					if elapsed >= float64(productionTime) {
						// Complete Production
						err := l.inventory.UpdateInventory(companyId, productId, finalQty)
						if err == nil {
							// Reset
							assignment.Set("production_started_at", types.DateTime{}) // Zero
							l.app.Save(assignment)
						} else {
							l.app.Logger().Error("[ECONOMY] Passive Production Error", "machine", machineItem.GetString("name"), "error", err)
						}
					}
				}
			} else {
				// Immediate Production (Every tick) - Only if production_time is 0 or missing
				_ = l.inventory.UpdateInventory(companyId, productId, finalQty)
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
			l.app.Logger().Info("[PAYROLL] Deducted", "company", company.GetString("name"), "amount", monthlyCost, "employees", len(employees), "machines", len(machines))
		}
	}
}

func (l *EconomyLogic) SellReserveItems() {
	reserves, _ := l.app.FindRecordsByFilter("reserve", "", "", 0, 0)
	if len(reserves) == 0 {
		return
	}

	for _, res := range reserves {
		companyId := res.GetString("company")
		itemId := res.GetString("item")
		qty := res.GetInt("quantity")

		if qty <= 0 {
			continue
		}

		item, err := l.app.FindRecordById("items", itemId)
		if err != nil {
			continue
		}

		price := item.GetFloat("base_price")
		revenue := float64(qty) * price

		company, err := l.app.FindRecordById("companies", companyId)
		if err != nil {
			continue
		}

		// Update Company Balance
		newBalance := company.GetFloat("balance") + revenue
		company.Set("balance", newBalance)
		l.app.Save(company)

		// Delete Reserve Record (Emptying it)
		// Or Set to 0 if we want to keep the slot?
		// User said "Only the quantity decreases".
		// Deleting is cleaner for the DB.
		l.app.Delete(res)

		l.app.Logger().Info("[CRON] Sold Reserve Item", "company", company.GetString("name"), "item", item.GetString("name"), "qty", qty, "revenue", revenue)
	}
}
