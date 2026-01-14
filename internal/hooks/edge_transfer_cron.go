package hooks

import (
	"fmt"
	"math"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// EdgeTransferRate is the fixed transfer rate per edge (items per second)
const EdgeTransferRate = 5.0

// EdgeTransferCron handles periodic transfer of items along edges
type EdgeTransferCron struct {
	app *pocketbase.PocketBase
}

func NewEdgeTransferCron(app *pocketbase.PocketBase) *EdgeTransferCron {
	return &EdgeTransferCron{app: app}
}

// TransferAll processes all edges and moves items from source to target
func (c *EdgeTransferCron) TransferAll() error {
	// Fetch all edges
	edges, err := c.app.FindRecordsByFilter("edge_relation", "", "", 0, 0)
	if err != nil {
		return fmt.Errorf("failed to fetch edges: %w", err)
	}

	now := time.Now()

	for _, edge := range edges {
		if err := c.processEdge(edge, now); err != nil {
			c.app.Logger().Error("[EDGE_TRANSFER] Failed to process edge", "edge", edge.Id, "err", err)
			// Continue processing other edges
		}
	}

	return nil
}

// processEdge transfers items along a single edge
func (c *EdgeTransferCron) processEdge(edge *core.Record, now time.Time) error {
	inputId := edge.GetString("input_id")
	inputType := edge.GetString("input_type")
	outputId := edge.GetString("output_id")
	outputType := edge.GetString("output_type")

	if inputId == "" || outputId == "" {
		return nil // Invalid edge
	}

	// SKIP deposit→machine edges: Extractors handle deposit extraction directly
	// in ProcessMachine via time-based production. Edge transfer is for:
	// machine→machine, machine→storage, storage→company, etc.
	if inputType == "deposit" && outputType == "machine" {
		return nil
	}

	// Calculate time since last transfer
	lastTransfer := edge.GetDateTime("last_transfer_at").Time()
	if lastTransfer.IsZero() {
		// First transfer - initialize timestamp
		edge.Set("last_transfer_at", now)
		return c.app.Save(edge)
	}

	delta := now.Sub(lastTransfer).Seconds()
	if delta < 1.0 {
		return nil // Too soon, wait at least 1 second between transfers
	}

	// Calculate theoretical transfer amount
	theoreticalTransfer := EdgeTransferRate * delta

	// Get available amount from source
	sourceItem, sourceQty := c.getSourceAvailable(inputId, inputType)
	if sourceItem == "" || sourceQty <= 0 {
		// No items to transfer, but still update timestamp to prevent time accumulation
		edge.Set("last_transfer_at", now)
		return c.app.Save(edge)
	}

	// Get available space in target
	targetSpace := c.getTargetSpace(outputId, outputType, sourceItem)
	if targetSpace <= 0 {
		// Target is full
		return nil
	}

	// Actual transfer = min(theoretical, available, space)
	toTransfer := math.Min(theoreticalTransfer, math.Min(sourceQty, targetSpace))
	toTransfer = math.Floor(toTransfer) // Only transfer whole units

	if toTransfer < 1 {
		return nil // Not enough to transfer
	}

	// Execute transfer
	if err := c.deductFromSource(inputId, inputType, sourceItem, toTransfer); err != nil {
		return fmt.Errorf("failed to deduct from source: %w", err)
	}

	if err := c.addToTarget(outputId, outputType, sourceItem, toTransfer); err != nil {
		return fmt.Errorf("failed to add to target: %w", err)
	}

	// Update last transfer timestamp
	edge.Set("last_transfer_at", now)
	if err := c.app.Save(edge); err != nil {
		return fmt.Errorf("failed to save edge: %w", err)
	}

	c.app.Logger().Debug("[EDGE_TRANSFER] Transferred",
		"edge", edge.Id,
		"item", sourceItem,
		"qty", toTransfer,
		"from", fmt.Sprintf("%s:%s", inputType, inputId),
		"to", fmt.Sprintf("%s:%s", outputType, outputId))

	return nil
}

// getSourceAvailable returns the item ID and available quantity from the source node
func (c *EdgeTransferCron) getSourceAvailable(nodeId, nodeType string) (string, float64) {
	switch nodeType {
	case "deposit":
		deposit, err := c.app.FindRecordById("deposits", nodeId)
		if err != nil {
			return "", 0
		}
		// For deposits, the item is the ressource_id
		return deposit.GetString("ressource_id"), deposit.GetFloat("quantity")

	case "machine":
		// Get from machine's output buffer
		// Sort by quantity descending to prioritize buffers that have items
		buffers, err := c.app.FindRecordsByFilter("machine_buffers",
			fmt.Sprintf("machine = '%s'", nodeId), "-quantity", 1, 0)
		if err != nil || len(buffers) == 0 {
			return "", 0
		}
		return buffers[0].GetString("item_id"), buffers[0].GetFloat("quantity")

	case "storage":
		// Get from storage's linked inventory
		// Sort by quantity descending to prioritize inventories that have items
		invs, err := c.app.FindRecordsByFilter("inventory",
			fmt.Sprintf("linked_storage = '%s'", nodeId), "-quantity", 1, 0)
		if err != nil || len(invs) == 0 {
			return "", 0
		}
		return invs[0].GetString("item_id"), invs[0].GetFloat("quantity")
	}
	return "", 0
}

// getTargetSpace returns available space in the target node for a given item
func (c *EdgeTransferCron) getTargetSpace(nodeId, nodeType, itemId string) float64 {
	const defaultCapacity = 10000.0

	switch nodeType {
	case "machine":
		// Get machine's input buffer
		buffers, err := c.app.FindRecordsByFilter("machine_input_buffers",
			fmt.Sprintf("machine = '%s' && item_id = '%s'", nodeId, itemId), "", 1, 0)
		if err != nil || len(buffers) == 0 {
			// No buffer exists yet - assume default capacity
			return defaultCapacity
		}
		capacity := buffers[0].GetFloat("capacity")
		if capacity <= 0 {
			capacity = defaultCapacity
		}
		current := buffers[0].GetFloat("quantity")
		return capacity - current

	case "storage":
		// Storage has a capacity (from machine metadata)
		// For now, assume large capacity
		return defaultCapacity

	case "company":
		// Company inventory is unlimited
		return math.MaxFloat64
	}
	return 0
}

// deductFromSource removes items from the source node
func (c *EdgeTransferCron) deductFromSource(nodeId, nodeType, itemId string, qty float64) error {
	switch nodeType {
	case "deposit":
		deposit, err := c.app.FindRecordById("deposits", nodeId)
		if err != nil {
			return err
		}
		newQty := deposit.GetFloat("quantity") - qty
		if newQty <= 0 {
			// Deposit depleted - delete it
			return c.app.Delete(deposit)
		}
		deposit.Set("quantity", math.Round(newQty))
		return c.app.Save(deposit)

	case "machine":
		buffers, err := c.app.FindRecordsByFilter("machine_buffers",
			fmt.Sprintf("machine = '%s' && item_id = '%s'", nodeId, itemId), "", 1, 0)
		if err != nil || len(buffers) == 0 {
			return fmt.Errorf("machine buffer not found")
		}
		buffer := buffers[0]
		newQty := buffer.GetFloat("quantity") - qty
		if newQty < 0 {
			newQty = 0
		}
		buffer.Set("quantity", newQty)
		return c.app.Save(buffer)

	case "storage":
		invs, err := c.app.FindRecordsByFilter("inventory",
			fmt.Sprintf("linked_storage = '%s' && item_id = '%s'", nodeId, itemId), "", 1, 0)
		if err != nil || len(invs) == 0 {
			return fmt.Errorf("storage inventory not found")
		}
		inv := invs[0]
		newQty := inv.GetFloat("quantity") - qty
		if newQty < 0 {
			newQty = 0
		}
		inv.Set("quantity", newQty)
		return c.app.Save(inv)
	}
	return nil
}

// addToTarget adds items to the target node
func (c *EdgeTransferCron) addToTarget(nodeId, nodeType, itemId string, qty float64) error {
	switch nodeType {
	case "machine":
		// Add to machine's INPUT buffer
		return c.addToMachineInputBuffer(nodeId, itemId, qty)

	case "storage":
		// Add to storage's linked inventory
		return c.addToStorageInventory(nodeId, itemId, qty)

	case "company":
		// Add to company's general inventory
		return c.addToCompanyInventory(nodeId, itemId, qty)
	}
	return nil
}

// addToMachineInputBuffer adds items to a machine's input buffer
func (c *EdgeTransferCron) addToMachineInputBuffer(machineId, itemId string, qty float64) error {
	buffers, err := c.app.FindRecordsByFilter("machine_input_buffers",
		fmt.Sprintf("machine = '%s' && item_id = '%s'", machineId, itemId), "", 1, 0)

	var buffer *core.Record
	if err == nil && len(buffers) > 0 {
		buffer = buffers[0]
		buffer.Set("quantity", buffer.GetFloat("quantity")+qty)
	} else {
		// Create new buffer
		collection, err := c.app.FindCollectionByNameOrId("machine_input_buffers")
		if err != nil {
			return err
		}
		buffer = core.NewRecord(collection)
		buffer.Set("machine", machineId)
		buffer.Set("item_id", itemId)
		buffer.Set("quantity", qty)
		buffer.Set("capacity", 10000) // Default capacity
	}

	return c.app.Save(buffer)
}

// addToStorageInventory adds items to a storage's linked inventory
func (c *EdgeTransferCron) addToStorageInventory(storageId, itemId string, qty float64) error {
	// Get storage machine to find company
	storage, err := c.app.FindRecordById("machines", storageId)
	if err != nil {
		return err
	}
	companyId := storage.GetString("company")

	invs, err := c.app.FindRecordsByFilter("inventory",
		fmt.Sprintf("linked_storage = '%s' && item_id = '%s'", storageId, itemId), "", 1, 0)

	var inv *core.Record
	if err == nil && len(invs) > 0 {
		inv = invs[0]
		inv.Set("quantity", inv.GetFloat("quantity")+qty)
	} else {
		// Create new inventory record
		collection, err := c.app.FindCollectionByNameOrId("inventory")
		if err != nil {
			return err
		}
		inv = core.NewRecord(collection)
		inv.Set("company", companyId)
		inv.Set("item_id", itemId)
		inv.Set("quantity", qty)
		inv.Set("linked_storage", storageId)
	}

	return c.app.Save(inv)
}

// addToCompanyInventory adds items to company's general inventory
func (c *EdgeTransferCron) addToCompanyInventory(companyId, itemId string, qty float64) error {
	invs, err := c.app.FindRecordsByFilter("inventory",
		fmt.Sprintf("company = '%s' && item_id = '%s' && linked_storage = ''", companyId, itemId), "", 1, 0)

	var inv *core.Record
	if err == nil && len(invs) > 0 {
		inv = invs[0]
		inv.Set("quantity", inv.GetFloat("quantity")+qty)
	} else {
		// Create new inventory record
		collection, err := c.app.FindCollectionByNameOrId("inventory")
		if err != nil {
			return err
		}
		inv = core.NewRecord(collection)
		inv.Set("company", companyId)
		inv.Set("item_id", itemId)
		inv.Set("quantity", qty)
	}

	return c.app.Save(inv)
}
