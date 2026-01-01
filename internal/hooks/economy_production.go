package hooks

import (
	"fmt"
	"math"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// ProcessMachine handles the production logic for a single machine assignment
func (l *EconomyLogic) ProcessMachine(
	companyId string,
	assignment *core.Record,
	energyStatus EnergyStatus,
	employees []*core.Record, // All employees of the company (to filter by assignment)
) error {
	machineItemId := assignment.GetString("machine")
	if machineItemId == "" {
		return nil
	}

	// Ideally machine matching item is expanded in calling function
	machineItem := assignment.ExpandedOne("machine")
	if machineItem == nil {
		// Fallback fetch if not expanded (should be expanded for loop perf)
		var err error
		machineItem, err = l.app.FindRecordById("items", machineItemId)
		if err != nil {
			return err
		}
	}

	// 1. Calculate Efficiency
	assignedEmpIds := assignment.GetStringSlice("employees")
	totalEfficiency := 0.0

	// Simple loop over all company employees to find assigned ones
	// Optimization: This is O(N*M) where N=Assigned, M=TotalEmployees.
	// Since M is small (<50 usually), it's fine.
	for _, empId := range assignedEmpIds {
		for _, emp := range employees {
			if emp.Id == empId {
				totalEfficiency += emp.GetFloat("efficiency")
			}
		}
	}

	if totalEfficiency <= 0 {
		return nil // No employees, no production (or base 0)
	}

	// 2. Calculate Final Output Quantity
	productQty := float64(machineItem.GetInt("product_quantity"))
	if productQty == 0 {
		productQty = 1
	}

	finalQty := int(math.Floor(productQty * totalEfficiency))
	if finalQty < 1 {
		finalQty = 1
	}
	if finalQty == 0 {
		return nil
	}

	// 3. Energy Check & Penalty
	needEnergy := machineItem.GetFloat("need_energy")
	energyType := machineItem.GetString("energy_type")

	if energyType == "Soleil" && needEnergy > 0 {
		if !IsSolarProductionActive() {
			return nil // Sun not active
		}
	} else if needEnergy > 0 && energyStatus.ProductionSpeed < 1.0 {
		// Apply Energy Penalty (Time Shift)
		if energyStatus.ProductionSpeed <= 0 {
			return nil // No energy
		}

		// Shift start time to simulate slowdown
		startedAt := assignment.GetDateTime("production_started_at")
		if !startedAt.IsZero() {
			lastUpdate := assignment.GetDateTime("updated").Time()
			if !lastUpdate.IsZero() {
				delta := time.Since(lastUpdate).Seconds()
				if delta > 0 && delta < 3600 { // Cap at 1h to avoid issues
					penalty := delta * (1.0 - energyStatus.ProductionSpeed)
					newStart := startedAt.Time().Add(time.Duration(penalty * float64(time.Second)))
					assignment.Set("production_started_at", newStart)
					// We save later if production completes, or forces save here if critical?
					// Let's force save to persist the lag
					l.app.Save(assignment)
				}
			}
		}
	}

	// 4. Production Logic (Recipe or Passive)
	recipeId := machineItem.GetString("use_recipe")
	productId := machineItem.GetString("product")

	if recipeId != "" {
		return l.processRecipeProduction(companyId, assignment, machineItem, recipeId, finalQty)
	} else if productId != "" {
		return l.processPassiveProduction(companyId, assignment, machineItem, productId, finalQty)
	}

	return nil
}

func (l *EconomyLogic) processRecipeProduction(companyId string, assignment *core.Record, machineItem *core.Record, recipeId string, finalQty int) error {
	// Check technology
	hasTech, techName := l.inventory.HasRequiredTechnology(l.app, companyId, recipeId)
	if !hasTech {
		// Log only once?
		return fmt.Errorf("missing technology: %s", techName)
	}

	recipe, err := l.app.FindRecordById("recipes", recipeId)
	if err != nil {
		return err
	}
	baseProductionTime := recipe.GetInt("production_time")
	effectiveProductionTime := float64(baseProductionTime)

	if baseProductionTime > 0 {
		startedAt := assignment.GetDateTime("production_started_at")
		if startedAt.IsZero() {
			// Start
			_, err := l.inventory.ConsumeInputs(l.app, companyId, recipeId, finalQty)
			if err == nil {
				assignment.Set("production_started_at", types.NowDateTime())
				l.app.Save(assignment)
			}
		} else {
			// Finish
			elapsed := time.Since(startedAt.Time()).Seconds()
			if elapsed >= effectiveProductionTime {
				err := l.inventory.CompleteProduction(l.app, companyId, recipeId, finalQty)
				if err == nil {
					assignment.Set("production_started_at", types.DateTime{}) // Reset
					l.app.Save(assignment)
				}
			}
		}
	} else {
		// Immediate
		_, _ = l.inventory.ProduceItem(l.app, companyId, recipeId, finalQty)
	}
	return nil
}

func (l *EconomyLogic) processPassiveProduction(companyId string, assignment *core.Record, machineItem *core.Record, productId string, finalQty int) error {
	baseProductionTime := machineItem.GetInt("production_time")
	effectiveProductionTime := float64(baseProductionTime)

	// --- DEPOSIT CHECK LOGIC (New) ---
	depositId := assignment.GetString("deposit")
	var deposit *core.Record

	// If machine is linked to a deposit, we verify it
	if depositId != "" {
		var err error
		deposit, err = l.app.FindRecordById("deposits", depositId)
		if err != nil || deposit.GetFloat("quantity") <= 0 {
			// Deposit empty or invalid -> Stop production
			// TODO: Send notification if it just emptied?
			return fmt.Errorf("deposit empty or invalid")
		}

		// Apply Richness Multiplier to Production Speed or Quantity?
		// Let's say Richness multiplies Quantity
		richness := deposit.GetFloat("richness")
		if richness > 0 {
			finalQty = int(float64(finalQty) * richness)
		}
	}

	if baseProductionTime > 0 {
		startedAt := assignment.GetDateTime("production_started_at")
		if startedAt.IsZero() {
			// Start
			assignment.Set("production_started_at", types.NowDateTime())
			l.app.Save(assignment)
		} else {
			elapsed := time.Since(startedAt.Time()).Seconds()
			if elapsed >= effectiveProductionTime {
				// Finish

				// Cap quantity by deposit remaining
				if deposit != nil {
					remaining := deposit.GetFloat("quantity")
					if float64(finalQty) > remaining {
						finalQty = int(remaining)
					}

					// Decrement Deposit
					deposit.Set("quantity", remaining-float64(finalQty))
					l.app.Save(deposit)
				}

				err := l.inventory.UpdateInventory(l.app, companyId, productId, finalQty)
				if err == nil {
					assignment.Set("production_started_at", types.DateTime{}) // Reset
					l.app.Save(assignment)
				}
			}
		}
	} else {
		// Immediate
		if deposit != nil {
			remaining := deposit.GetFloat("quantity")
			if float64(finalQty) > remaining {
				finalQty = int(remaining)
			}
			deposit.Set("quantity", remaining-float64(finalQty))
			l.app.Save(deposit)
		}

		_ = l.inventory.UpdateInventory(l.app, companyId, productId, finalQty)
	}

	return nil
}
