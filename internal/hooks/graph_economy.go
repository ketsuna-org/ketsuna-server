package hooks

import (
	"sync" // Added sync

	"github.com/pocketbase/pocketbase"
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

	// Delegate to GraphTraversal for Global Pull
	// TraverseGlobal consumes from buffers and adds the consumed items to company inventory
	flow, err := gt.TraverseGlobal(companyId)
	if err != nil {
		g.app.Logger().Error("[GRAPH] Traversal failed", "companyId", companyId, "err", err)
		return nil, err
	}

	// Return flow for reporting purposes only (already persisted by TraverseGlobal)
	return flow, nil
}
