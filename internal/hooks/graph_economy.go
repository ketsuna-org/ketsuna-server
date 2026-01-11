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

	// NOTE: TraverseStorages REMOVED - it caused double-processing
	// Storage nodes are now processed naturally during TraverseGlobal via pull model
	// Each storage pulls from upstream and adds to its buffer in a single pass

	// Delegate to GraphTraversal for Global Pull
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
