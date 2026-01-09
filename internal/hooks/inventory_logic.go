package hooks

import (
	"fmt"
	"math"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"ketsuna.com/server/internal/gamedata"
)

// InventoryLogic handles inventory operations
type InventoryLogic struct {
	app *pocketbase.PocketBase
}

func NewInventoryLogic(app *pocketbase.PocketBase) *InventoryLogic {
	return &InventoryLogic{app: app}
}

// UpdateInventory adds or removes quantity from an inventory item
func (l *InventoryLogic) UpdateInventory(app core.App, companyId, itemId string, quantity int) error {
	// Check if item is a Machine
	item := gamedata.GetItem(itemId)
	if item != nil && item.Type == "Machine" {
		if quantity > 0 {
			// Create new machine records
			collection, err := app.FindCollectionByNameOrId("machines")
			if err != nil {
				return err
			}

			for i := 0; i < quantity; i++ {
				record := core.NewRecord(collection)
				record.Set("company", companyId)
				record.Set("machine_id", itemId)
				record.Set("placed", false)
				record.Set("name", item.Name)
				// Initialize other fields if necessary (durability, etc.)

				if err := app.Save(record); err != nil {
					return err
				}
			}
			return nil
		} else if quantity < 0 {
			// Remove machines (Consumed)
			// This logic is also partially in ConsumeItem, but good to have here for completeness
			reqQty := int(math.Abs(float64(quantity)))
			records, err := app.FindRecordsByFilter("machines",
				fmt.Sprintf("company = '%s' && machine_id = '%s' && placed = false", companyId, itemId),
				"-created",
				reqQty,
				0,
			)
			if err != nil {
				return err
			}
			if len(records) < reqQty {
				return fmt.Errorf("pas assez de machines '%s' pour en retirer %d", item.Name, reqQty)
			}

			for _, r := range records {
				if err := app.Delete(r); err != nil {
					return err
				}
			}
			return nil
		}
		return nil // Quantity 0, do nothing
	}

	// Standard Inventory Logic
	// 1. Try to find existing inventory
	// Use item_id text field instead of item relation
	filter := fmt.Sprintf("company = '%s' && item_id = '%s'", companyId, itemId)
	record, err := app.FindFirstRecordByFilter("inventory", filter)

	if err != nil {
		// Not found usually returns error in PB.
	}

	if record != nil {
		// --- UPDATE ---
		currentQty := record.GetInt("quantity")
		newQty := currentQty + quantity

		if newQty < 0 {
			return fmt.Errorf("quantité insuffisante (disponible: %d, requis: %d)", currentQty, int(math.Abs(float64(quantity))))
		}

		if newQty == 0 {
			return app.Delete(record)
		}

		record.Set("quantity", newQty)
		return app.Save(record)
	}

	// --- CREATE ---
	if quantity <= 0 {
		return fmt.Errorf("impossible de retirer %d d'un inventaire inexistant. Item: %s, Company: %s", quantity, itemId, companyId)
	}

	collection, err := app.FindCollectionByNameOrId("inventory")
	if err != nil {
		return err
	}

	newRecord := core.NewRecord(collection)
	newRecord.Set("company", companyId)
	newRecord.Set("item_id", itemId)
	newRecord.Set("quantity", quantity)

	return app.Save(newRecord)
}

// HasEnoughItems checks if company has enough of an item
func (l *InventoryLogic) HasEnoughItems(app core.App, companyId, itemId string, requiredQty int) bool {
	// Check item type
	item := gamedata.GetItem(itemId)
	if item != nil && item.Type == "Machine" {
		// For machines, count unplaced machines in machines collection
		// using machine_id field
		records, err := app.FindRecordsByFilter("machines",
			fmt.Sprintf("company = '%s' && machine_id = '%s' && placed = false", companyId, itemId),
			"-created",
			requiredQty, // Limit to required quantity to optimize
			0,
		)
		if err != nil {
			return false
		}
		return len(records) >= requiredQty
	}

	// For standard items, check inventory collection
	filter := fmt.Sprintf("company = '%s' && item_id = '%s'", companyId, itemId)
	record, err := app.FindFirstRecordByFilter("inventory", filter)
	if err != nil || record == nil {
		return false
	}
	return record.GetInt("quantity") >= requiredQty
}

// HasRequiredTechnology checks if a company has the required tech for a recipe
// Returns (hasIt bool, techName string) - techName is empty if no tech required
func (l *InventoryLogic) HasRequiredTechnology(app core.App, companyId, recipeId string) (bool, string) {
	// Trigger lazy tech update first
	if err := UpdateCompanyTechStatus(app, companyId); err != nil {
		l.app.Logger().Error("[TECH] Lazy status update failed", "company", companyId, "err", err)
	}

	// Use static gamedata for recipe lookup
	recipe := gamedata.GetRecipe(recipeId)
	if recipe == nil {
		return false, ""
	}

	requiredTechId := recipe.RequiredTech
	if requiredTechId == "" {
		return true, "" // No tech required
	}

	// Use technology_id text field and check for COMPLETED status strictly
	filter := fmt.Sprintf("company = '%s' && technology_id = '%s' && status = 'completed'", companyId, requiredTechId)
	_, err := app.FindFirstRecordByFilter("company_techs", filter)
	if err != nil {
		// Use static gamedata for tech name
		techName := gamedata.GetTechnologyName(requiredTechId)
		return false, techName
	}
	return true, ""
}

// ConsumeItem is a helper to consume a single item (simplifies callers)
func (l *InventoryLogic) ConsumeItem(app core.App, companyId, itemId string, quantity int) (float64, error) {
	if quantity <= 0 {
		return 0, fmt.Errorf("quantité invalide")
	}
	if !l.HasEnoughItems(app, companyId, itemId, quantity) {
		return 0, fmt.Errorf("stock insuffisant")
	}

	item := gamedata.GetItem(itemId)
	if item != nil && item.Type == "Machine" {
		// Consume machines
		records, err := app.FindRecordsByFilter("machines",
			fmt.Sprintf("company = '%s' && machine_id = '%s' && placed = false", companyId, itemId),
			"-created",
			quantity,
			0,
		)
		if err != nil || len(records) < quantity {
			return 0, fmt.Errorf("pas assez de machines '%s' non placées (requis: %d)", item.Name, quantity)
		}

		for _, r := range records {
			if err := app.Delete(r); err != nil {
				return 0, err
			}
		}
		return 0, nil
	}

	if err := l.UpdateInventory(app, companyId, itemId, -quantity); err != nil {
		return 0, err
	}
	return 0, nil // No XP for simple consumption
}

// AddRefinedItem is a helper to add an item (simplifies callers, same as UpdateInventory but explicit intent)
func (l *InventoryLogic) AddRefinedItem(app core.App, companyId, itemId string, quantity int) error {
	return l.UpdateInventory(app, companyId, itemId, quantity)
}

// ConsumeInputs verifies requirements and subtracts ingredients for a recipe
// Uses static gamedata for recipe and item lookups
// Returns xpGained
func (l *InventoryLogic) ConsumeInputs(app core.App, companyId, recipeId string, quantity int) (float64, error) {
	company, err := app.FindRecordById("companies", companyId)
	if err != nil {
		return 0, err
	}

	// Use static gamedata for recipe
	recipe := gamedata.GetRecipe(recipeId)
	if recipe == nil {
		return 0, fmt.Errorf("unknown recipe: %s", recipeId)
	}

	// 1. Check Technology using technology_id
	requiredTechId := recipe.RequiredTech
	if requiredTechId != "" {
		// Trigger lazy tech update first
		if err := UpdateCompanyTechStatus(app, companyId); err != nil {
			l.app.Logger().Error("[TECH] Lazy status update failed in ConsumeInputs", "company", companyId, "err", err)
		}

		filter := fmt.Sprintf("company = '%s' && technology_id = '%s' && status = 'completed'", companyId, requiredTechId)
		_, err := app.FindFirstRecordByFilter("company_techs", filter)
		if err != nil {
			techName := gamedata.GetTechnologyName(requiredTechId)
			return 0, fmt.Errorf("technologie requise: %s", techName)
		}
	}

	// 2. Check Stock for all inputs from static recipe data
	for _, input := range recipe.Inputs {
		totalRequired := input.Quantity * quantity
		if !l.HasEnoughItems(app, companyId, input.ItemID, totalRequired) {
			itemName := gamedata.GetItemName(input.ItemID)
			return 0, fmt.Errorf("quantité insuffisante de %s. Requis: %d", itemName, totalRequired)
		}
	}

	// 3. Consume Items from static recipe inputs
	for _, input := range recipe.Inputs {
		totalRequired := input.Quantity * quantity

		// Check if it's a machine
		item := gamedata.GetItem(input.ItemID)
		if item != nil && item.Type == "Machine" {
			// Find unplaced machines to delete
			records, err := app.FindRecordsByFilter("machines",
				fmt.Sprintf("company = '%s' && machine_id = '%s' && placed = false", companyId, input.ItemID),
				"-created", // Delete newest first? Or oldest? Doesn't matter much for identical machines
				totalRequired,
				0,
			)
			if err != nil || len(records) < totalRequired {
				return 0, fmt.Errorf("erreur fatale: stock de machines '%s' a changé pendant la transaction", item.Name)
			}

			// Delete them
			for _, r := range records {
				if err := app.Delete(r); err != nil {
					return 0, err
				}
			}
		} else {
			// Standard inventory item
			if err := l.UpdateInventory(app, companyId, input.ItemID, -totalRequired); err != nil {
				return 0, err
			}
		}
	}

	// 4. XP (no XP reward in static recipes currently)
	xpGained := 0.0

	if err := app.Save(company); err != nil {
		return 0, err
	}

	return xpGained, nil
}

// ProduceItem handles the immediate production of an item (short recipe or manual)
type ProductionResult struct {
	Success        bool
	Produced       int
	ItemName       string
	XpGained       float64
	ProductionTime int
}

func (l *InventoryLogic) ProduceItem(app core.App, companyId, recipeId string, quantity int) (*ProductionResult, error) {
	// Use static gamedata for recipe
	recipe := gamedata.GetRecipe(recipeId)
	if recipe == nil {
		return nil, fmt.Errorf("unknown recipe: %s", recipeId)
	}

	// Consume inputs
	xpGained, err := l.ConsumeInputs(app, companyId, recipeId, quantity)
	if err != nil {
		return nil, err
	}

	outputItemId := recipe.OutputItem
	outputQty := recipe.OutputQuantity
	if outputQty == 0 {
		outputQty = 1
	}
	totalOutput := outputQty * quantity

	if err := l.UpdateInventory(app, companyId, outputItemId, totalOutput); err != nil {
		return nil, err
	}

	// Use static gamedata for item name
	itemName := gamedata.GetItemName(outputItemId)

	return &ProductionResult{
		Success:        true,
		Produced:       totalOutput,
		ItemName:       itemName,
		XpGained:       xpGained,
		ProductionTime: recipe.ProductionTime,
	}, nil
}

// CompleteProduction adds the output item (used for long-running production finishing)
func (l *InventoryLogic) CompleteProduction(app core.App, companyId, recipeId string, quantity int) error {
	// Use static gamedata for recipe
	recipe := gamedata.GetRecipe(recipeId)
	if recipe == nil {
		return fmt.Errorf("unknown recipe: %s", recipeId)
	}

	outputItemId := recipe.OutputItem
	outputQty := recipe.OutputQuantity
	if outputQty == 0 {
		outputQty = 1
	}
	totalOutput := outputQty * quantity

	if err := l.UpdateInventory(app, companyId, outputItemId, totalOutput); err != nil {
		return err
	}

	return nil
}

type SellResult struct {
	Revenue       float64
	UnitSellPrice float64
	TechGain      float64
}

// SellInventory handles selling items to the system
// Uses static gamedata for item data - no market updates since items are static
func (l *InventoryLogic) SellInventory(app core.App, companyId, itemId string, quantity int) (*SellResult, error) {
	if quantity <= 0 {
		return nil, fmt.Errorf("quantité doit être > 0")
	}

	company, err := app.FindRecordById("companies", companyId)
	if err != nil {
		return nil, err
	}

	// Use static gamedata for item
	item := gamedata.GetItem(itemId)
	if item == nil {
		return nil, fmt.Errorf("unknown item: %s", itemId)
	}

	// Check stock using item_id
	if !l.HasEnoughItems(app, companyId, itemId, quantity) {
		inv, _ := app.FindFirstRecordByFilter("inventory", fmt.Sprintf("company = '%s' && item_id = '%s'", companyId, itemId))
		have := 0
		if inv != nil {
			have = inv.GetInt("quantity")
		}
		return nil, fmt.Errorf("stock insuffisant. Disponible: %d, demandé: %d", have, quantity)
	}

	// Calculate prices from static data
	unitBuyPrice := item.BasePrice
	unitSellPrice := math.Round(((unitBuyPrice/2)+math.SmallestNonzeroFloat64)*100) / 100
	revenue := unitSellPrice * float64(quantity)

	// Update Inventory
	if err := l.UpdateInventory(app, companyId, itemId, -quantity); err != nil {
		return nil, err
	}

	// Update Company Balance
	currentBalance := company.GetFloat("balance")
	newBalance := math.Round((currentBalance+revenue)*100) / 100
	company.Set("balance", newBalance)

	// No market updates - items are now static

	if err := app.Save(company); err != nil {
		return nil, err
	}

	return &SellResult{
		Revenue:       revenue,
		UnitSellPrice: unitSellPrice,
		TechGain:      0.0,
	}, nil
}
