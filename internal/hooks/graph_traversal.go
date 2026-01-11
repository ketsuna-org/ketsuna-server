package hooks

import (
	"fmt"
	"math"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"ketsuna.com/server/internal/gamedata"
)

// EnergyBalance represents the energy state for a company
type EnergyBalance struct {
	Available       float64 // From generators + stored
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

type GraphTraversal struct {
	app            *pocketbase.PocketBase
	lazyCalc       *LazyCalculator
	energyBalance  *EnergyBalance
	visited        map[string]bool
	inventoryCache map[string]*core.Record
	recordCache    map[string]*core.Record
	employeeCache  map[string][]*core.Record
	edgeCache      map[string][]*core.Record // Cache: output_id -> incoming edges
}

func NewGraphTraversal(app *pocketbase.PocketBase) *GraphTraversal {
	return &GraphTraversal{
		app:            app,
		lazyCalc:       NewLazyCalculator(app),
		visited:        make(map[string]bool),
		inventoryCache: make(map[string]*core.Record),
		recordCache:    make(map[string]*core.Record),
		employeeCache:  make(map[string][]*core.Record),
		edgeCache:      make(map[string][]*core.Record),
	}
}

// --- CORE TRAVERSAL ---

// TraverseGlobal performs a full recursive flow calculation starting from the company (Pull-based)
func (gt *GraphTraversal) TraverseGlobal(companyId string) (map[string]float64, error) {
	gt.visited = make(map[string]bool)
	gt.inventoryCache = make(map[string]*core.Record)
	gt.recordCache = make(map[string]*core.Record)
	gt.employeeCache = make(map[string][]*core.Record)
	gt.edgeCache = make(map[string][]*core.Record)

	// 1. Pre-loading (Optimisation)
	gt.preloadData(companyId)

	// 2. Energy Calculation
	// Note: We need all machines for energy
	allMachines := make([]*core.Record, 0)
	for _, rec := range gt.recordCache {
		if rec.Collection().Name == "machines" {
			allMachines = append(allMachines, rec)
		}
	}
	// If cache empty, fetch them (fallback)
	if len(allMachines) == 0 {
		allMachines, _ = gt.app.FindRecordsByFilter("machines", fmt.Sprintf("company = '%s'", companyId), "", 0, 0)
	}

	gt.UpdateGenerators(companyId, allMachines)
	balance, _ := gt.CalculateEnergyBalance(companyId, allMachines)
	gt.energyBalance = balance

	// 3. Recursive Pull from Company Inputs
	totalFlow := make(map[string]float64)
	edgesToCompany := gt.edgeCache[companyId]

	for _, edge := range edgesToCompany {
		if edge.GetString("output_type") == "company" {
			// Recursive call
			flow, _ := gt.processNodeRecursive(edge.GetString("input_id"), edge.GetString("input_type"), companyId, true)
			if flow.Quantity > 0 {
				totalFlow[flow.ItemID] += flow.Quantity
			}
		}
	}

	// 4. Persistence
	gt.AddFlowsToInventory(companyId, totalFlow)
	return totalFlow, nil
}

// TraverseTarget performs a local update for a specific node (Targeted Mode)
// It checks immediate inputs for availability but does NOT recurse further up.
func (gt *GraphTraversal) TraverseTarget(nodeId, nodeType string) (*NodeFlow, error) {
	// Initialize minimal cache for local context
	gt.visited = make(map[string]bool)
	gt.edgeCache = make(map[string][]*core.Record)

	// Pre-fetch edges for this node only? Or standard pre-load?
	// For responsiveness, we might want to query only what's needed.
	// But simplest is to fetch edges for this node.

	// Fetch incoming edges for this node
	incomingEdges, _ := gt.app.FindRecordsByFilter("edge_relation", fmt.Sprintf("output_id='%s'", nodeId), "", 0, 0)
	gt.edgeCache[nodeId] = incomingEdges

	// We also need outgoing edges to know requirements if we were doing push,
	// but here we are doing a local Pull or Check.

	// For TraverseTarget, we assume we are "pulling" from this node to see what it *would* output.
	// So we call processNodeRecursive with recursive=false
	return gt.processNodeRecursive(nodeId, nodeType, "USER_INTERACTION", false)
}

func (gt *GraphTraversal) preloadData(companyId string) {
	invRecords, _ := gt.app.FindRecordsByFilter("inventory", fmt.Sprintf("company = '%s'", companyId), "", 0, 0)
	for _, rec := range invRecords {
		gt.inventoryCache[rec.GetString("item_id")] = rec
	}

	// Load machines first
	allMachines, _ := gt.app.FindRecordsByFilter("machines", fmt.Sprintf("company = '%s'", companyId), "", 0, 0)
	var machineIds []interface{}
	for _, m := range allMachines {
		gt.recordCache["machines:"+m.Id] = m
		machineIds = append(machineIds, m.Id)
	}

	// Load nodes edges
	// We only need edges where the destination is one of our machines (or the company)
	// OR if the input is our deposit? (But deposit points to machine, so output_id = machine)
	// So filtering by output_id IN (machines) covers the "Input -> Machine" links.
	// Filtering by input_id IN (machines) covers "Machine -> Output" links.

	// Optimization: Fetch all edges for now (limit 1000).
	// Using empty string "" for filter to match all.
	allEdges, err := gt.app.FindRecordsByFilter("edge_relation", "", "", 1000, 0)
	if err != nil {
		gt.app.Logger().Error("[GRAPH] Failed to load edges", "err", err)
	}

	gt.app.Logger().Info("[GRAPH] Preload Edges", "count", len(allEdges))

	for _, edge := range allEdges {
		outId := edge.GetString("output_id")
		gt.edgeCache[outId] = append(gt.edgeCache[outId], edge)

		// DEBUG: specific check for our machine
		if outId == "eh9s9x7ouewqfsc" {
			gt.app.Logger().Info("[GRAPH] Found edge for target machine", "edgeId", edge.Id, "inputId", edge.GetString("input_id"))
		}
	}
}

// processNodeRecursive is the core logic engine
func (gt *GraphTraversal) processNodeRecursive(nodeId, nodeType, requestedBy string, recursive bool) (*NodeFlow, error) {
	// Loop detection / Visited check
	// Storage nodes can be queried multiple times (they're buffers)
	// But production nodes (machines, deposits) should only produce once per traversal
	visitKey := fmt.Sprintf("%s:%s", nodeType, nodeId)

	// For storage: always allow entry, but track if we already pulled from upstream
	// For other nodes: standard visited check prevents re-processing
	if nodeType != "storage" {
		if gt.visited[visitKey] {
			return &NodeFlow{}, nil
		}
		gt.visited[visitKey] = true
	}

	switch nodeType {
	case "deposit":
		// Deposits are always leaves in terms of resource flow, they don't consume items.
		return gt.ProcessDeposit(nodeId, requestedBy, recursive)
	case "machine":
		return gt.ProcessMachine(nodeId, requestedBy, recursive)
	case "storage":
		return gt.ProcessStorage(nodeId, requestedBy, recursive)
	}
	return &NodeFlow{}, nil
}

// --- NODE HANDLERS ---

func (gt *GraphTraversal) ProcessMachine(machineId, requestedBy string, recursive bool) (*NodeFlow, error) {
	gt.app.Logger().Info("[GRAPH] ProcessMachine called", "machine", machineId, "requestedBy", requestedBy, "recursive", recursive)

	machine, err := gt.getMachine(machineId)
	if err != nil || !machine.GetBool("placed") {
		gt.app.Logger().Info("[GRAPH] ProcessMachine: Not placed or error", "machine", machineId, "err", err)
		return &NodeFlow{}, err
	}

	itemDef := gamedata.GetItem(machine.GetString("machine_id"))
	if itemDef == nil || (itemDef.ProduceEnergy > 0 && itemDef.Product == "") {
		gt.app.Logger().Info("[GRAPH] ProcessMachine: No itemDef or energy-only", "machine", machineId)
		return &NodeFlow{}, nil
	}

	// Determine Active Recipe: machine.active_recipe takes priority, fallback to itemDef.UseRecipe
	activeRecipeId := machine.GetString("active_recipe")
	if activeRecipeId == "" {
		activeRecipeId = itemDef.UseRecipe
	}

	// Determine Output Item
	outputItem := itemDef.Product
	if outputItem == "" && activeRecipeId != "" {
		if r := gamedata.GetRecipe(activeRecipeId); r != nil {
			outputItem = r.OutputItem
		}
	}
	if outputItem == "" {
		gt.app.Logger().Info("[GRAPH] ProcessMachine: No output item", "machine", machineId)
		return &NodeFlow{}, nil
	}

	// 1. Check Energy (Global constraint)
	energyMult := 1.0
	// Only apply energy constraints if we have an EnergyBalance (which might not be calculated in TargetMode)
	if gt.energyBalance != nil && itemDef.NeedEnergy > 0 {
		if gt.energyBalance.Ratio < 0.1 {
			gt.app.Logger().Info("[GRAPH] ProcessMachine: Low energy ratio", "machine", machineId, "ratio", gt.energyBalance.Ratio)
			return &NodeFlow{ItemID: outputItem}, nil
		}
		energyMult = gt.energyBalance.Ratio
	}

	// 2. Calculate Theoretical Cycles (Time-based)
	startedAt := machine.GetDateTime("production_started_at").Time()
	gt.app.Logger().Info("[GRAPH] ProcessMachine: Production timer", "machine", machineId, "startedAt", startedAt, "isZero", startedAt.IsZero())

	if startedAt.IsZero() {
		if recursive { // Only auto-start in global mode? Or always? Let's say always for consistency.
			machine.Set("production_started_at", time.Now())
			gt.app.Save(machine)
			gt.app.Logger().Info("[GRAPH] ProcessMachine: Started production timer", "machine", machineId)
		}
		return &NodeFlow{ItemID: outputItem}, nil
	}

	delta := time.Since(startedAt).Seconds()
	cycleTime := float64(itemDef.ProductionTime)
	if cycleTime <= 0 {
		cycleTime = 60
	}

	effectiveDelta := delta * energyMult
	timeBasedCycles := int(math.Floor(effectiveDelta / cycleTime))
	gt.app.Logger().Info("[GRAPH] ProcessMachine: Cycle calculation", "machine", machineId, "delta", delta, "cycleTime", cycleTime, "timeBasedCycles", timeBasedCycles)

	if timeBasedCycles < 1 {
		gt.app.Logger().Info("[GRAPH] ProcessMachine: Not enough time for a cycle", "machine", machineId)
		return &NodeFlow{ItemID: outputItem}, nil
	}

	// 3. Check Inputs (The "Pull")
	inputsReceived := make(map[string]float64)

	if recursive {
		// Traverse up to get inputs
		for _, edge := range gt.edgeCache[machineId] {
			flow, _ := gt.processNodeRecursive(edge.GetString("input_id"), edge.GetString("input_type"), machineId, true)
			if flow.Quantity > 0 {
				inputsReceived[flow.ItemID] += flow.Quantity
			}
		}
	} else {
		// "Targeted" mode: Check immediate availability.
		// For Storage nodes: Check their current 'quantity' in DB/Cache (Snapshot)
		// For Machine nodes: Harder. We might skip them or assume they have produced 0 for this 'tick' if not recursing.
		// Specification says: "Mets à jour uniquement les entrées (inputs) directement adjacentes." aka "Calcul local".
		for _, edge := range gt.edgeCache[machineId] {
			// If input is Storage, we can peek at its inventory
			if edge.GetString("input_type") == "storage" {
				// We need to fetch the storage's inventory record
				// Optimization: processNodeRecursive with recursive=false checks the storage snapshot.

				flow, _ := gt.processNodeRecursive(edge.GetString("input_id"), edge.GetString("input_type"), machineId, false)
				if flow.Quantity > 0 {
					inputsReceived[flow.ItemID] += flow.Quantity
				}
			}
			// If input is Machine, without recursion, we get 0 from it (it hasn't pushed anything).
		}
	}

	// 4. Calculate Real Cycles (Resource-based)
	maxCycles := timeBasedCycles

	// Cap at reasonable max per tick to prevent resource hogging
	// This ensures fair sharing between multiple machines pulling from same storage
	const maxCyclesPerTick = 3
	if maxCycles > maxCyclesPerTick {
		maxCycles = maxCyclesPerTick
		gt.app.Logger().Info("[GRAPH] ProcessMachine: Capped cycles", "machine", machineId, "timeBasedCycles", timeBasedCycles, "capped", maxCyclesPerTick)
	}

	if activeRecipeId != "" {
		recipe := gamedata.GetRecipe(activeRecipeId)
		if recipe != nil {
			for _, input := range recipe.Inputs {
				possible := int(inputsReceived[input.ItemID] / float64(input.Quantity))
				if possible < maxCycles {
					maxCycles = possible
				}
			}
		}
	} else {
		// Extractor (1:1 with deposit)
		// Note: inputsReceived contains flow from Deposit
		// For Forestry Machine, outputItem is "wood".
		// inputsReceived["wood"] comes from the Deposit processing.

		available := inputsReceived[outputItem]
		possible := int(available / 1.0)

		if possible < maxCycles {
			maxCycles = possible
		}
	}

	if maxCycles < 1 {
		gt.app.Logger().Info("[GRAPH] ProcessMachine: No cycles possible", "machine", machineId, "inputs", inputsReceived)
		return &NodeFlow{ItemID: outputItem}, nil
	}

	// 5. Commit Production & Consume Inputs (Active Extraction)
	qtyPerCycle := float64(itemDef.ProductQuantity)
	if qtyPerCycle <= 0 {
		qtyPerCycle = 1
	}
	totalProduced := math.Round(float64(maxCycles)*qtyPerCycle*100) / 100

	if recursive {
		// Update timers only (Durability removed)
		timeAdvanced := (float64(maxCycles) * cycleTime) / energyMult
		machine.Set("production_started_at", startedAt.Add(time.Duration(timeAdvanced*float64(time.Second))))
		gt.app.Save(machine)

		// CONSUME INPUTS FROM SOURCES (Deposits, Storage)
		for _, edge := range gt.edgeCache[machineId] {
			inputType := edge.GetString("input_type")
			inputId := edge.GetString("input_id")

			switch inputType {
			case "deposit":
				// Consume from Deposit (for extractors)
				consumed := totalProduced // 1:1 extraction ratio

				deposit, err := gt.app.FindRecordById("deposits", inputId)
				if err == nil {
					curQty := deposit.GetFloat("quantity")
					newQty := curQty - consumed
					if newQty < 0 {
						newQty = 0
					}

					deposit.Set("quantity", math.Round(newQty))
					gt.app.Save(deposit)

					// Record consumption statistic
					gt.recordStatistic(machine.GetString("company"), deposit.GetString("ressource_id"), "consumption", consumed)

					gt.app.Logger().Info("[GRAPH] Machine Consumed from Deposit", "machine", machineId, "deposit", inputId, "consumed", consumed, "left", newQty)
				}
			case "storage":
				// Consume from Storage (for processing machines)
				consumed := 0.0

				if activeRecipeId != "" {
					// Recipe-based consumption: consume based on recipe inputs * cycles
					recipe := gamedata.GetRecipe(activeRecipeId)
					if recipe != nil {
						for _, input := range recipe.Inputs {
							consumed += float64(input.Quantity) * float64(maxCycles)
						}
					}
				} else {
					// Non-recipe: consume totalProduced (1:1 ratio)
					consumed = totalProduced
				}

				// Find the storage's linked inventory and deduct
				invRecords, err := gt.app.FindRecordsByFilter("inventory", fmt.Sprintf("linked_storage = '%s'", inputId), "", 1, 0)
				if err == nil && len(invRecords) > 0 {
					inv := invRecords[0]
					curQty := inv.GetFloat("quantity")
					newQty := curQty - consumed
					if newQty < 0 {
						newQty = 0
					}

					inv.Set("quantity", newQty)
					gt.app.Save(inv)

					// Record consumption statistic
					gt.recordStatistic(machine.GetString("company"), inv.GetString("item_id"), "consumption", consumed)

					gt.app.Logger().Info("[GRAPH] Machine Consumed from Storage", "machine", machineId, "storage", inputId, "consumed", consumed, "left", newQty)
				}
			}
		}

		// Record production statistic
		if totalProduced > 0 {
			gt.recordStatistic(machine.GetString("company"), outputItem, "production", totalProduced)
		}
	}

	return &NodeFlow{ItemID: outputItem, Quantity: totalProduced, NodeType: "machine", NodeID: machineId}, nil
}

func (gt *GraphTraversal) ProcessStorage(storageId, requestedBy string, recursive bool) (*NodeFlow, error) {
	gt.app.Logger().Info("[GRAPH] ProcessStorage called", "storage", storageId, "requestedBy", requestedBy, "recursive", recursive)

	// Get storage machine to find company
	storageMachine, err := gt.getMachine(storageId)
	if err != nil {
		gt.app.Logger().Error("[GRAPH] ProcessStorage: Storage not found", "storage", storageId)
		return &NodeFlow{}, nil
	}
	companyId := storageMachine.GetString("company")

	// Track if we already pulled from upstream this traversal
	visitKey := fmt.Sprintf("storage:%s", storageId)
	alreadyPulled := gt.visited[visitKey]

	// Only pull from upstream on FIRST call (subsequent calls just report availability)
	if recursive && !alreadyPulled {
		gt.visited[visitKey] = true // Mark as pulled

		edges := gt.edgeCache[storageId]
		gt.app.Logger().Info("[GRAPH] ProcessStorage: Pulling from upstream", "storage", storageId, "edgeCount", len(edges))

		for _, edge := range edges {
			inputId := edge.GetString("input_id")
			inputType := edge.GetString("input_type")
			gt.app.Logger().Info("[GRAPH] ProcessStorage: Processing edge", "inputId", inputId, "inputType", inputType)

			flow, err := gt.processNodeRecursive(inputId, inputType, storageId, true)
			if err != nil {
				gt.app.Logger().Error("[GRAPH] ProcessStorage: Error from input", "inputId", inputId, "err", err)
				continue
			}
			gt.app.Logger().Info("[GRAPH] ProcessStorage: Got flow from input", "inputId", inputId, "flowItem", flow.ItemID, "flowQty", flow.Quantity)

			if flow.Quantity > 0 && flow.ItemID != "" {
				// Find or create linked inventory and add the incoming flow
				inv, err := gt.findOrCreateLinkedInventory(companyId, storageId, flow.ItemID)
				if err != nil {
					gt.app.Logger().Error("[GRAPH] ProcessStorage: Failed to find/create inventory", "err", err)
					continue
				}

				newQty := inv.GetFloat("quantity") + flow.Quantity
				inv.Set("quantity", newQty)
				gt.app.Save(inv)
				gt.app.Logger().Info("[GRAPH] Storage received input", "storage", storageId, "item", flow.ItemID, "added", flow.Quantity, "total", newQty)
			}
		}
	} else if alreadyPulled {
		gt.app.Logger().Info("[GRAPH] ProcessStorage: Already pulled, just reporting", "storage", storageId)
	}

	// Find linked inventory for serving requests (use the first one found)
	var inv *core.Record
	invRecords, _ := gt.app.FindRecordsByFilter("inventory", fmt.Sprintf("linked_storage = '%s'", storageId), "", 1, 0)
	if len(invRecords) == 0 {
		gt.app.Logger().Info("[GRAPH] ProcessStorage: No linked inventory to serve", "storage", storageId)
		return &NodeFlow{}, nil
	}
	inv = invRecords[0]
	itemId := inv.GetString("item_id")

	// Report how much is available (but DON'T consume here - let the machine consume what it needs)
	currentQty := inv.GetFloat("quantity")
	gt.app.Logger().Info("[GRAPH] ProcessStorage: Reporting availability", "storage", storageId, "item", itemId, "available", currentQty)

	// Return available quantity - the calling machine will consume what it needs
	return &NodeFlow{ItemID: itemId, Quantity: currentQty, NodeType: "storage", NodeID: storageId}, nil
}

// --- HELPERS ---

func (gt *GraphTraversal) AddFlowsToInventory(companyId string, flows map[string]float64) error {
	for itemId, quantity := range flows {
		if quantity <= 0 || itemId == "" {
			continue
		}

		inv, ok := gt.inventoryCache[itemId]
		if !ok {
			collection, _ := gt.app.FindCollectionByNameOrId("inventory")
			inv = core.NewRecord(collection)
			inv.Set("company", companyId)
			inv.Set("item_id", itemId)
			inv.Set("quantity", 0)
			gt.inventoryCache[itemId] = inv
		}

		inv.Set("quantity", inv.GetFloat("quantity")+quantity)
		gt.app.Save(inv)
	}
	return nil
}

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

// findOrCreateLinkedInventory finds or creates an inventory record linked to a storage
func (gt *GraphTraversal) findOrCreateLinkedInventory(companyId, storageId, itemId string) (*core.Record, error) {
	// Try to find existing inventory with this storage and item
	filter := fmt.Sprintf("linked_storage = '%s' && item_id = '%s'", storageId, itemId)
	invRecords, err := gt.app.FindRecordsByFilter("inventory", filter, "", 1, 0)
	if err == nil && len(invRecords) > 0 {
		return invRecords[0], nil
	}

	// Create new inventory record
	collection, err := gt.app.FindCollectionByNameOrId("inventory")
	if err != nil {
		return nil, fmt.Errorf("inventory collection not found: %w", err)
	}

	inv := core.NewRecord(collection)
	inv.Set("company", companyId)
	inv.Set("item_id", itemId)
	inv.Set("quantity", 0)
	inv.Set("linked_storage", storageId)

	if err := gt.app.Save(inv); err != nil {
		return nil, fmt.Errorf("failed to create linked inventory: %w", err)
	}

	gt.app.Logger().Info("[GRAPH] Created linked inventory for storage", "storage", storageId, "item", itemId, "company", companyId)
	return inv, nil
}

// (Conserver les fonctions UpdateGenerators, CalculateEnergyBalance et ProcessDeposit de ton code original ici)

// ProcessDeposit processes a deposit node
func (gt *GraphTraversal) ProcessDeposit(depositId, requestedBy string, recursive bool) (*NodeFlow, error) {
	deposit, err := gt.app.FindRecordById("deposits", depositId)
	if err != nil {
		gt.app.Logger().Error("[GRAPH] ProcessDeposit: Deposit not found", "id", depositId)
		return &NodeFlow{}, err
	}

	// Get resource type
	resourceId := deposit.GetString("ressource_id")
	if resourceId == "" {
		return &NodeFlow{}, nil
	}

	// NOTE: Only mutate in recursive mode
	if !recursive {
		// Just estimate based on potential
		// Simplified: return 0 or theoretical max?
		// For availability check, we can return 1 if employees are present.
		return &NodeFlow{ItemID: resourceId, Quantity: 0}, nil
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
		gt.app.Logger().Error("[GRAPH] ProcessDeposit: Failed to get employees", "id", depositId, "err", err)
		return &NodeFlow{}, err
	}

	// PASSIVE MINING LOGIC (Employees on Deposit)
	if len(employees) > 0 {
		// Calculate Total Effective Yield based on individual productivity
		weightedYield := 0.0

		for _, emp := range employees {
			// Calculate effective work seconds for this employee in the time window
			effSeconds := gt.lazyCalc.CalculateEmployeeProductivity(emp, lastHarvest, now)
			if effSeconds > 0 {
				miningSkill := float64(emp.GetInt("mining"))
				weightedYield += miningSkill * effSeconds
			}
		}

		if weightedYield > 0 {
			// Yield = WeightedSkillSeconds / Interval
			yield := weightedYield / GraphHarvestInterval

			// Check deposit capacity
			currentQty := deposit.GetFloat("quantity")
			if yield > currentQty {
				yield = currentQty
			}

			// FORCE INTEGER
			yield = math.Floor(yield)

			if yield > 0 {
				currentHarvested := deposit.GetFloat("harvested")
				newHarvested := currentHarvested + yield
				newQuantity := currentQty - yield

				// Ensure we are saving integer values
				deposit.Set("harvested", math.Round(newHarvested))
				deposit.Set("quantity", math.Round(newQuantity))
				deposit.Set("last_harvest_at", now)

				if err := gt.app.Save(deposit); err != nil {
					gt.app.Logger().Error("[GRAPH] Failed to save deposit", "id", depositId, "err", err)
				} else {
					// gt.app.Logger().Info("[GRAPH] Deposit Passive Mine", "id", depositId, "yield", yield, "remaining", newQuantity)
				}
			} else {
				// No yield but time passed, update timer
				deposit.Set("last_harvest_at", now)
				gt.app.Save(deposit)
			}
		} else {
			// Update timer even if no effective work (e.g. paused/resting)
			deposit.Set("last_harvest_at", now)
			gt.app.Save(deposit)
		}
	} else {
		// No employees: do not update last_harvest_at?
		// Or do we? If we don't, next time they arrive they get "instant credit"?
		// Strategy: Update it to "now" so they don't get free work.
		// UNLESS the game design wants "offline progress" for unassigned periods? Unlikely.
		// For simplicity/safety vs time jumps: Just update it.
		// But earlier I said "don't update".
		// Re-thinking: If no one is there, the resource sits there. Time passes.
		// If I assign someone NOW, they start working FROM NOW.
		// So `last_harvest_at` should be NOW.

		// If I don't update it, and `last_harvest` was 1 hour ago.
		// I assign someone. Next tick (1 sec later).
		// Calc: now - last_harvest = 1 hour + 1 sec.
		// Result: HUGE yield instantly.
		// user request: "une 'itération' d'employé reste sur le Dépot".
		// Implies we should prevent this exploit.
		// FIX: Update timestamp even if no employees.
		deposit.Set("last_harvest_at", now)
		gt.app.Save(deposit)
	}

	// RETURN AVAILABLE QUANTITY
	// Machines need to know how much is LEFT to extract.
	remainingQty := deposit.GetFloat("quantity")

	return &NodeFlow{
		ItemID:   resourceId,
		Quantity: remainingQty, // This is what's available for extraction
		NodeType: "deposit",
		NodeID:   depositId,
	}, nil
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
				balance.Available += itemDef.ProduceEnergy
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

// getMachineEmployees retrieves employees assigned to a machine from cache or DB
func (gt *GraphTraversal) getMachineEmployees(machineId string) ([]*core.Record, error) {
	if emps, ok := gt.employeeCache[machineId]; ok {
		return emps, nil
	}
	return gt.app.FindRecordsByFilter("employees", fmt.Sprintf("machine = '%s'", machineId), "", 0, 0)
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

// recordStatistic records a production or consumption event for company statistics
// eventType: "production", "consumption", "money_in", "money_out"
func (gt *GraphTraversal) recordStatistic(companyId, itemId, eventType string, quantity float64) {
	if quantity <= 0 || companyId == "" {
		return
	}

	collection, err := gt.app.FindCollectionByNameOrId("company_statistics")
	if err != nil {
		gt.app.Logger().Error("[STATS] Failed to find company_statistics collection", "err", err)
		return
	}

	record := core.NewRecord(collection)
	record.Set("company", companyId)
	record.Set("item_id", itemId)
	record.Set("event_type", eventType)
	record.Set("quantity", quantity)

	if err := gt.app.Save(record); err != nil {
		gt.app.Logger().Error("[STATS] Failed to save statistic", "err", err)
	}
}

// TraverseStorages iterates all storage nodes for the company to trigger buffering
func (gt *GraphTraversal) TraverseStorages(companyId string) error {
	machines, err := gt.app.FindRecordsByFilter("machines",
		fmt.Sprintf("company = '%s'", companyId),
		"", 0, 0)
	if err != nil {
		return err
	}

	for _, mach := range machines {
		gamedataId := mach.GetString("machine_id")
		itemDef := gamedata.GetItem(gamedataId)
		if itemDef != nil && itemDef.Type == gamedata.ItemTypeStockage && mach.GetBool("placed") {
			// Process as storage/sink (Recursive=true because this is a maintenance task usually)
			gt.ProcessStorage(mach.Id, "", true)
		}
	}
	return nil
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

		// Find outgoing edges using edgeCache
		edges := gt.edgeCache[currentId]
		for _, edge := range edges {
			outType := edge.GetString("output_type")
			outId := edge.GetString("output_id")

			switch outType {
			case "company":
				companyIds[outId] = true
			case "storage":
				storageIds[outId] = true
			case "machine":
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
