package hooks

import (
	"fmt"
	"sync" // Added sync

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// GraphHarvestInterval defines the time unit for mining rates (e.g., rate per 60 seconds)
const GraphHarvestInterval = 60.0

type GraphEconomy struct {
	app   *pocketbase.PocketBase
	locks sync.Map // Map[string]*sync.Mutex for CompanyID
}

func NewGraphEconomy(app *pocketbase.PocketBase) *GraphEconomy {
	return &GraphEconomy{
		app: app,
		// locks auto-initialized
	}
}

// getLock returns the mutex for a specific company
func (g *GraphEconomy) getLock(companyId string) *sync.Mutex {
	lock, _ := g.locks.LoadOrStore(companyId, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

// CalculateCompanyInventory triggers a pull for all resources flowing into the company
// using the new GraphTraversal engine.
func (g *GraphEconomy) CalculateCompanyInventory(companyId string) (map[string]float64, error) {
	// CRITICAL: Lock to prevent Race Conditions (Double Harvesting) using Keyed Mutex
	// Since TraverseGlobal advances time and consumes resources, concurrent calls (e.g. Tick + View)
	// could process the same time delta twice if not serialized.
	mu := g.getLock(companyId)
	mu.Lock()
	defer mu.Unlock()

	gt := NewGraphTraversal(g.app)

	// 1. Traverse Storages (Update Buffers)
	// Note: TraverseStorages is a helper to ensure all storage buffers catch up if they are heavily used but not connected to company.
	// But in Pull-model, we only care about what flows to Company OR what is requested.
	// We keep it for consistency with old behavior of "updating everything".
	if err := gt.TraverseStorages(companyId); err != nil {
		g.app.Logger().Error("[GRAPH] TraverseStorages failed", "companyId", companyId, "err", err)
	}

	// 2. Delegate to GraphTraversal for Sales (Global Pull)
	flow, err := gt.TraverseGlobal(companyId)
	if err != nil {
		g.app.Logger().Error("[GRAPH] Traversal failed", "companyId", companyId, "err", err)
		return nil, err
	}

	// Commit the flow to company inventory
	if len(flow) > 0 {
		for itemId, qty := range flow {
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
				g.app.Logger().Error("[GRAPH] Failed to save inventory", "item", itemId, "err", err)
			}
		}
	}

	return flow, nil
}

// CalculateNodeFlow calculates how much a node can output NOW.
// Used for single-node verification or debugging.
func (g *GraphEconomy) CalculateNodeFlow(nodeId string, nodeType string) (float64, string, error) {
	gt := NewGraphTraversal(g.app)

	// We need to calculate energy balance first for correct machine processing
	// But we don't know the company ID easily from just a node ID without looking it up.
	// We'll try to find it.
	companyId := ""

	switch nodeType {
	case "deposit":
		rec, err := g.app.FindRecordById("deposits", nodeId)
		if err == nil {
			companyId = rec.GetString("company")
		}
	case "machine":
		rec, err := g.app.FindRecordById("machines", nodeId)
		if err == nil {
			companyId = rec.GetString("company")
		}
	case "storage":
		// Storage implies inventory or machine
		rec, err := g.app.FindRecordById("machines", nodeId) // Try machine first
		if err == nil {
			companyId = rec.GetString("company")
		}
	}

	if companyId != "" {
		// Initialize energy
		balance, err := gt.CalculateEnergyBalance(companyId, nil)
		if err == nil {
			gt.energyBalance = balance
		} else {
			// Fallback if calculation fails
			gt.energyBalance = &EnergyBalance{
				Ratio:     1.0,
				Available: 1000000, // Infinite fallback
			}
			g.app.Logger().Error("[GRAPH] Failed to calculate energy balance, using fallback", "err", err)
		}
	} else {
		// No company found? Use fallback
		gt.energyBalance = &EnergyBalance{Ratio: 1.0}
	}

	// Use TraverseTarget for local calculation
	flow, err := gt.TraverseTarget(nodeId, nodeType)
	if err != nil {
		return 0, "", err
	}

	return flow.Quantity, flow.ItemID, nil
}

// Helper to manually trigger an update for testing
func (g *GraphEconomy) ManualUpdate(companyId string) error {
	_, err := g.CalculateCompanyInventory(companyId)
	return err
}

// TriggerNodeUpdate finds downstream sinks and updates them.
// This allows "Push-like" updates starting from a source/machine.
func (g *GraphEconomy) TriggerNodeUpdate(nodeId string) error {
	gt := NewGraphTraversal(g.app)

	storages, companies, err := gt.FindSinks(nodeId)
	if err != nil {
		return err
	}

	// Update Storages
	for _, storeId := range storages {
		// Check local status of storage (Buffer Update)
		if _, err := gt.TraverseTarget(storeId, "storage"); err != nil {
			g.app.Logger().Error("[GRAPH] Trigger update failed for storage", "id", storeId, "err", err)
		}
	}

	// Update Companies
	for _, compId := range companies {
		// Also Lock here since we are calling TraverseGlobal
		mu := g.getLock(compId)
		mu.Lock()
		// defer inside loop is risky if loop is long, but OK for small loops.
		// Better style: func closure.
		func() {
			defer mu.Unlock()
			if _, err := gt.TraverseGlobal(compId); err != nil {
				g.app.Logger().Error("[GRAPH] Trigger update failed for company", "id", compId, "err", err)
			}
		}()
	}

	return nil
}
