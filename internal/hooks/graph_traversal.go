package hooks

import (
	"fmt"
	"math"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"ketsuna.com/server/internal/gamedata"
)

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

	// 3. Recursive Pull from Company Inputs
	totalFlow := make(map[string]float64)
	edgesToCompany := gt.edgeCache[companyId]

	for _, edge := range edgesToCompany {
		if edge.GetString("output_type") == "company" {
			inputId := edge.GetString("input_id")
			inputType := edge.GetString("input_type")

			// Recursive call to get available resources
			flow, _ := gt.processNodeRecursive(inputId, inputType, companyId, true)
			if flow.Quantity > 0 {
				totalFlow[flow.ItemID] += flow.Quantity

				// CONSUME from the source (storage) - transfer to company
				switch inputType {
				case "storage":
					// Find storage's linked inventory and deduct
					invRecords, err := gt.app.FindRecordsByFilter("inventory", fmt.Sprintf("linked_storage = '%s' && item_id = '%s'", inputId, flow.ItemID), "", 1, 0)
					if err == nil && len(invRecords) > 0 {
						inv := invRecords[0]
						curQty := inv.GetFloat("quantity")
						newQty := curQty - flow.Quantity
						if newQty < 0 {
							newQty = 0
						}
						inv.Set("quantity", newQty)
						gt.app.Save(inv)
						gt.app.Logger().Debug("[GRAPH] Company consumed from storage", "storage", inputId, "item", flow.ItemID, "consumed", flow.Quantity, "remaining", newQty)
					}
				case "machine":
					// Consume from machine buffer
					gt.consumeFromBuffer(inputId, inputType, flow.Quantity)
				}
			}
		}
	}

	// 3.5. Catch-up: Process Disconnected Machines (Orphans)
	// Machines not connected to the company (even indirectly) were not visited above.
	// We must process them so they still consume inputs and produce into their buffers.
	for _, rec := range allMachines {
		machineId := rec.Id
		visitKey := fmt.Sprintf("machine:%s", machineId)
		if !gt.visited[visitKey] {
			// Process in Global mode (recursive=true) to update timers and consume inputs
			// We discard the return flow because it doesn't reach the company
			gt.ProcessMachine(machineId, "GlobalCatchup", true)
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
	invRecords, _ := gt.app.FindRecordsByFilter("inventory", fmt.Sprintf("company = '%s' && linked_storage = ''", companyId), "", 0, 0)
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

	gt.app.Logger().Debug("[GRAPH] Preload Edges", "count", len(allEdges))

	for _, edge := range allEdges {
		outId := edge.GetString("output_id")
		gt.edgeCache[outId] = append(gt.edgeCache[outId], edge)

		// DEBUG: specific check for our machine
		if outId == "eh9s9x7ouewqfsc" {
			gt.app.Logger().Debug("[GRAPH] Found edge for target machine", "edgeId", edge.Id, "inputId", edge.GetString("input_id"))
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
	gt.app.Logger().Debug("[GRAPH] ProcessMachine called", "machine", machineId, "requestedBy", requestedBy, "recursive", recursive)

	machine, err := gt.getMachine(machineId)
	if err != nil || !machine.GetBool("placed") {
		gt.app.Logger().Debug("[GRAPH] ProcessMachine: Not placed or error", "machine", machineId, "err", err)
		return &NodeFlow{}, err
	}

	itemDef := gamedata.GetItem(machine.GetString("machine_id"))
	if itemDef == nil || (itemDef.ProduceEnergy > 0 && itemDef.Product == "") {
		gt.app.Logger().Debug("[GRAPH] ProcessMachine: No itemDef or energy-only", "machine", machineId)
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
		gt.app.Logger().Debug("[GRAPH] ProcessMachine: No output item", "machine", machineId)
		return &NodeFlow{}, nil
	}

	// 1.5. Recursively Process Inputs (Pull Upstream)
	// We must ensure that machines feeding this one are also processed,
	// otherwise they might be dormant if not connected to company directly.
	if recursive {
		for _, edge := range gt.edgeCache[machineId] {
			inputId := edge.GetString("input_id")
			inputType := edge.GetString("input_type")
			// We don't care about the return flow here (we rely on buffers),
			// just trigger the processing so they produce.
			gt.processNodeRecursive(inputId, inputType, machineId, true)
		}
	}

	// 2. Calculate Theoretical Cycles (Time-based)
	startedAt := machine.GetDateTime("production_started_at").Time()
	gt.app.Logger().Debug("[GRAPH] ProcessMachine: Production timer", "machine", machineId, "startedAt", startedAt, "isZero", startedAt.IsZero())

	if startedAt.IsZero() {
		if recursive { // Only auto-start in global mode? Or always? Let's say always for consistency.
			machine.Set("production_started_at", time.Now())
			gt.app.Save(machine)
			gt.app.Logger().Debug("[GRAPH] ProcessMachine: Started production timer", "machine", machineId)
		}
		return &NodeFlow{ItemID: outputItem}, nil
	}

	delta := time.Since(startedAt).Seconds()

	// Determine cycle time: use recipe time for recipe machines, itemDef for extractors
	var cycleTime float64
	if activeRecipeId != "" {
		if r := gamedata.GetRecipe(activeRecipeId); r != nil && r.ProductionTime > 0 {
			cycleTime = float64(r.ProductionTime)
		}
	}
	if cycleTime <= 0 {
		cycleTime = float64(itemDef.ProductionTime)
	}
	if cycleTime <= 0 {
		cycleTime = 60
	}

	effectiveDelta := delta
	timeBasedCycles := int(math.Floor(effectiveDelta / cycleTime))
	gt.app.Logger().Debug("[GRAPH] ProcessMachine: Cycle calculation", "machine", machineId, "delta", delta, "cycleTime", cycleTime, "timeBasedCycles", timeBasedCycles)

	if timeBasedCycles < 1 {
		gt.app.Logger().Debug("[GRAPH] ProcessMachine: Not enough time for a cycle", "machine", machineId)
		return &NodeFlow{ItemID: outputItem}, nil
	}

	// 3. Check Inputs from LOCAL INPUT BUFFERS (Edge-based model)
	// Edges fill input buffers; machines consume from input buffers
	inputsReceived := make(map[string]float64)

	if activeRecipeId != "" {
		// Recipe machine: read from input buffers (filled by edge transfers)
		inputBuffers, err := gt.app.FindRecordsByFilter("machine_input_buffers",
			fmt.Sprintf("machine = '%s'", machineId), "", 0, 0)
		if err == nil {
			for _, buf := range inputBuffers {
				itemId := buf.GetString("item_id")
				qty := buf.GetFloat("quantity")
				if qty > 0 {
					inputsReceived[itemId] = qty
				}
			}
		}
		gt.app.Logger().Debug("[GRAPH] ProcessMachine: Input buffers", "machine", machineId, "inputs", inputsReceived)
	}
	// Note: Extractors don't use input buffers - they pull directly from deposits

	// 4. Calculate Real Cycles (Resource-based)
	maxCycles := timeBasedCycles

	// Cap at reasonable max per tick to prevent resource hogging
	const maxCyclesPerTick = 3
	if maxCycles > maxCyclesPerTick {
		maxCycles = maxCyclesPerTick
		gt.app.Logger().Debug("[GRAPH] ProcessMachine: Capped cycles", "machine", machineId, "timeBasedCycles", timeBasedCycles, "capped", maxCyclesPerTick)
	}

	if activeRecipeId != "" {
		recipe := gamedata.GetRecipe(activeRecipeId)
		if recipe != nil {
			for _, input := range recipe.Inputs {
				available := inputsReceived[input.ItemID]
				possible := int(available / float64(input.Quantity))
				if possible < maxCycles {
					maxCycles = possible
				}
			}
		}
	} else {
		// Extractor - check deposit.quantity DIRECTLY using ProductQuantity per cycle
		// NOTE: Extractors don't use employees, they extract from deposit.quantity
		// ProcessDeposit returns harvested buffer which is for employee mining
		// So we bypass that and check the deposit directly

		// Calculate extraction per cycle using the machine's ProductQuantity
		extractionPerCycle := float64(itemDef.ProductQuantity)
		if extractionPerCycle <= 0 {
			extractionPerCycle = 1
		}

		// CRITICAL: Extractors MUST have a deposit connected to produce anything
		hasDepositInput := false
		for _, edge := range gt.edgeCache[machineId] {
			if edge.GetString("input_type") == "deposit" {
				hasDepositInput = true
				depositId := edge.GetString("input_id")
				deposit, err := gt.app.FindRecordById("deposits", depositId)
				if err == nil {
					available := deposit.GetFloat("quantity")

					// PARTIAL EXTRACTION LOGIC:
					// If deposit has less than needed for a full cycle, we can still extract what's left
					if available <= 0 {
						// Deposit is empty, no cycles possible
						maxCycles = 0
					} else {
						possible := int(available / extractionPerCycle)
						if possible < maxCycles {
							maxCycles = possible
						}

						// If possible == 0 but available > 0, we can do a partial cycle
						// This allows extracting the remainder even if it's less than a full cycle
						if maxCycles == 0 && available > 0 {
							// We'll do a "partial cycle" - extract what's left
							maxCycles = 1 // Force 1 cycle, production will be adjusted below
							gt.app.Logger().Debug("[GRAPH] Extractor: Partial extraction mode", "machine", machineId, "deposit", depositId, "available", available, "extractionPerCycle", extractionPerCycle)
						}
					}

					gt.app.Logger().Debug("[GRAPH] Extractor checking deposit", "machine", machineId, "deposit", depositId, "available", available, "extractionPerCycle", extractionPerCycle, "maxCycles", maxCycles)
				} else {
					gt.app.Logger().Error("[GRAPH] Extractor: Deposit not found", "depositId", depositId)
					maxCycles = 0
				}
				break // Only one deposit per extractor
			}
		}

		// If no deposit is connected, extractor cannot produce anything
		if !hasDepositInput {
			gt.app.Logger().Debug("[GRAPH] Extractor has no deposit input, 0 production", "machine", machineId)
			maxCycles = 0
		}
	}

	if maxCycles < 1 {
		gt.app.Logger().Debug("[GRAPH] ProcessMachine: No cycles possible", "machine", machineId, "inputs", inputsReceived)
		return &NodeFlow{ItemID: outputItem}, nil
	}

	// 5. Commit Production & Consume Inputs
	// Use recipe output quantity for recipe machines, itemDef for extractors
	var qtyPerCycle float64
	if activeRecipeId != "" {
		if r := gamedata.GetRecipe(activeRecipeId); r != nil && r.OutputQuantity > 0 {
			qtyPerCycle = float64(r.OutputQuantity)
		}
	}
	if qtyPerCycle <= 0 {
		qtyPerCycle = float64(itemDef.ProductQuantity)
	}
	if qtyPerCycle <= 0 {
		qtyPerCycle = 1
	}
	totalProduced := math.Round(float64(maxCycles)*qtyPerCycle*100) / 100

	if recursive {
		// Update timers only (Durability removed)
		timeAdvanced := (float64(maxCycles) * cycleTime)
		machine.Set("production_started_at", startedAt.Add(time.Duration(timeAdvanced*float64(time.Second))))
		gt.app.Save(machine)

		// CONSUME FROM INPUT BUFFERS (Buffered Model)
		for _, edge := range gt.edgeCache[machineId] {
			inputType := edge.GetString("input_type")
			inputId := edge.GetString("input_id")

			switch inputType {
			case "deposit":
				// Extractors (no recipe) consume directly from deposit
				// EdgeTransferCron skips deposit→machine edges, so we handle it here
				if activeRecipeId == "" {
					deposit, err := gt.app.FindRecordById("deposits", inputId)
					if err == nil {
						curQty := deposit.GetFloat("quantity")

						// PARTIAL EXTRACTION: Consume only what's available
						consumed := totalProduced
						if consumed > curQty {
							consumed = curQty
							totalProduced = consumed // Adjust production
						}

						newQty := curQty - consumed
						if newQty < 0 {
							newQty = 0
						}

						deposit.Set("quantity", math.Round(newQty))

						// DELETE DEPOSIT IF DEPLETED
						if newQty <= 0 {
							gt.app.Logger().Debug("[GRAPH] Deposit depleted, deleting", "deposit", inputId)
							if err := gt.app.Delete(deposit); err != nil {
								gt.app.Logger().Error("[GRAPH] Failed to delete depleted deposit", "err", err)
							} else {
								gt.app.Delete(edge) // Delete edge too
							}
						} else {
							gt.app.Save(deposit)
						}

						gt.recordStatistic(machine.GetString("company"), deposit.GetString("ressource_id"), "consumption", consumed)
						gt.app.Logger().Debug("[GRAPH] Extractor consumed from deposit", "machine", machineId, "deposit", inputId, "consumed", consumed)
					}
				}
			case "storage":
				// OLD: Recipe machines used to consume from storage directly
				// NEW: Edges now transfer items to input buffers; skip this
				// Kept for backwards compatibility with non-edge connections
				gt.app.Logger().Debug("[GRAPH] Skipping storage consumption (edge-based model)", "machine", machineId, "storage", inputId)

			case "machine":
				// OLD: Recipe machines used to consume from upstream machine output buffer
				// NEW: Edges now transfer items to input buffers; skip this
				// Kept for backwards compatibility with non-edge connections
				gt.app.Logger().Debug("[GRAPH] Skipping machine consumption (edge-based model)", "machine", machineId, "upstream", inputId)
			}
		}

		// NEW: Consume from INPUT BUFFERS for recipe machines
		if activeRecipeId != "" {
			recipe := gamedata.GetRecipe(activeRecipeId)
			if recipe != nil {
				for _, input := range recipe.Inputs {
					consumeQty := float64(input.Quantity) * float64(maxCycles)

					// Find and deduct from input buffer
					inputBuffers, err := gt.app.FindRecordsByFilter("machine_input_buffers",
						fmt.Sprintf("machine = '%s' && item_id = '%s'", machineId, input.ItemID), "", 1, 0)
					if err == nil && len(inputBuffers) > 0 {
						buf := inputBuffers[0]
						curQty := buf.GetFloat("quantity")
						newQty := curQty - consumeQty
						if newQty < 0 {
							newQty = 0
						}
						buf.Set("quantity", newQty)
						gt.app.Save(buf)

						gt.recordStatistic(machine.GetString("company"), input.ItemID, "consumption", consumeQty)
						gt.app.Logger().Debug("[GRAPH] Machine consumed from input buffer", "machine", machineId, "item", input.ItemID, "consumed", consumeQty, "remaining", newQty)
					}
				}
			}
		}

		// Record production statistic
		if totalProduced > 0 {
			gt.recordStatistic(machine.GetString("company"), outputItem, "production", totalProduced)
		}

		// ADD TO MACHINE OUTPUT BUFFER (Buffered Model) - Optional
		// If buffer collection doesn't exist, just return production directly
		bufferWorked := false
		outputBuffer, err := gt.getOrCreateMachineBuffer(machineId, outputItem)
		if err != nil {
			gt.app.Logger().Warn("[GRAPH] Buffer not available, using direct production", "machine", machineId, "err", err)
		} else {
			newBufferQty := outputBuffer.GetFloat("quantity") + totalProduced
			outputBuffer.Set("quantity", newBufferQty)
			gt.app.Save(outputBuffer)
			gt.app.Logger().Debug("[GRAPH] Machine added to output buffer", "machine", machineId, "item", outputItem, "added", totalProduced, "bufferTotal", newBufferQty)
			bufferWorked = true
		}

		// Always return only what was produced THIS cycle
		// The buffer is for internal machine-to-machine transfers, not for bulk dumping
		gt.app.Logger().Debug("[GRAPH] Machine produced", "machine", machineId, "item", outputItem, "qty", totalProduced, "bufferWorked", bufferWorked)
		return &NodeFlow{ItemID: outputItem, Quantity: totalProduced, NodeType: "machine", NodeID: machineId}, nil
	}

	// NOTE: We only reach here if recursive=false (targeted mode), NOT in global traversal
	// In targeted mode, just return 0 - no production without full traversal
	gt.app.Logger().Debug("[GRAPH] Machine in non-recursive mode, returning 0", "machine", machineId)
	return &NodeFlow{ItemID: outputItem, Quantity: 0, NodeType: "machine", NodeID: machineId}, nil
}

func (gt *GraphTraversal) ProcessStorage(storageId, requestedBy string, recursive bool) (*NodeFlow, error) {
	gt.app.Logger().Debug("[GRAPH] ProcessStorage called", "storage", storageId, "requestedBy", requestedBy, "recursive", recursive)

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
		gt.app.Logger().Debug("[GRAPH] ProcessStorage: Pulling from upstream", "storage", storageId, "edgeCount", len(edges))

		for _, edge := range edges {
			inputId := edge.GetString("input_id")
			inputType := edge.GetString("input_type")
			gt.app.Logger().Debug("[GRAPH] ProcessStorage: Processing edge", "inputId", inputId, "inputType", inputType)

			flow, err := gt.processNodeRecursive(inputId, inputType, storageId, true)
			if err != nil {
				gt.app.Logger().Error("[GRAPH] ProcessStorage: Error from input", "inputId", inputId, "err", err)
				continue
			}
			gt.app.Logger().Debug("[GRAPH] ProcessStorage: Got flow from input", "inputId", inputId, "flowItem", flow.ItemID, "flowQty", flow.Quantity)

			if flow.Quantity > 0 && flow.ItemID != "" {
				// Find or create linked inventory
				inv, err := gt.findOrCreateLinkedInventory(companyId, storageId, flow.ItemID)
				if err != nil {
					gt.app.Logger().Error("[GRAPH] ProcessStorage: Failed to find/create inventory", "err", err)
					continue
				}

				// Get storage capacity limit
				storageDef := gamedata.GetItem(storageMachine.GetString("machine_id"))
				maxCapacity := float64(gamedata.MachineBufferCapacity * 100) // Default: 10000
				if storageDef != nil && storageDef.Metadata != nil {
					// Could add StorageCapacity to metadata in future
				}

				currentQty := inv.GetFloat("quantity")
				spaceAvailable := maxCapacity - currentQty
				toAdd := flow.Quantity

				// Enforce capacity limit
				if toAdd > spaceAvailable {
					toAdd = spaceAvailable
					if toAdd < 0 {
						toAdd = 0
					}
					gt.app.Logger().Debug("[GRAPH] ProcessStorage: Capacity limit reached", "storage", storageId, "wanted", flow.Quantity, "adding", toAdd)
				}

				if toAdd > 0 {
					// CONSUME FROM UPSTREAM BUFFER
					gt.consumeFromBuffer(inputId, inputType, toAdd)

					// Add to storage inventory
					newQty := currentQty + toAdd
					inv.Set("quantity", newQty)
					gt.app.Save(inv)
					gt.app.Logger().Debug("[GRAPH] Storage received input", "storage", storageId, "item", flow.ItemID, "added", toAdd, "total", newQty)
				}
			}
		}
	} else if alreadyPulled {
		gt.app.Logger().Debug("[GRAPH] ProcessStorage: Already pulled, just reporting", "storage", storageId)
	}

	// Find linked inventory for serving requests (use the first one found)
	var inv *core.Record
	invRecords, _ := gt.app.FindRecordsByFilter("inventory", fmt.Sprintf("linked_storage = '%s'", storageId), "", 1, 0)
	if len(invRecords) == 0 {
		gt.app.Logger().Debug("[GRAPH] ProcessStorage: No linked inventory to serve", "storage", storageId)
		return &NodeFlow{}, nil
	}
	inv = invRecords[0]
	itemId := inv.GetString("item_id")

	// Report how much is available (but DON'T consume here - let the machine consume what it needs)
	currentQty := inv.GetFloat("quantity")
	gt.app.Logger().Debug("[GRAPH] ProcessStorage: Reporting availability", "storage", storageId, "item", itemId, "available", currentQty)

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

	gt.app.Logger().Debug("[GRAPH] Created linked inventory for storage", "storage", storageId, "item", itemId, "company", companyId)
	return inv, nil
}

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
					// gt.app.Logger().Debug("[GRAPH] Deposit Passive Mine", "id", depositId, "yield", yield, "remaining", newQuantity)
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

	// RETURN BUFFER QUANTITY (what's been harvested and ready for extraction)
	// CRITICAL FIX: Previously returned remainingQty (millions!) causing duplication
	// Now returns harvested buffer - what employees have actually mined
	bufferQty := deposit.GetFloat("harvested")
	gt.app.Logger().Debug("[GRAPH] ProcessDeposit: Returning buffer", "id", depositId, "buffer", bufferQty, "remaining", deposit.GetFloat("quantity"))

	return &NodeFlow{
		ItemID:   resourceId,
		Quantity: bufferQty, // ✅ Return buffer contents, not remaining deposit
		NodeType: "deposit",
		NodeID:   depositId,
	}, nil
}

// getMachineEmployees retrieves employees assigned to a machine from cache or DB
func (gt *GraphTraversal) getMachineEmployees(machineId string) ([]*core.Record, error) {
	if emps, ok := gt.employeeCache[machineId]; ok {
		return emps, nil
	}
	return gt.app.FindRecordsByFilter("employees", fmt.Sprintf("machine = '%s'", machineId), "", 0, 0)
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
		fmt.Sprintf("company = '%s' && placed = '%t'", companyId, true),
		"", 0, 0)
	if err != nil {
		return err
	}

	for _, mach := range machines {
		gamedataId := mach.GetString("machine_id")
		itemDef := gamedata.GetItem(gamedataId)
		if itemDef != nil && itemDef.Type == gamedata.ItemTypeStockage {
			// Process as storage/sink (Recursive=true because this is a maintenance task usually)
			gt.ProcessStorage(mach.Id, "", true)
		}
	}
	return nil
}

// =============================================================================
// BUFFER MANAGEMENT HELPERS (Buffered Resource Flow Model)
// =============================================================================

// getOrCreateMachineBuffer finds or creates a buffer record for a machine's output
func (gt *GraphTraversal) getOrCreateMachineBuffer(machineId, itemId string) (*core.Record, error) {
	filter := fmt.Sprintf("machine = '%s' && item_id = '%s'", machineId, itemId)
	records, err := gt.app.FindRecordsByFilter("machine_buffers", filter, "", 1, 0)
	if err == nil && len(records) > 0 {
		return records[0], nil
	}

	// Create new buffer
	collection, err := gt.app.FindCollectionByNameOrId("machine_buffers")
	if err != nil {
		return nil, fmt.Errorf("machine_buffers collection not found: %w", err)
	}

	buffer := core.NewRecord(collection)
	buffer.Set("machine", machineId)
	buffer.Set("item_id", itemId)
	buffer.Set("quantity", 0)

	if err := gt.app.Save(buffer); err != nil {
		return nil, fmt.Errorf("failed to create machine buffer: %w", err)
	}

	gt.app.Logger().Debug("[GRAPH] Created machine buffer", "machine", machineId, "item", itemId)
	return buffer, nil
}

// getMachineBuffer retrieves the output buffer for a machine (returns nil if not found)
func (gt *GraphTraversal) getMachineBuffer(machineId string) *core.Record {
	filter := fmt.Sprintf("machine = '%s'", machineId)
	records, err := gt.app.FindRecordsByFilter("machine_buffers", filter, "", 1, 0)
	if err == nil && len(records) > 0 {
		return records[0]
	}
	return nil
}

// consumeFromBuffer removes quantity from a node's buffer
func (gt *GraphTraversal) consumeFromBuffer(nodeId, nodeType string, amount float64) error {
	switch nodeType {
	case "deposit":
		deposit, err := gt.app.FindRecordById("deposits", nodeId)
		if err != nil {
			return err
		}
		cur := deposit.GetFloat("harvested")
		newVal := cur - amount
		if newVal < 0 {
			newVal = 0
		}
		deposit.Set("harvested", math.Round(newVal))
		gt.app.Logger().Debug("[GRAPH] Consumed from deposit buffer", "deposit", nodeId, "amount", amount, "remaining", newVal)
		return gt.app.Save(deposit)

	case "machine":
		buffer := gt.getMachineBuffer(nodeId)
		if buffer != nil {
			cur := buffer.GetFloat("quantity")
			newVal := cur - amount
			if newVal < 0 {
				newVal = 0
			}
			buffer.Set("quantity", newVal)
			gt.app.Logger().Debug("[GRAPH] Consumed from machine buffer", "machine", nodeId, "amount", amount, "remaining", newVal)
			return gt.app.Save(buffer)
		}
		return nil

	case "storage":
		// Storage uses inventory with linked_storage - already handled elsewhere
		return nil
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
