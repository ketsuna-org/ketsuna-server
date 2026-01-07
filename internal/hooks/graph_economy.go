package hooks

import (
	"fmt"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// GraphHarvestInterval defines the time unit for mining rates (e.g., rate per 60 seconds)
const GraphHarvestInterval = 60.0

type GraphEconomy struct {
	app *pocketbase.PocketBase
}

func NewGraphEconomy(app *pocketbase.PocketBase) *GraphEconomy {
	return &GraphEconomy{app: app}
}

// CalculateCompanyInventory triggers a pull for all resources flowing into the company
// using the new GraphTraversal engine.
func (g *GraphEconomy) CalculateCompanyInventory(companyId string) (map[string]float64, error) {
	gt := NewGraphTraversal(g.app)

	// 1. Traverse Storages (Update Buffers)
	if err := gt.TraverseStorages(companyId); err != nil {
		g.app.Logger().Error("[GRAPH] TraverseStorages failed", "companyId", companyId, "err", err)
		// Continue? Yes, sales should still happen.
	}

	// 2. Delegate to GraphTraversal for Sales
	flow, err := gt.TraverseFromCompany(companyId)
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

	flow, err := gt.ProcessNode(nodeId, nodeType, "")
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
		if _, err := gt.ProcessNode(storeId, "storage", ""); err != nil {
			g.app.Logger().Error("[GRAPH] Trigger update failed for storage", "id", storeId, "err", err)
		}
	}

	// Update Companies
	for _, compId := range companies {
		if _, err := gt.TraverseFromCompany(compId); err != nil {
			g.app.Logger().Error("[GRAPH] Trigger update failed for company", "id", compId, "err", err)
		}
	}

	return nil
}
