package hooks

import (
	"fmt"
	"math"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"ketsuna.com/server/internal/gamedata"
)

// GraphTraversal handles the graph-based economy traversal
type GraphTraversal struct {
	app            *pocketbase.PocketBase
	lazyCalc       *LazyCalculator
	energyBalance  *EnergyBalance
	visited        map[string]bool           // Prevent infinite loops
	inventoryCache map[string]*core.Record   // Cache inventory by item_id
	recordCache    map[string]*core.Record   // Cache fetched records by collection:id
	employeeCache  map[string][]*core.Record // Cache employees by assignment (machineId -> employees)
}

// NewGraphTraversal creates a new graph traversal instance
func NewGraphTraversal(app *pocketbase.PocketBase) *GraphTraversal {
	return &GraphTraversal{
		app:            app,
		lazyCalc:       NewLazyCalculator(app),
		visited:        make(map[string]bool),
		inventoryCache: make(map[string]*core.Record),
		recordCache:    make(map[string]*core.Record),
		employeeCache:  make(map[string][]*core.Record),
	}
}

// Helper: Get machine from cache or DB
func (gt *GraphTraversal) getMachine(id string) (*core.Record, error) {
	key := "machines:" + id
	if rec, ok := gt.recordCache[key]; ok {
		return rec, nil
	}
	rec, err := gt.app.FindRecordById("machines", id)
	if err == nil {
		gt.recordCache[key] = rec
	}
	return rec, err
}

// Helper: Get employees for machine from cache or DB
func (gt *GraphTraversal) getMachineEmployees(machineId string) ([]*core.Record, error) {
	if emps, ok := gt.employeeCache[machineId]; ok {
		return emps, nil
	}
	// Fallback to DB if not pre-cached (shouldn't happen if Traverse is called correctly)
	return gt.app.FindRecordsByFilter("employees", fmt.Sprintf("machine = '%s'", machineId), "", 0, 0)
}

// EnergyBalance represents the energy state for a company
type EnergyBalance struct {
	Available float64 // From generators + stored (buffer not included in available for consumption? )
	// Actually Available usually means current power output.
	// Stored is separate.
	Demand          float64 // From active machines
	Ratio           float64 // Available / Demand (0-1)
	StoredEnergy    float64 // Sum of all machines with stored_energy
	MaxStoredEnergy float64 // Total capacity
}

// NodeFlow represents the resource flow from a node
type NodeFlow struct {
	ItemID   string
	Quantity float64
	NodeType string
	NodeID   string
}

// FindSinks finds all downstream storage or company nodes from a given start node
func (gt *GraphTraversal) FindSinks(startNodeId string) ([]string, []string, error) {
	storageIds := make(map[string]bool)
	companyIds := make(map[string]bool)
	visited := make(map[string]bool)

	var traverse func(string)
	traverse = func(currentId string) {
		if visited[currentId] {
			return
		}
		visited[currentId] = true

		// Find outgoing edges (where input_id = currentId)
		edges, err := gt.app.FindRecordsByFilter(
			"edge_relation",
			fmt.Sprintf("input_id = '%s'", currentId),
			"", 0, 0,
		)
		if err != nil {
			return
		}

		for _, edge := range edges {
			outType := edge.GetString("output_type")
			outId := edge.GetString("output_id")

			switch outType {
			case "company":
				companyIds[outId] = true
			case "storage":
				storageIds[outId] = true
			case "machine":
				// Recurse
				traverse(outId)
			}
		}
	}

	traverse(startNodeId)

	sIds := make([]string, 0, len(storageIds))
	for id := range storageIds {
		sIds = append(sIds, id)
	}
	cIds := make([]string, 0, len(companyIds))
	for id := range companyIds {
		cIds = append(cIds, id)
	}

	return sIds, cIds, nil
}

// TraverseStorages iterates all storage nodes for the company to trigger buffering
func (gt *GraphTraversal) TraverseStorages(companyId string) error {
	// Find all storage machines for this company
	machines, err := gt.app.FindRecordsByFilter("machines",
		fmt.Sprintf("company = '%s'", companyId),
		"", 0, 0)
	if err != nil {
		return err
	}

	for _, mach := range machines {
		gamedataId := mach.GetString("machine_id")
		itemDef := gamedata.GetItem(gamedataId)
		if itemDef != nil && itemDef.Type == gamedata.ItemTypeStockage {
			// This is a storage node. Process it as a SINK (requestedBy = "").
			gt.ProcessNode(mach.Id, "storage", "")
		}
	}
	return nil
}

// TraverseFromCompany starts graph traversal from the company node
// It pulls resources from all connected nodes
func (gt *GraphTraversal) TraverseFromCompany(companyId string) (map[string]float64, error) {
	totalFlow := make(map[string]float64)
	gt.visited = make(map[string]bool)
	gt.inventoryCache = make(map[string]*core.Record)
	gt.recordCache = make(map[string]*core.Record)
	gt.employeeCache = make(map[string][]*core.Record)

	// OPIMIZATION 1: Pre-load company inventory
	invRecords, _ := gt.app.FindRecordsByFilter("inventory",
		fmt.Sprintf("company = '%s'", companyId), "", 0, 0)
	for _, rec := range invRecords {
		gt.inventoryCache[rec.GetString("item_id")] = rec
	}

	// OPTIMIZATION 2: Pre-load all machines for the company
	allMachines, err := gt.app.FindRecordsByFilter("machines",
		fmt.Sprintf("company = '%s'", companyId), "", 0, 0)
	if err == nil {
		for _, m := range allMachines {
			gt.recordCache["machines:"+m.Id] = m
		}
	}

	// OPTIMIZATION 3: Pre-load all employees for the company
	// Note: We need employees assigned to machines (employer = companyId and machine != "")
	// For simplicity, just fetch all company employees
	allEmployees, err := gt.app.FindRecordsByFilter("employees",
		fmt.Sprintf("employer = '%s'", companyId), "", 0, 0)
	if err == nil {
		for _, emp := range allEmployees {
			mId := emp.GetString("machine")
			if mId != "" {
				gt.employeeCache[mId] = append(gt.employeeCache[mId], emp)
			}
			dId := emp.GetString("deposit")
			if dId != "" {
				// We can also cache deposit employees if we want
				// Using "deposits:"+dId prefix to avoid collision? No, map key is assignment ID.
				// Since we use separate lookups for machine vs deposit, we can share map if keys are unique (UUIDs are unique).
				gt.employeeCache[dId] = append(gt.employeeCache[dId], emp)
			}
		}
	}

	// Update generators (consume fuel) - Pass pre-loaded machines
	if err := gt.UpdateGenerators(companyId, allMachines); err != nil {
		gt.app.Logger().Error("[GRAPH] Failed to update generators", "err", err)
	}

	// First, calculate energy balance for the entire company - Pass pre-loaded machines
	energyBalance, err := gt.CalculateEnergyBalance(companyId, allMachines)
	if err != nil {
		return totalFlow, err
	}
	gt.energyBalance = energyBalance

	gt.app.Logger().Info("[GRAPH] Starting company traversal",
		"companyId", companyId,
		"energyRatio", energyBalance.Ratio,
		"energyAvailable", energyBalance.Available,
		"energyDemand", energyBalance.Demand)

	// Find all edges pointing TO the company
	edges, err := gt.app.FindRecordsByFilter(
		"edge_relation",
		fmt.Sprintf("output_id = '%s' && output_type = 'company'", companyId),
		"",
		0,
		0,
	)
	if err != nil {
		return totalFlow, err
	}

	gt.app.Logger().Info("[GRAPH] Found edges to company", "count", len(edges))

	// Process each incoming edge
	for _, edge := range edges {
		inputType := edge.GetString("input_type")
		inputId := edge.GetString("input_id")

		flow, err := gt.ProcessNode(inputId, inputType, companyId)
		if err != nil {
			gt.app.Logger().Error("[GRAPH] Error processing node",
				"nodeType", inputType,
				"nodeId", inputId,
				"err", err)
			continue
		}

		if flow.Quantity > 0 && flow.ItemID != "" {
			totalFlow[flow.ItemID] += flow.Quantity
			gt.app.Logger().Info("[GRAPH] Flow from node",
				"nodeType", inputType,
				"nodeId", inputId,
				"item", flow.ItemID,
				"quantity", flow.Quantity)
		}
	}

	// Add all accumulated flows to company inventory
	if err := gt.AddFlowsToInventory(companyId, totalFlow); err != nil {
		gt.app.Logger().Error("[GRAPH] Failed to add flows to inventory", "err", err)
	}

	return totalFlow, nil
}

// ProcessNode processes a single node and returns its output
func (gt *GraphTraversal) ProcessNode(nodeId, nodeType, requestedBy string) (*NodeFlow, error) {
	visitKey := fmt.Sprintf("%s:%s", nodeType, nodeId)
	if gt.visited[visitKey] {
		// gt.app.Logger().Warn("[GRAPH] Cycle detected, skipping", "node", visitKey) // Reduce log noise
		return &NodeFlow{}, nil
	}
	gt.visited[visitKey] = true

	switch nodeType {
	case "deposit":
		return gt.ProcessDeposit(nodeId, requestedBy)
	case "machine":
		return gt.ProcessMachine(nodeId, requestedBy)
	case "storage":
		return gt.ProcessStorage(nodeId, requestedBy)
	default:
		return &NodeFlow{}, fmt.Errorf("unknown node type: %s", nodeType)
	}
}

// ProcessDeposit processes a deposit node
func (gt *GraphTraversal) ProcessDeposit(depositId, requestedBy string) (*NodeFlow, error) {
	deposit, err := gt.app.FindRecordById("deposits", depositId)
	if err != nil {
		return &NodeFlow{}, err
	}

	// Get resource type
	resourceId := deposit.GetString("ressource_id")
	if resourceId == "" {
		return &NodeFlow{}, nil
	}

	lastHarvest := deposit.GetDateTime("last_harvest_at").Time()
	if lastHarvest.IsZero() {
		lastHarvest = deposit.GetDateTime("updated").Time()
	}

	now := time.Now()
	// Minimum interval check
	if now.Sub(lastHarvest).Seconds() < 1.0 {
		return &NodeFlow{ItemID: resourceId, Quantity: 0}, nil
	}

	// Use Cache for employees
	employees, err := gt.getMachineEmployees(depositId)
	if err != nil {
		return &NodeFlow{}, err
	}

	// Calculate Total Effective Yield based on individual productivity
	weightedYield := 0.0
	// activeEmployees := 0

	for _, emp := range employees {
		// Calculate effective work seconds for this employee in the time window
		effSeconds := gt.lazyCalc.CalculateEmployeeProductivity(emp, lastHarvest, now)

		if effSeconds > 0 {
			miningSkill := float64(emp.GetInt("mining"))
			// Yield contribution per employee
			weightedYield += miningSkill * effSeconds
			// activeEmployees++
		}
	}

	if weightedYield <= 0 {
		deposit.Set("last_harvest_at", now)
		gt.app.Save(deposit)
		return &NodeFlow{ItemID: resourceId, Quantity: 0}, nil
	}

	// Yield = WeightedSkillSeconds / Interval
	yield := weightedYield / GraphHarvestInterval

	// Check deposit capacity
	currentQty := deposit.GetFloat("quantity")
	if yield > currentQty {
		yield = currentQty
	}

	yield = math.Round(yield*100) / 100

	if yield <= 0 {
		return &NodeFlow{ItemID: resourceId, Quantity: 0}, nil
	}

	currentHarvested := deposit.GetFloat("harvested")
	newHarvested := currentHarvested + yield
	newQuantity := currentQty - yield

	deposit.Set("harvested", newHarvested)
	deposit.Set("quantity", newQuantity)
	deposit.Set("last_harvest_at", now)

	if err := gt.app.Save(deposit); err != nil {
		gt.app.Logger().Error("[GRAPH] Failed to save deposit", "id", depositId, "err", err)
		return &NodeFlow{}, err
	}

	return &NodeFlow{
		ItemID:   resourceId,
		Quantity: yield,
		NodeType: "deposit",
		NodeID:   depositId,
	}, nil
}

// AddFlowsToInventory adds the accumulated flows to company inventory
func (gt *GraphTraversal) AddFlowsToInventory(companyId string, flows map[string]float64) error {
	if len(flows) == 0 {
		return nil
	}

	for itemId, quantity := range flows {
		if quantity <= 0 {
			continue
		}

		// Find existing inventory record for this item
		invRecords, err := gt.app.FindRecordsByFilter("inventory",
			fmt.Sprintf("company = '%s' && item_id = '%s'", companyId, itemId),
			"", 1, 0)

		var inv *core.Record

		if err == nil && len(invRecords) > 0 {
			// Update existing
			inv = invRecords[0]
			currentQty := inv.GetFloat("quantity")
			inv.Set("quantity", currentQty+quantity)
		} else {
			// Create new inventory record
			collection, err := gt.app.FindCollectionByNameOrId("inventory")
			if err != nil {
				gt.app.Logger().Error("[GRAPH] Failed to find inventory collection", "err", err)
				continue
			}

			inv = core.NewRecord(collection)
			inv.Set("company", companyId)
			inv.Set("item_id", itemId)
			inv.Set("quantity", quantity)
		}

		// Save inventory
		if err := gt.app.Save(inv); err != nil {
			gt.app.Logger().Error("[GRAPH] Failed to save inventory",
				"company", companyId,
				"item", itemId,
				"quantity", quantity,
				"err", err)
			continue
		}

		gt.app.Logger().Info("[GRAPH] Added to inventory",
			"company", companyId,
			"item", itemId,
			"quantity", quantity)
	}

	return nil
}

// ProcessMachine processes a machine node
func (gt *GraphTraversal) ProcessMachine(machineId, requestedBy string) (*NodeFlow, error) {
	// USE CACHE via helper
	machine, err := gt.getMachine(machineId)
	if err != nil {
		return &NodeFlow{}, err
	}

	gamedataId := machine.GetString("machine_id")
	itemDef := gamedata.GetItem(gamedataId)
	if itemDef == nil {
		return &NodeFlow{}, fmt.Errorf("unknown gamedata id: %s", gamedataId)
	}

	// Skip storage machines (handled by ProcessStorage)
	if itemDef.Type == gamedata.ItemTypeStockage {
		return gt.ProcessStorage(machineId, requestedBy)
	}

	// Determine output item
	outputItem := itemDef.Product
	if outputItem == "" && itemDef.UseRecipe != "" {
		recipe := gamedata.GetRecipe(itemDef.UseRecipe)
		if recipe != nil {
			outputItem = recipe.OutputItem
		}
	}

	// Check if this is an energy generator
	isEnergyGenerator := itemDef.ProduceEnergy > 0 && outputItem == ""

	if outputItem == "" && !isEnergyGenerator {
		return &NodeFlow{}, nil
	}

	durability := machine.GetFloat("durability")
	energyMultiplier := 1.0

	// Safety check
	if gt.energyBalance == nil {
		// Should not happen if called correctly
		gt.energyBalance = &EnergyBalance{Ratio: 1.0, Available: 1000}
	}

	if itemDef.NeedEnergy > 0 && gt.energyBalance.Ratio < 0.1 {
		return &NodeFlow{ItemID: outputItem, Quantity: 0}, nil
	}
	if itemDef.NeedEnergy > 0 {
		energyMultiplier = gt.energyBalance.Ratio
	}

	// === PRODUCTION LOGIC ===
	startedAtVal := machine.GetDateTime("production_started_at")
	startedAt := startedAtVal.Time()
	now := time.Now()

	if startedAt.IsZero() {
		machine.Set("production_started_at", now)
		if durability == 0 {
			machine.Set("durability", float64(gamedata.MachineDurabilityOnPlace))
		}
		gt.app.Save(machine)
		return &NodeFlow{ItemID: outputItem, Quantity: 0}, nil
	}

	// 1. Maintain (Repair) first - USE CACHE
	employees, err := gt.getMachineEmployees(machineId)
	if err == nil && len(employees) > 0 {
		repairedDurability := gt.lazyCalc.CalculateMachineDurability(durability, startedAt, employees)
		if repairedDurability > durability {
			durability = repairedDurability
			machine.Set("durability", durability)
			gt.app.Logger().Info("[GRAPH] Maintenance applied", "machineId", machineId, "newDurability", durability)
		}
	}

	if durability <= 0 {
		machine.Set("production_started_at", now)
		gt.app.Save(machine)
		return &NodeFlow{ItemID: outputItem, Quantity: 0}, nil
	}

	delta := now.Sub(startedAt).Seconds()
	if delta < 1.0 {
		return &NodeFlow{ItemID: outputItem, Quantity: 0}, nil
	}

	cycleTime := float64(itemDef.ProductionTime)
	if cycleTime <= 0 {
		cycleTime = float64(gamedata.DefaultHarvestCycle)
	}

	// Effective time = Delta * PowerRatio
	effectiveDelta := delta * energyMultiplier

	// Potential cycles based on time and machine power
	cyclesCompleted := int(math.Floor(effectiveDelta / cycleTime))
	if cyclesCompleted < 1 {
		return &NodeFlow{ItemID: outputItem, Quantity: 0}, nil
	}

	// Cap cycles by durability
	if float64(cyclesCompleted) > durability {
		cyclesCompleted = int(durability)
	}
	if cyclesCompleted < 1 {
		machine.Set("production_started_at", now)
		gt.app.Save(machine)
		return &NodeFlow{ItemID: outputItem, Quantity: 0}, nil
	}

	// Calculate max possible cycles based on Inputs (Recipe)
	maxPossibleCycles := cyclesCompleted

	if itemDef.UseRecipe != "" {
		recipe := gamedata.GetRecipe(itemDef.UseRecipe)
		if recipe != nil {
			// Find inputs from edges - Not perfectly cached yet, but finding edges is fast enough (indexed)
			// Optimizaion: We could cache edges too?
			incomingEdges, _ := gt.app.FindRecordsByFilter(
				"edge_relation",
				fmt.Sprintf("output_id = '%s' && output_type = 'machine'", machineId),
				"", 0, 0,
			)

			// Pull inputs
			inputsReceived := make(map[string]float64)
			for _, edge := range incomingEdges {
				inType := edge.GetString("input_type")
				inId := edge.GetString("input_id")
				flow, _ := gt.ProcessNode(inId, inType, machineId)
				if flow.Quantity > 0 {
					inputsReceived[flow.ItemID] += flow.Quantity
				}
			}

			// Determine actual cycles based on inputs
			limitCycles := cyclesCompleted
			for _, input := range recipe.Inputs {
				has := inputsReceived[input.ItemID]
				neededPerCycle := float64(input.Quantity)
				possible := int(has / neededPerCycle)
				if possible < limitCycles {
					limitCycles = possible
				}
			}
			maxPossibleCycles = limitCycles
		}
	} else if itemDef.Product != "" {
		// EXTRACTOR LOGIC: No recipe, but has a product (e.g. Iron Ore, Wood)
		// We must check if it's connected to a deposit and consume from it.
		// Enforce: Must have input (deposit) to produce.
		maxPossibleCycles = 0 // Default to 0 if no input found

		incomingEdges, _ := gt.app.FindRecordsByFilter(
			"edge_relation",
			fmt.Sprintf("output_id = '%s' && output_type = 'machine'", machineId),
			"", 0, 0,
		)

		if len(incomingEdges) > 0 {
			// It has inputs (likely a deposit)
			// We need to pull from them ensuring we can produce 'ProductQuantity' per cycle.
			inputsReceived := make(map[string]float64)
			baseQty := float64(itemDef.ProductQuantity)
			if baseQty <= 0 {
				baseQty = 1
			}

			// For extractors, we consume 1 unit of NodeFlow (Deposit Yield) to produce 1 Unit of Product?
			// Usually: Deposit yields "Iron Ore", Machine produces "Iron Ore". 1 to 1.
			// Or: Deposit yields "Raw Resource", Machine "Refines"?
			// In this game: Deposit = "iron_ore". Machine Product = "iron_ore".
			// So 1 produced = 1 consumed from deposit.

			for _, edge := range incomingEdges {
				inType := edge.GetString("input_type")
				inId := edge.GetString("input_id")
				flow, _ := gt.ProcessNode(inId, inType, machineId)
				if flow.Quantity > 0 {
					inputsReceived[flow.ItemID] += flow.Quantity
				}
			}

			// Limit cycles based on deposit yield
			// Needed per cycle = ProductQuantity
			limitCycles := cyclesCompleted

			// We only care about the item we are producing (or any input?)
			// Extractors usually only have 1 input.
			// Let's sum all inputs matching the product ID?
			// Actually, the deposit item ID matches the product ID.
			available := inputsReceived[itemDef.Product]

			// If we got nothing matching the product, maybe it's a "transformer"?
			// But here we are in "No Recipe" block.
			// Note: If inputsReceived is empty but we found edges, it means flow was 0 (empty deposit).

			neededPerCycle := baseQty
			possible := int(available / neededPerCycle)

			if possible < limitCycles {
				limitCycles = possible
			}
			maxPossibleCycles = limitCycles
		}
	}

	if maxPossibleCycles < 1 {
		machine.Set("production_started_at", now)
		gt.app.Save(machine)
		return &NodeFlow{ItemID: outputItem, Quantity: 0}, nil
	}

	cyclesCompleted = maxPossibleCycles

	// Calculate Output Quantity
	baseQty := float64(itemDef.ProductQuantity)
	if itemDef.UseRecipe != "" {
		recipe := gamedata.GetRecipe(itemDef.UseRecipe)
		if recipe != nil {
			baseQty = float64(recipe.OutputQuantity)
		}
	}
	if baseQty <= 0 {
		baseQty = 1
	}

	totalProduced := float64(cyclesCompleted) * baseQty

	// Calculate Employee Bonus
	if len(employees) > 0 {
		totalSkillSeconds := 0.0
		for _, emp := range employees {
			effSeconds := gt.lazyCalc.CalculateEmployeeProductivity(emp, startedAt, now)
			skill := float64(emp.GetInt("mining"))
			totalSkillSeconds += skill * effSeconds
		}

		usedSeconds := float64(cyclesCompleted) * cycleTime
		ratio := 1.0
		if effectiveDelta > 0 {
			ratio = usedSeconds / effectiveDelta
		}

		// Apply boost
		boostCycles := (energyMultiplier * totalSkillSeconds * ratio / 10.0) / cycleTime
		totalProduced += boostCycles * baseQty
	}

	totalProduced = math.Round(totalProduced*100) / 100

	// Update Durability
	durabilityLoss := float64(cyclesCompleted)
	machine.Set("durability", durability-durabilityLoss)

	// Update Timestamp
	timeAdvanced := (float64(cyclesCompleted) * cycleTime) / energyMultiplier
	newStart := startedAt.Add(time.Duration(timeAdvanced * float64(time.Second)))
	machine.Set("production_started_at", newStart)

	if err := gt.app.Save(machine); err != nil {
		return &NodeFlow{}, err
	}

	gt.app.Logger().Info("[GRAPH] Machine produced",
		"machineId", machineId,
		"item", outputItem,
		"qty", totalProduced,
		"cycles", cyclesCompleted,
	)

	return &NodeFlow{
		ItemID:   outputItem,
		Quantity: totalProduced,
		NodeType: "machine",
		NodeID:   machineId,
	}, nil
}

// ProcessStorage processes a storage node
// Storages are special machines or inventory points.
// We expect them to act as buffers: Pull Input -> Store -> Push Output.
func (gt *GraphTraversal) ProcessStorage(storageId, requestedBy string) (*NodeFlow, error) {
	// 1. Pull from inputs into storage inv
	incomingEdges, _ := gt.app.FindRecordsByFilter(
		"edge_relation",
		fmt.Sprintf("output_id = '%s' && output_type = 'storage'", storageId),
		"", 0, 0,
	)

	pulled := make(map[string]float64)
	for _, edge := range incomingEdges {
		inType := edge.GetString("input_type")
		inId := edge.GetString("input_id")
		flow, _ := gt.ProcessNode(inId, inType, storageId)
		if flow.Quantity > 0 {
			pulled[flow.ItemID] += flow.Quantity
		}
	}

	// 2. Add pulled to inventory
	// Find inventory record linked to this storage
	invRecords, _ := gt.app.FindRecordsByFilter(
		"inventory",
		fmt.Sprintf("linked_storage = '%s'", storageId),
		"", 1, 0,
	)

	var inv *core.Record
	if len(invRecords) > 0 {
		inv = invRecords[0]
	} else {
		// If we have stuff to store, create record
		if len(pulled) > 0 {
			// Find company from machine
			mach, _ := gt.app.FindRecordById("machines", storageId)
			companyId := ""
			if mach != nil {
				companyId = mach.GetString("company")
			}

			collection, _ := gt.app.FindCollectionByNameOrId("inventory")
			inv = core.NewRecord(collection)
			inv.Set("company", companyId)
			inv.Set("linked_storage", storageId)
			// Assuming single item type storage for now or picking first
			for id := range pulled {
				inv.Set("item_id", id)
				break
			}
		}
	}

	if inv != nil {
		// Update Quantity
		itemId := inv.GetString("item_id")
		qty := inv.GetFloat("quantity")

		if amt, ok := pulled[itemId]; ok {
			qty += amt
			inv.Set("quantity", qty)
			gt.app.Save(inv)
		}

		// Return available quantity
		// ONLY drain if requestedBy is set (Pulled by Machine or Company)
		// If requestedBy is empty, we are in SINK mode (just updating buffering), so we keep it.
		if requestedBy != "" {
			total := inv.GetFloat("quantity")
			if total > 0 {
				inv.Set("quantity", 0)
				gt.app.Save(inv)
				return &NodeFlow{ItemID: itemId, Quantity: total, NodeType: "storage", NodeID: storageId}, nil
			}
		} else {
			// SINK MODE: We just accumulated.
			// Return 0 flow so we don't duplicate it upstream if we were somehow called?
			// Actually if requestedBy is empty, the return value is likely ignored or logged.
			// But for safety, return 0.
			return &NodeFlow{ItemID: itemId, Quantity: 0, NodeType: "storage", NodeID: storageId}, nil
		}
	}

	return &NodeFlow{}, nil
}

// checkFuelAvailable checks if company has fuel for generator using cache
func (gt *GraphTraversal) checkFuelAvailable(_ string, fuelItems []string) bool {
	for _, item := range fuelItems {
		if rec, ok := gt.inventoryCache[item]; ok {
			if rec.GetFloat("quantity") > 0 {
				return true
			}
		}
	}
	return false
}

// CalculateEnergyBalance calculates the company's energy state
// machines argument is optional optimization to avoid refetching
func (gt *GraphTraversal) CalculateEnergyBalance(companyId string, optMachines []*core.Record) (*EnergyBalance, error) {
	balance := &EnergyBalance{
		Ratio: 1.0,
	}

	var machines []*core.Record
	var err error

	if optMachines != nil {
		machines = optMachines
	} else {
		machines, err = gt.app.FindRecordsByFilter(
			"machines",
			fmt.Sprintf("company = '%s'", companyId),
			"",
			0,
			0,
		)
		if err != nil {
			return balance, err
		}
	}

	for _, machine := range machines {
		itemDef := gamedata.GetItem(machine.GetString("machine_id"))
		if itemDef == nil {
			continue
		}

		// Energy Production
		if itemDef.ProduceEnergy > 0 {
			canProduce := true
			// Check if it needs fuel
			if len(itemDef.CanConsume) > 0 {
				if !gt.checkFuelAvailable(companyId, itemDef.CanConsume) {
					canProduce = false
				}
				// Note: Actual fuel consumption happens in UpdateGenerators
			}

			if canProduce {
				if itemDef.EnergyType == gamedata.EnergyTypeSoleil {
					if IsSolarProductionActive() {
						balance.Available += itemDef.ProduceEnergy
					}
				} else {
					balance.Available += itemDef.ProduceEnergy
				}
			}
		}

		// Energy Storage
		if itemDef.CanStoreEnergy > 0 {
			balance.MaxStoredEnergy += itemDef.CanStoreEnergy
			balance.StoredEnergy += machine.GetFloat("stored_energy")
		}

		// Energy Consumption
		if itemDef.NeedEnergy > 0 {
			balance.Demand += itemDef.NeedEnergy
		}
	}

	// Calculate ratio
	totalAvailable := balance.Available + balance.StoredEnergy
	if balance.Demand > 0 {
		balance.Ratio = totalAvailable / balance.Demand
		if balance.Ratio > 1.0 {
			balance.Ratio = 1.0
		}
	}

	return balance, nil
}

// IsSolarProductionActive checks if solar panels are producing
func IsSolarProductionActive() bool {
	hour := time.Now().UTC().Hour()
	return hour >= 8 && hour < 19
}

// UpdateGenerators processes all energy generators, consuming fuel and updating state
func (gt *GraphTraversal) UpdateGenerators(companyId string, optMachines []*core.Record) error {
	var machines []*core.Record
	var err error

	if optMachines != nil {
		machines = optMachines
	} else {
		machines, err = gt.app.FindRecordsByFilter(
			"machines",
			fmt.Sprintf("company = '%s'", companyId),
			"", 0, 0,
		)
		if err != nil {
			return err
		}
	}

	for _, machine := range machines {
		itemDef := gamedata.GetItem(machine.GetString("machine_id"))
		if itemDef == nil || itemDef.ProduceEnergy <= 0 {
			continue // Not a generator
		}

		if len(itemDef.CanConsume) == 0 {
			continue
		}

		startedAtVal := machine.GetDateTime("production_started_at")
		startedAt := startedAtVal.Time()
		if startedAt.IsZero() {
			startedAt = time.Now()
			machine.Set("production_started_at", startedAt)
			gt.app.Save(machine)
			continue
		}

		delta := time.Since(startedAt).Seconds()
		cycleTime := float64(itemDef.ProductionTime)
		if cycleTime <= 0 {
			cycleTime = 60.0
		}

		cyclesCompleted := int(math.Floor(delta / cycleTime))
		if cyclesCompleted < 1 {
			continue
		}

		fuelConsumed := true
		for _, fuelItem := range itemDef.CanConsume {
			fuelNeeded := float64(cyclesCompleted)
			invRecords, _ := gt.app.FindRecordsByFilter("inventory",
				fmt.Sprintf("company='%s' && item_id='%s'", companyId, fuelItem),
				"", 1, 0)

			if len(invRecords) > 0 {
				inv := invRecords[0]
				qty := inv.GetFloat("quantity")
				if qty >= fuelNeeded {
					inv.Set("quantity", qty-fuelNeeded)
					gt.app.Save(inv)
				} else {
					fuelConsumed = false
					possible := int(qty)
					if possible > 0 {
						inv.Set("quantity", 0)
						gt.app.Save(inv)
						cyclesCompleted = possible
					} else {
						cyclesCompleted = 0
					}
				}
			} else {
				fuelConsumed = false
				cyclesCompleted = 0
			}
		}

		if cyclesCompleted > 0 {
			timeAdvanced := float64(cyclesCompleted) * cycleTime
			newStart := startedAt.Add(time.Duration(timeAdvanced * float64(time.Second)))
			machine.Set("production_started_at", newStart)
			gt.app.Save(machine)
		} else {
			if !fuelConsumed {
				machine.Set("production_started_at", time.Now())
				gt.app.Save(machine)
			}
		}
	}
	return nil
}
