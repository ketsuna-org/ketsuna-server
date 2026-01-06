package hooks

import (
	"fmt"
	"math"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
	"ketsuna.com/server/internal/gamedata"
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

	machineItem := gamedata.GetItem(machineItemId)
	if machineItem == nil {
		// If item is not in static data... maybe error or return?
		return fmt.Errorf("unknown machine item: %s", machineItemId)
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
	productQty := float64(machineItem.ProductQuantity)
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
	needEnergy := machineItem.NeedEnergy
	energyType := string(machineItem.EnergyType)

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
	recipeId := machineItem.UseRecipe
	productId := machineItem.Product

	if recipeId != "" {
		return l.processRecipeProduction(companyId, assignment, machineItem, recipeId, finalQty)
	} else if productId != "" {
		return l.processPassiveProduction(companyId, assignment, machineItem, productId, finalQty)
	}

	return nil
}

func (l *EconomyLogic) processRecipeProduction(companyId string, assignment *core.Record, _ *gamedata.Item, recipeId string, finalQty int) error {
	// Check technology
	hasTech, techName := l.inventory.HasRequiredTechnology(l.app, companyId, recipeId)
	if !hasTech {
		// Log only once?
		return fmt.Errorf("missing technology: %s", techName)
	}

	recipe := gamedata.GetRecipe(recipeId)
	if recipe == nil {
		return fmt.Errorf("unknown recipe: %s", recipeId)
	}
	baseProductionTime := recipe.ProductionTime
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

func (l *EconomyLogic) processPassiveProduction(companyId string, assignment *core.Record, machineItem *gamedata.Item, productId string, finalQty int) error {
	baseProductionTime := machineItem.ProductionTime
	effectiveProductionTime := float64(baseProductionTime)

	// --- DEPOSIT CHECK LOGIC ---
	// Fetch product item to check if it requires a deposit (minable)
	productItem := gamedata.GetItem(productId)
	if productItem == nil {
		return fmt.Errorf("unknown product item: %s", productId)
	}

	depositId := assignment.GetString("deposit")
	var deposit *core.Record

	if productItem.IsExplorable {
		// Strict Check: Must have a deposit if the item comes from exploration
		if depositId == "" {
			return fmt.Errorf("missing deposit for explorable resource")
		}
	}

	// If machine is linked to a deposit, we verify it (even if not strictly explorable, supports future use cases)
	if depositId != "" {
		var err error
		deposit, err = l.app.FindRecordById("deposits", depositId)
		if err != nil || deposit.GetFloat("quantity") < 1 {
			// Deposit empty or invalid -> Stop production (< 1 to handle floating point)
			return fmt.Errorf("deposit empty or invalid")
		}

		// Apply Size Level Multiplier to Production Quantity
		// Size is a level from 1-10, convert to multiplier (0.2 to 2.0)
		size := deposit.GetFloat("size")
		if size > 0 {
			multiplier := size / 5.0 // Level 5 = 1.0x, Level 10 = 2.0x, Level 1 = 0.2x
			finalQty = int(float64(finalQty) * multiplier)
			if finalQty < 1 {
				finalQty = 1
			}
		}
	} else if productItem.IsExplorable {
		// Redundant check but explicit: logic shouldn't reach here if explorable and no deposit
		return fmt.Errorf("missing deposit")
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
					if remaining < 1 {
						// Deposit already empty - unassign and delete
						assignment.Set("deposit", "")
						assignment.Set("production_started_at", types.DateTime{}) // Reset production
						l.app.Save(assignment)
						l.app.Delete(deposit)
						return nil // Skip production
					}
					if float64(finalQty) > remaining {
						finalQty = int(remaining)
					}
					if finalQty <= 0 {
						return nil // Nothing to produce
					}

					// Decrement Deposit
					newQty := remaining - float64(finalQty)
					deposit.Set("quantity", newQty)
					l.app.Save(deposit)

					// Delete deposit and unassign machine if empty
					if newQty < 1 {
						assignment.Set("deposit", "")
						l.app.Save(assignment)
						l.app.Delete(deposit)
					}
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
			if remaining < 1 {
				// Deposit already empty - unassign and delete
				assignment.Set("deposit", "")
				l.app.Save(assignment)
				l.app.Delete(deposit)
				return nil // Skip production
			}
			if float64(finalQty) > remaining {
				finalQty = int(remaining)
			}
			if finalQty <= 0 {
				return nil // Nothing to produce
			}
			newQty := remaining - float64(finalQty)
			deposit.Set("quantity", newQty)
			l.app.Save(deposit)

			// Delete deposit and unassign machine if empty
			if newQty < 1 {
				assignment.Set("deposit", "")
				l.app.Save(assignment)
				l.app.Delete(deposit)
			}
		}

		_ = l.inventory.UpdateInventory(l.app, companyId, productId, finalQty)
	}

	return nil
}
