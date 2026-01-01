package hooks

import (
	"fmt"
	"math"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// InventoryLogic handles inventory operations
type InventoryLogic struct {
	app *pocketbase.PocketBase
}

func NewInventoryLogic(app *pocketbase.PocketBase) *InventoryLogic {
	return &InventoryLogic{app: app}
}

// UpdateInventory adds or removes quantity from an inventory item
func (l *InventoryLogic) UpdateInventory(companyId, itemId string, quantity int) error {
	// 1. Try to find existing inventory
	// v0.35: app.FindFirstRecordByFilter
	filter := fmt.Sprintf("company = '%s' && item = '%s'", companyId, itemId)
	record, err := l.app.FindFirstRecordByFilter("inventory", filter)

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
			return l.app.Delete(record)
		}

		record.Set("quantity", newQty)
		return l.app.Save(record)
	}

	// --- CREATE ---
	if quantity <= 0 {
		return fmt.Errorf("impossible de retirer %d d'un inventaire inexistant. Item: %s, Company: %s", quantity, itemId, companyId)
	}

	collection, err := l.app.FindCollectionByNameOrId("inventory")
	if err != nil {
		return err
	}

	newRecord := core.NewRecord(collection)
	newRecord.Set("company", companyId)
	newRecord.Set("item", itemId)
	newRecord.Set("quantity", quantity)

	return l.app.Save(newRecord)
}

// HasEnoughItems checks if company has enough of an item
func (l *InventoryLogic) HasEnoughItems(companyId, itemId string, requiredQty int) bool {
	filter := fmt.Sprintf("company = '%s' && item = '%s'", companyId, itemId)
	record, err := l.app.FindFirstRecordByFilter("inventory", filter)
	if err != nil || record == nil {
		return false
	}
	return record.GetInt("quantity") >= requiredQty
}

// HasRequiredTechnology checks if a company has the required tech for a recipe
// Returns (hasIt bool, techName string) - techName is empty if no tech required
func (l *InventoryLogic) HasRequiredTechnology(companyId, recipeId string) (bool, string) {
	recipe, err := l.app.FindRecordById("recipes", recipeId)
	if err != nil {
		return false, ""
	}

	requiredTechId := recipe.GetString("required_tech")
	if requiredTechId == "" {
		return true, "" // No tech required
	}

	filter := fmt.Sprintf("company = '%s' && technology = '%s'", companyId, requiredTechId)
	_, err = l.app.FindFirstRecordByFilter("company_techs", filter)
	if err != nil {
		tech, _ := l.app.FindRecordById("technologies", requiredTechId)
		techName := "Unknown Tech"
		if tech != nil {
			techName = tech.GetString("name")
		}
		return false, techName
	}
	return true, ""
}

// ConsumeItem is a helper to consume a single item (simplifies callers)
func (l *InventoryLogic) ConsumeItem(companyId, itemId string, quantity int) (float64, error) {
	if quantity <= 0 {
		return 0, fmt.Errorf("quantité invalide")
	}
	if !l.HasEnoughItems(companyId, itemId, quantity) {
		return 0, fmt.Errorf("stock insuffisant")
	}
	if err := l.UpdateInventory(companyId, itemId, -quantity); err != nil {
		return 0, err
	}
	return 0, nil // No XP for simple consumption
}

// AddRefinedItem is a helper to add an item (simplifies callers, same as UpdateInventory but explicit intent)
func (l *InventoryLogic) AddRefinedItem(companyId, itemId string, quantity int) error {
	return l.UpdateInventory(companyId, itemId, quantity)
}

// ConsumeInputs verifies requirements and subtracts ingredients for a recipe
// Supports both 'inputs_items' (simple list) and 'ingredients' (relation with specific quantities)
// Returns xpGained
func (l *InventoryLogic) ConsumeInputs(companyId, recipeId string, quantity int) (float64, error) {
	company, err := l.app.FindRecordById("companies", companyId)
	if err != nil {
		return 0, err
	}
	recipe, err := l.app.FindRecordById("recipes", recipeId)
	if err != nil {
		return 0, err
	}

	// 1. Check Technology
	requiredTechId := recipe.GetString("required_tech")
	if requiredTechId != "" {
		filter := fmt.Sprintf("company = '%s' && technology = '%s'", companyId, requiredTechId)
		_, err := l.app.FindFirstRecordByFilter("company_techs", filter)
		if err != nil {
			tech, _ := l.app.FindRecordById("technologies", requiredTechId)
			techName := "Unknown Tech"
			if tech != nil {
				techName = tech.GetString("name")
			}
			return 0, fmt.Errorf("technologie requise: %s", techName)
		}
	}

	// 2. Check Stock for all inputs
	// A. Check 'inputs_items' (Simple list with global input_quantity)
	inputIds := recipe.GetStringSlice("inputs_items")
	unitQty := recipe.GetInt("input_quantity")
	if unitQty == 0 {
		unitQty = 1
	}
	totalRequiredPerItem := unitQty * quantity

	for _, itemId := range inputIds {
		if !l.HasEnoughItems(companyId, itemId, totalRequiredPerItem) {
			item, _ := l.app.FindRecordById("items", itemId)
			itemName := "Unknown Item"
			if item != nil {
				itemName = item.GetString("name")
			}
			return 0, fmt.Errorf("quantité insuffisante de %s. Requis: %d", itemName, totalRequiredPerItem)
		}
	}

	// B. Check 'ingredients' (Relation to recipes_ingredients)
	ingredientRelationIds := recipe.GetStringSlice("ingredients")
	type ingredientReq struct {
		ItemId string
		Qty    int
	}
	var complexIngredients []ingredientReq

	for _, ingRelId := range ingredientRelationIds {
		ingRec, err := l.app.FindRecordById("recipes_ingredients", ingRelId)
		if err != nil {
			continue // Skip invalid relations
		}
		itemId := ingRec.GetString("item")
		qty := ingRec.GetInt("quantity")
		totalQty := qty * quantity

		if !l.HasEnoughItems(companyId, itemId, totalQty) {
			item, _ := l.app.FindRecordById("items", itemId)
			itemName := "Unknown Item"
			if item != nil {
				itemName = item.GetString("name")
			}
			return 0, fmt.Errorf("quantité insuffisante de %s. Requis: %d", itemName, totalQty)
		}
		complexIngredients = append(complexIngredients, ingredientReq{ItemId: itemId, Qty: totalQty})
	}

	// 3. Consume Items
	// A. Consume 'inputs_items'
	for _, itemId := range inputIds {
		if err := l.UpdateInventory(companyId, itemId, -totalRequiredPerItem); err != nil {
			return 0, err
		}
	}

	// B. Consume 'ingredients'
	for _, req := range complexIngredients {
		if err := l.UpdateInventory(companyId, req.ItemId, -req.Qty); err != nil {
			return 0, err
		}
	}

	// 4.  XP
	xpGained := float64((len(inputIds) + len(complexIngredients)) * 10 * quantity)

	if err := l.app.Save(company); err != nil {
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

func (l *InventoryLogic) ProduceItem(companyId, recipeId string, quantity int) (*ProductionResult, error) {
	recipe, err := l.app.FindRecordById("recipes", recipeId)
	if err != nil {
		return nil, err
	}

	// Consume inputs
	xpGained, err := l.ConsumeInputs(companyId, recipeId, quantity)
	if err != nil {
		return nil, err
	}

	outputItemId := recipe.GetString("output_item")
	if err := l.UpdateInventory(companyId, outputItemId, quantity); err != nil {
		return nil, err
	}

	outputItem, _ := l.app.FindRecordById("items", outputItemId)
	itemName := "Unknown"
	if outputItem != nil {
		itemName = outputItem.GetString("name")
	}

	return &ProductionResult{
		Success:        true,
		Produced:       quantity,
		ItemName:       itemName,
		XpGained:       xpGained,
		ProductionTime: recipe.GetInt("production_time"),
	}, nil
}

// CompleteProduction adds the output item (used for long-running production finishing)
func (l *InventoryLogic) CompleteProduction(companyId, recipeId string, quantity int) error {
	recipe, err := l.app.FindRecordById("recipes", recipeId)
	if err != nil {
		return err
	}
	outputItemId := recipe.GetString("output_item")

	if err := l.UpdateInventory(companyId, outputItemId, quantity); err != nil {
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
func (l *InventoryLogic) SellInventory(companyId, itemId string, quantity int) (*SellResult, error) {
	if quantity <= 0 {
		return nil, fmt.Errorf("quantité doit être > 0")
	}

	company, err := l.app.FindRecordById("companies", companyId)
	if err != nil {
		return nil, err
	}
	item, err := l.app.FindRecordById("items", itemId)
	if err != nil {
		return nil, err
	}

	// Check stock
	if !l.HasEnoughItems(companyId, itemId, quantity) {
		inv, _ := l.app.FindFirstRecordByFilter("inventory", fmt.Sprintf("company = '%s' && item = '%s'", companyId, itemId))
		have := 0
		if inv != nil {
			have = inv.GetInt("quantity")
		}
		return nil, fmt.Errorf("stock insuffisant. Disponible: %d, demandé: %d", have, quantity)
	}

	// Calculate prices
	unitBuyPrice := item.GetFloat("base_price")
	unitSellPrice := math.Round(((unitBuyPrice/2)+math.SmallestNonzeroFloat64)*100) / 100
	revenue := unitSellPrice * float64(quantity)

	// Update Inventory
	if err := l.UpdateInventory(companyId, itemId, -quantity); err != nil {
		return nil, err
	}

	// Update Company Balance
	currentBalance := company.GetFloat("balance")

	newBalance := math.Round((currentBalance+revenue)*100) / 100
	company.Set("balance", newBalance)

	// Tech points (not awarded on sale anymore)
	techGain := 0.0
	if err := l.app.Save(company); err != nil {
		return nil, err
	}

	return &SellResult{
		Revenue:       revenue,
		UnitSellPrice: unitSellPrice,
		TechGain:      techGain,
	}, nil
}
