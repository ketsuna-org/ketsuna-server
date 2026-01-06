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

	// Machine Data (Static) needs to be fetched based on `machine_id` field?
	// Wait, `machines` collection has `machine_id` which likely refers to `items.ID` (e.g. "forestry_machine").
	gamedataId := machine.GetString("machine_id")
	itemDef := gamedata.GetItem(gamedataId)
	if itemDef == nil {
		return 0, "", fmt.Errorf("unknown gamedata id: %s", gamedataId)
	}

	// Determine what it produces
	outputItem := itemDef.Product
	if outputItem == "" {
		return 0, "", nil // Not a producer
	}

	// 1. Calculate Production Cycles
	// Using `production_started_at`
	startedAtVal := machine.GetDateTime("production_started_at")
	startedAt := startedAtVal.Time()

	if startedAt.IsZero() {
		// Initialize if zero (first run)
		startedAt = time.Now()
		machine.Set("production_started_at", startedAt)
		g.app.Save(machine)
		return 0, outputItem, nil
	}

	now := time.Now()
	delta := now.Sub(startedAt).Seconds()
	cycleTime := float64(itemDef.ProductionTime)
	if cycleTime <= 0 {
		cycleTime = 1 // Safety
	}

	cyclesCompleted := math.Floor(delta / cycleTime)

	if cyclesCompleted < 1 {
		return 0, outputItem, nil
	}

	// 2. Consume Inputs (Recursive Pull)
	// Find inputs connected to this machine
	_, err = g.app.FindRecordsByFilter(
		"edge_relation",
		fmt.Sprintf("output_type = 'machine' && output_id = '%s'", machine.Id),
		"",
		0,
		0,
	)

	// Logic: We need ingredients for `cyclesCompleted` batches.
	// Can we pull enough?
	// For MVP, we assume "Try to fullfil all cycles". If input fails, we reduce cycles?
	// Or simplistic: Just pull what is available and limit cycles.
	// This "Pull" logic for consumption is complex because inputs might be other machines.
	// If Input is a Deposit, we effectively "Mine" it just-in-time for this machine cycle.

	// WARNING: Just-in-time pulling for consumption might mine the deposit.
	// If we mine 10 from deposit, but only need 5, do we void the 5?
	// Or does the machine have an input buffer?
	// The User said: "In every action... read the graph from top to bottom".
	// The User also said "Inventory of a Machine, we calculate latest action".
	// Implicitly, Machines process using buffers.
	// But `machines` schema DOES NOT have input buffers (only `stored_energy`).
	// So it's "Direct Flow" or "Virtual Buffer"?

	// Assumption for MVP Graph:
	// We maximize cycles based on Inputs Available.
	// We iterate inputs, Pull from them.
	// We verify if we have enough for 1 cycle, then N cycles.

	// Note: Recipe requirements logic is needed here (e.g. 2 Wood -> 1 Plank).
	// `itemDef.UseRecipe` gives us the ingredients.
	// But `itemDef` struct in gamedata (`items.go`) handles `UseRecipe`.
	// I need access to Recipe definitions. `gamedata/recipes.go`?
	// I haven't seen `recipes.go`.

	// Shortcut: If `itemDef.Product` is set, check if `CanConsume` or `UseRecipe` is set.
	// If no inputs required (e.g. Forestry Machine?), we just produce.
	// Forestry Machine: `Product: "wood"`. No `UseRecipe`.
	// So it generates from nothing (Manual/Labor).
	// Check employees for Labor logic?
	// `MaxEmployee`.
	// If no logic for input ingredients, we just assume Labor/Energy is satisfied or check it.

	// Let's implement simplest "No Input" Machines first (Extractors).
	if itemDef.UseRecipe == "" {
		// Extractor / Generator
		// Check Energy/Labor if we want to be strict.
		// For now, assume operation if cycles passed.

		totalProduced := cyclesCompleted * float64(itemDef.ProductQuantity)
		if totalProduced == 0 {
			// fallback if quantity not set
			totalProduced = cyclesCompleted
		}

		// Update Production Start Time
		// We advance it by cycles * cycleTime (preserving partial progress)
		newStart := startedAt.Add(time.Duration(cyclesCompleted * cycleTime * float64(time.Second)))
		machine.Set("production_started_at", newStart)

		// PERSIST INVENTORY
		// Check for existing inventory linked to this machine
		inventoryRecords, _ := g.app.FindRecordsByFilter(
			"inventory",
			fmt.Sprintf("linked_storage = '%s' && item = '%s'", machineId, outputItem),
			"",
			1,
			0,
		)

		var invRecord *core.Record
		if len(inventoryRecords) > 0 {
			invRecord = inventoryRecords[0]
			invRecord.Set("quantity", invRecord.GetFloat("quantity")+totalProduced)
		} else {
			collection, _ := g.app.FindCollectionByNameOrId("inventory")
			invRecord = core.NewRecord(collection)
			invRecord.Set("company", machine.GetString("company")) // Machine belongs to company
			invRecord.Set("item", outputItem)
			invRecord.Set("quantity", totalProduced)
			invRecord.Set("linked_storage", machineId)
		}

		if err := g.app.Save(invRecord); err != nil {
			return 0, outputItem, err
		}

		// Save machine state (timestamp)
		g.app.Save(machine)

		return totalProduced, outputItem, nil
	} else {
		// Machine NEEDS inputs (Factory).
		// TODO: Implement Recipe Pulling.
		// For now, return 0 to be safe and avoiding infinite recursion without checks.
		return 0, outputItem, nil
	}
}
