package hooks

import (
	"fmt"
	"math"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"ketsuna.com/server/internal/gamedata"
)

const (
	// GraphHarvestInterval defines the time unit for mining rates (e.g., rate per 60 seconds)
	GraphHarvestInterval = 60.0
)

type GraphEconomy struct {
	app *pocketbase.PocketBase
}

func NewGraphEconomy(app *pocketbase.PocketBase) *GraphEconomy {
	return &GraphEconomy{app: app}
}

// CalculateCompanyInventory triggers a pull for all resources flowing into the company
// returning a map of ItemID -> Quantity accumulated/available.
// This function MODIFIES the database (moves items from buffers to company inventory).
func (g *GraphEconomy) CalculateCompanyInventory(companyId string) (map[string]float64, error) {
	// 1. Find all edges where Output = Company
	edges, err := g.app.FindRecordsByFilter(
		"edge_relation",
		fmt.Sprintf("output_type = 'company' && output_id = '%s'", companyId),
		"",
		0,
		0,
	)
	if err != nil {
		return nil, err
	}

	totalFlow := make(map[string]float64)

	for _, edge := range edges {
		inputType := edge.GetString("input_type")
		inputId := edge.GetString("input_id")

		// Calculate flow from this specific input
		flow, itemId, err := g.CalculateNodeFlow(inputId, inputType)
		if err != nil {
			g.app.Logger().Error("Graph calc error", "node", inputId, "err", err)
			continue
		}

		if flow > 0 && itemId != "" {
			totalFlow[itemId] += flow
		}
	}

	// 2. Commit the flow to the Company's actual Inventory
	// (This acts as the "Catchment" logic)
	// We need to fetch/create inventory records for the company and add the flow.
	if len(totalFlow) > 0 {
		// Use InventoryLogic (need to instantiate or pass it, for now inline simple logic)
		// Better: expose a method in InventoryLogic or use record ops directly.
		// I will use direct DB ops for now to keep it self-contained in this Refactor.

		for itemId, qty := range totalFlow {
			// Find existing inventory
			records, _ := g.app.FindRecordsByFilter(
				"inventory",
				fmt.Sprintf("company = '%s' && item_id = '%s'", companyId, itemId),
				"",
				1,
				0,
			)

			var rec *core.Record
			if len(records) > 0 {
				rec = records[0]
				newQty := rec.GetFloat("quantity") + qty
				rec.Set("quantity", newQty)
			} else {
				// Create new
				collection, _ := g.app.FindCollectionByNameOrId("inventory")
				rec = core.NewRecord(collection)
				rec.Set("company", companyId)
				rec.Set("item_id", itemId)
				rec.Set("quantity", qty)
			}

			if err := g.app.Save(rec); err != nil {
				g.app.Logger().Error("Failed to save inventory update", "item", itemId, "err", err)
			}
		}
	}

	return totalFlow, nil
}

// CalculateNodeFlow calculates how much a node (Deposit/Machine) can output NOW.
// This function updates the Node's state (timers, quantity) to reflect the extraction.
func (g *GraphEconomy) CalculateNodeFlow(nodeId string, nodeType string) (float64, string, error) {
	switch nodeType {
	case "deposit":
		return g.processDeposit(nodeId)
	case "machine":
		return g.processMachine(nodeId)
	default:
		return 0, "", nil
	}
}

func (g *GraphEconomy) processDeposit(depositId string) (float64, string, error) {
	deposit, err := g.app.FindRecordById("deposits", depositId)
	if err != nil {
		return 0, "", err
	}

	// Check Resource Type
	resourceId := deposit.GetString("ressource_id")
	if resourceId == "" {
		return 0, "", nil
	}

	// 1. Calculate Time Delta
	// Defaults to Created if Updated is effectively same as Created and never updated?
	// Actually, Updated is reliable.
	lastUpdate := deposit.GetDateTime("updated").Time()
	now := time.Now()
	deltaSeconds := now.Sub(lastUpdate).Seconds()

	if deltaSeconds < 1.0 {
		return 0, resourceId, nil // Too fast
	}

	// 2. Calculate Mining Rate
	// Sum of Employees assigned to this deposit
	employees, err := g.app.FindRecordsByFilter(
		"employees",
		fmt.Sprintf("deposit = '%s'", depositId),
		"",
		0,
		0,
	)
	if err != nil {
		return 0, resourceId, err
	}

	totalMiningPower := 0.0
	for _, emp := range employees {
		// Field is "mining" (int)
		totalMiningPower += float64(emp.GetInt("mining"))
	}

	if totalMiningPower == 0 {
		return 0, resourceId, nil
	}

	// Yield = Rate * (Delta / Interval)
	// Example: Power 4, Interval 60s. Delta 120s -> 4 * (120/60) = 8 items.
	yield := totalMiningPower * (deltaSeconds / GraphHarvestInterval)

	// 3. Check Capacity (Depletion)
	currentQty := deposit.GetFloat("quantity")
	if yield > currentQty {
		yield = currentQty
	}

	if yield <= 0 {
		return 0, resourceId, nil
	}

	// 4. Update Deposit
	// We subtract yield from the ground.
	newQty := currentQty - yield
	deposit.Set("quantity", newQty)

	// This Save() updates 'updated' timestamp, triggering the reset for next delta.
	if err := g.app.Save(deposit); err != nil {
		return 0, resourceId, err
	}

	return yield, resourceId, nil
}

func (g *GraphEconomy) processMachine(machineId string) (float64, string, error) {
	machine, err := g.app.FindRecordById("machines", machineId)
	if err != nil {
		return 0, "", err
	}

	// Get machine static data
	gamedataId := machine.GetString("machine_id")
	itemDef := gamedata.GetItem(gamedataId)
	if itemDef == nil {
		return 0, "", fmt.Errorf("unknown gamedata id: %s", gamedataId)
	}

	// Determine what it produces
	outputItem := itemDef.Product
	if outputItem == "" && itemDef.UseRecipe != "" {
		// Get output from recipe
		recipe := gamedata.GetRecipe(itemDef.UseRecipe)
		if recipe != nil {
			outputItem = recipe.OutputItem
		}
	}
	if outputItem == "" {
		return 0, "", nil // Not a producer
	}

	// Initialize lazy calculator
	lazyCalc := NewLazyCalculator(g.app)

	// === CHECK 1: DURABILITY ===
	durability := machine.GetFloat("durability")
	if durability <= 0 {
		g.app.Logger().Info("[GRAPH] Machine stopped - no durability", "machineId", machineId)
		return 0, outputItem, nil
	}

	// === CHECK 2: GLOBAL ENERGY ===
	energyPercent := lazyCalc.CalculateGlobalEnergyPercent()
	if energyPercent <= 0 {
		// Workers resting - but MAINTENANCE can work!
		g.app.Logger().Info("[GRAPH] Workers resting - checking maintenance", "machineId", machineId)

		// Apply maintenance if any maintenance employees assigned
		assignedEmployees, _ := g.app.FindRecordsByFilter("employees", fmt.Sprintf("machine = '%s'", machineId), "", 100, 0)
		totalMaintenanceSkill := 0
		for _, emp := range assignedEmployees {
			totalMaintenanceSkill += emp.GetInt("maintenance")
		}

		if totalMaintenanceSkill > 0 {
			// Calculate time since last update
			lastUpdate := machine.GetDateTime("updated").Time()
			repairedDurability := lazyCalc.CalculateMachineDurability(durability, lastUpdate, totalMaintenanceSkill)

			if repairedDurability > durability {
				machine.Set("durability", repairedDurability)
				g.app.Save(machine)
				g.app.Logger().Info("[GRAPH] Maintenance applied",
					"machineId", machineId,
					"oldDurability", durability,
					"newDurability", repairedDurability,
					"maintenanceSkill", totalMaintenanceSkill)
			}
		}

		return 0, outputItem, nil
	}

	// === CALCULATE PRODUCTION CYCLES ===
	startedAtVal := machine.GetDateTime("production_started_at")
	startedAt := startedAtVal.Time()

	if startedAt.IsZero() {
		// Initialize production timestamp
		startedAt = time.Now()
		machine.Set("production_started_at", startedAt)
		// Set initial durability if not set
		if durability == 0 {
			machine.Set("durability", float64(gamedata.MachineDurabilityOnPlace))
		}
		g.app.Save(machine)
		return 0, outputItem, nil
	}

	now := time.Now()
	delta := now.Sub(startedAt).Seconds()
	cycleTime := float64(itemDef.ProductionTime)
	if cycleTime <= 0 {
		cycleTime = float64(gamedata.DefaultHarvestCycle)
	}

	cyclesCompleted := int(math.Floor(delta / cycleTime))
	if cyclesCompleted < 1 {
		return 0, outputItem, nil
	}

	// === GET EMPLOYEES AND CALCULATE BONUSES ===
	assignedEmployees, _ := g.app.FindRecordsByFilter("employees", fmt.Sprintf("machine = '%s'", machineId), "", 100, 0)
	var totalMiningPower float64 = 0

	for _, emp := range assignedEmployees {
		totalMiningPower += float64(emp.GetInt("mining"))
	}

	// === PROCESS BASED ON MACHINE TYPE ===
	var totalProduced float64
	companyId := machine.GetString("company")

	if itemDef.UseRecipe == "" {
		// EXTRACTOR: No recipe, direct production
		baseProduction := float64(itemDef.ProductQuantity)
		if baseProduction <= 0 {
			baseProduction = 1
		}

		// Apply mining bonus: base * (1 + totalMining/10)
		miningMultiplier := 1.0
		if totalMiningPower > 0 {
			miningMultiplier = 1.0 + (totalMiningPower / 10.0)
		}

		// Apply energy efficiency
		energyMultiplier := energyPercent / 100.0

		// Max possible production based on cycles
		potentialProduction := float64(cyclesCompleted) * baseProduction * miningMultiplier * energyMultiplier

		// Check Deposit availability
		depositId := machine.GetString("deposit")
		if depositId != "" {
			deposit, err := g.app.FindRecordById("deposits", depositId)
			if err != nil {
				return 0, outputItem, err
			}

			// Verify resource match (optional but good)
			// depResource := deposit.GetString("ressource_id")
			// if depResource != outputItem { ... }

			currentQty := deposit.GetFloat("quantity")
			if potentialProduction > currentQty {
				potentialProduction = currentQty
			}

			if potentialProduction > 0 {
				deposit.Set("quantity", currentQty-potentialProduction)
				if err := g.app.Save(deposit); err != nil {
					return 0, outputItem, err
				}
				totalProduced = potentialProduction
			}
		} else {
			// If no deposit, maybe it's a generator or magic?
			// Logic dictates extractors need deposits.
			// If it has no deposit, we might assume 0 production or let it run free?
			// Given the game context, let's assume 0 if it's supposed to be on a deposit.
			// But we can check if it requires a deposit (placed check).
			// For now, let's assume if it has no recipe, it needs a deposit.
			totalProduced = 0
			g.app.Logger().Info("[GRAPH] Extractor has no deposit", "machineId", machineId)
		}

	} else {
		// FACTORY: Needs recipe inputs
		recipe := gamedata.GetRecipe(itemDef.UseRecipe)
		if recipe == nil {
			return 0, outputItem, fmt.Errorf("recipe not found: %s", itemDef.UseRecipe)
		}

		// Calculate how many cycles we can actually complete based on available inputs
		maxPossibleCycles := cyclesCompleted

		for _, input := range recipe.Inputs {
			// Check company inventory for this input
			invRecords, _ := g.app.FindRecordsByFilter(
				"inventory",
				fmt.Sprintf("company = '%s' && item_id = '%s'", companyId, input.ItemID),
				"",
				1,
				0,
			)

			availableQty := 0.0
			if len(invRecords) > 0 {
				availableQty = invRecords[0].GetFloat("quantity")
			}

			// How many cycles can this input support?
			requiredPerCycle := float64(input.Quantity)
			possibleCycles := int(availableQty / requiredPerCycle)

			if possibleCycles < maxPossibleCycles {
				maxPossibleCycles = possibleCycles
			}
		}

		if maxPossibleCycles <= 0 {
			g.app.Logger().Info("[GRAPH] Factory missing inputs", "machineId", machineId, "recipe", itemDef.UseRecipe)
			return 0, outputItem, nil
		}

		// Consume inputs for the cycles we can complete
		for _, input := range recipe.Inputs {
			totalRequired := float64(input.Quantity * maxPossibleCycles)

			invRecords, _ := g.app.FindRecordsByFilter(
				"inventory",
				fmt.Sprintf("company = '%s' && item_id = '%s'", companyId, input.ItemID),
				"",
				1,
				0,
			)

			if len(invRecords) > 0 {
				inv := invRecords[0]
				currentQty := inv.GetFloat("quantity")
				newQty := currentQty - totalRequired
				if newQty <= 0 {
					g.app.Delete(inv)
				} else {
					inv.Set("quantity", newQty)
					g.app.Save(inv)
				}
			}
		}

		cyclesCompleted = maxPossibleCycles
		outputQty := recipe.OutputQuantity
		if outputQty <= 0 {
			outputQty = 1
		}
		totalProduced = float64(cyclesCompleted * outputQty)
	}

	if totalProduced <= 0 {
		return 0, outputItem, nil
	}

	// === UPDATE DURABILITY ===
	newDurability := durability - float64(cyclesCompleted)
	if newDurability < 0 {
		newDurability = 0
	}
	machine.Set("durability", newDurability)

	// === UPDATE PRODUCTION TIMESTAMP ===
	newStart := startedAt.Add(time.Duration(float64(cyclesCompleted) * cycleTime * float64(time.Second)))
	machine.Set("production_started_at", newStart)

	// === PERSIST TO INVENTORY ===
	invRecords, _ := g.app.FindRecordsByFilter(
		"inventory",
		fmt.Sprintf("company = '%s' && item_id = '%s'", companyId, outputItem),
		"",
		1,
		0,
	)

	var invRecord *core.Record
	if len(invRecords) > 0 {
		invRecord = invRecords[0]
		invRecord.Set("quantity", invRecord.GetFloat("quantity")+totalProduced)
	} else {
		collection, _ := g.app.FindCollectionByNameOrId("inventory")
		invRecord = core.NewRecord(collection)
		invRecord.Set("company", companyId)
		invRecord.Set("item_id", outputItem)
		invRecord.Set("quantity", totalProduced)
	}

	if err := g.app.Save(invRecord); err != nil {
		return 0, outputItem, err
	}

	// Save machine state
	if err := g.app.Save(machine); err != nil {
		g.app.Logger().Error("[GRAPH] Failed to save machine state", "err", err)
	}

	g.app.Logger().Info("[GRAPH] Production complete",
		"machineId", machineId,
		"cycles", cyclesCompleted,
		"produced", totalProduced,
		"item", outputItem,
		"durability", newDurability,
	)

	return totalProduced, outputItem, nil
}
