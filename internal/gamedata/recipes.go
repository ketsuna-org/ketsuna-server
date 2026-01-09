package gamedata

// RecipeIngredient represents an input item and quantity for a recipe
type RecipeIngredient struct {
	ItemID   string `json:"item_id"`
	Quantity int    `json:"quantity"`
}

// Recipe represents a crafting recipe
type Recipe struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	OutputItem      string             `json:"output_item"`      // Item ID produced
	OutputQuantity  int                `json:"output_quantity"`  // Quantity produced per craft
	ProductionTime  int                `json:"production_time"`  // Seconds to complete
	RequiredTech    string             `json:"required_tech"`    // Technology ID required
	Inputs          []RecipeIngredient `json:"inputs"`           // Required ingredients
	MachineType     string             `json:"machine_type"`     // Machine ID that can execute this recipe (empty = any/manual)
	ManualCraftable bool               `json:"manual_craftable"` // If true, player can craft manually in factory
	Icon            string             `json:"icon,omitempty"`
}

// =============================================================================
// STATIC RECIPE DATABASE
// =============================================================================

var Recipes = map[string]Recipe{
	// -------------------------------------------------------------------------
	// COMPOSANTS DE BASE
	// -------------------------------------------------------------------------
	"wooden_plank_recipe": {
		ID: "wooden_plank_recipe", Name: "Sciage du Bois",
		OutputItem: "wooden_plank", OutputQuantity: 2, ProductionTime: 20,
		RequiredTech: "basic_automation", MachineType: "sawmill",
		Inputs: []RecipeIngredient{{ItemID: "wood", Quantity: 1}},
		Icon:   "🪵 ",
	},
	"iron_ingot_recipe": {
		ID: "iron_ingot_recipe", Name: "Fonte du Fer",
		OutputItem: "iron_ingot", OutputQuantity: 1, ProductionTime: 30,
		RequiredTech: "basic_metallurgy", MachineType: "iron_foundry",
		Inputs: []RecipeIngredient{{ItemID: "iron_ore", Quantity: 2}},
		Icon:   "🔩",
	},
	"copper_ingot_recipe": {
		ID: "copper_ingot_recipe", Name: "Fonte du Cuivre",
		OutputItem: "copper_ingot", OutputQuantity: 1, ProductionTime: 30,
		RequiredTech: "basic_metallurgy", MachineType: "copper_foundry",
		Inputs: []RecipeIngredient{{ItemID: "copper_ore", Quantity: 2}},
		Icon:   "🟠",
	},
	"glass_recipe": {
		ID: "glass_recipe", Name: "Fabrication du Verre",
		OutputItem: "glass", OutputQuantity: 1, ProductionTime: 60,
		RequiredTech: "glass_production", MachineType: "glass_furnace",
		Inputs: []RecipeIngredient{{ItemID: "silica", Quantity: 10}},
		Icon:   "🪟",
	},
	"steel_recipe": {
		ID: "steel_recipe", Name: "Fabrication d'Acier",
		OutputItem: "steel", OutputQuantity: 1, ProductionTime: 120,
		RequiredTech: "steel_production", MachineType: "steel_press",
		Inputs: []RecipeIngredient{{ItemID: "iron_ingot", Quantity: 3}},
		Icon:   "⚙️",
	},
	"plastic_recipe": {
		ID: "plastic_recipe", Name: "Rafinage du Pétrole",
		OutputItem: "plastic", OutputQuantity: 2, ProductionTime: 120,
		RequiredTech: "plastic_era", MachineType: "oil_refinery",
		Inputs: []RecipeIngredient{{ItemID: "crude_oil", Quantity: 1}},
		Icon:   "🧱",
	},

	// -------------------------------------------------------------------------
	// COMPOSANTS AVANCÉS
	// -------------------------------------------------------------------------
	"gear_recipe": {
		ID: "gear_recipe", Name: "Fabrication d'Engrenage",
		OutputItem: "gear", OutputQuantity: 2, ProductionTime: 45,
		RequiredTech: "steel_production", MachineType: "steel_press",
		Inputs: []RecipeIngredient{{ItemID: "steel", Quantity: 1}},
		Icon:   "⚙️",
	},
	"electric_cable_recipe": {
		ID: "electric_cable_recipe", Name: "Câblage Électrique",
		OutputItem: "electric_cable", OutputQuantity: 5, ProductionTime: 30,
		RequiredTech: "plastic_era", MachineType: "assembly_line",
		Inputs: []RecipeIngredient{
			{ItemID: "copper_ingot", Quantity: 2},
			{ItemID: "plastic", Quantity: 1},
		},
		Icon: "🔌",
	},
	"simple_circuit_recipe": {
		ID: "simple_circuit_recipe", Name: "Circuit Simple",
		OutputItem: "simple_circuit", OutputQuantity: 1, ProductionTime: 90,
		RequiredTech: "electronics", MachineType: "assembly_line",
		Inputs: []RecipeIngredient{
			{ItemID: "copper_ingot", Quantity: 3},
			{ItemID: "plastic", Quantity: 2},
		},
		Icon: "🔲",
	},
	"battery_cell_recipe": {
		ID: "battery_cell_recipe", Name: "Cellule de Batterie",
		OutputItem: "battery_cell", OutputQuantity: 1, ProductionTime: 120,
		RequiredTech: "assembly_line_tech", MachineType: "assembly_line",
		Inputs: []RecipeIngredient{
			{ItemID: "lithium", Quantity: 5},
			{ItemID: "plastic", Quantity: 3},
		},
		Icon: "🔋",
	},
	"processor_recipe": {
		ID: "processor_recipe", Name: "Fabrication de Processeur",
		OutputItem: "processor", OutputQuantity: 1, ProductionTime: 180,
		RequiredTech: "electronics", MachineType: "hightech_factory",
		Inputs: []RecipeIngredient{
			{ItemID: "simple_circuit", Quantity: 5},
			{ItemID: "gold_ore", Quantity: 2},
			{ItemID: "lithium", Quantity: 3},
		},
		Icon: "💻",
	},

	// -------------------------------------------------------------------------
	// PRODUITS FINIS
	// -------------------------------------------------------------------------
	"electric_motor_recipe": {
		ID: "electric_motor_recipe", Name: "Moteur Électrique",
		OutputItem: "electric_motor", OutputQuantity: 1, ProductionTime: 360,
		RequiredTech: "assembly_line_tech", MachineType: "assembly_line",
		Inputs: []RecipeIngredient{
			{ItemID: "copper_ingot", Quantity: 10},
			{ItemID: "iron_ingot", Quantity: 10},
			{ItemID: "steel", Quantity: 10},
			{ItemID: "plastic", Quantity: 10},
		},
		Icon: "⚡",
	},
	"smartphone_recipe": {
		ID: "smartphone_recipe", Name: "Assemblage Smartphone",
		OutputItem: "smartphone", OutputQuantity: 1, ProductionTime: 600,
		RequiredTech: "hightech_manufacturing", MachineType: "hightech_factory",
		Inputs: []RecipeIngredient{
			{ItemID: "glass", Quantity: 5},
			{ItemID: "lithium", Quantity: 5},
			{ItemID: "gold_ore", Quantity: 5},
			{ItemID: "copper_ingot", Quantity: 5},
			{ItemID: "plastic", Quantity: 5},
		},
		Icon: "📱",
	},
	"computer_recipe": {
		ID: "computer_recipe", Name: "Assemblage Ordinateur",
		OutputItem: "computer", OutputQuantity: 1, ProductionTime: 900,
		RequiredTech: "hightech_manufacturing", MachineType: "hightech_factory",
		Inputs: []RecipeIngredient{
			{ItemID: "processor", Quantity: 2},
			{ItemID: "simple_circuit", Quantity: 10},
			{ItemID: "plastic", Quantity: 15},
			{ItemID: "steel", Quantity: 5},
		},
		Icon: "🖥️",
	},

	// -------------------------------------------------------------------------
	// MACHINES AVANCÉES (Craft-only)
	// -------------------------------------------------------------------------
	"oil_platform_recipe": {
		ID: "oil_platform_recipe", Name: "Construction Plateforme Pétrolière",
		OutputItem: "oil_platform", OutputQuantity: 1, ProductionTime: 200, // 3 minutes et 20 secondes
		RequiredTech: "oil_platform_tech", MachineType: "assembly_line",
		ManualCraftable: true, // Uniquement via machine
		Inputs: []RecipeIngredient{
			{ItemID: "petrol_pumpjack", Quantity: 100},
			{ItemID: "steel", Quantity: 500},
			{ItemID: "plastic", Quantity: 200},
		},
		Icon: "🏗️",
	},
	"reinforced_steel_recipe": {
		ID: "reinforced_steel_recipe", Name: "Fabrication Acier Renforcé",
		OutputItem: "reinforced_steel", OutputQuantity: 5, ProductionTime: 180,
		RequiredTech: "advanced_steel_tech", MachineType: "steel_press",
		ManualCraftable: true,
		Inputs: []RecipeIngredient{
			{ItemID: "steel", Quantity: 10},
			{ItemID: "iron_ingot", Quantity: 5},
		},
		Icon: "🛡️",
	},
}

// GetRecipe returns a recipe by ID, returns nil if not found
func GetRecipe(id string) *Recipe {
	if recipe, ok := Recipes[id]; ok {
		return &recipe
	}
	return nil
}

// GetRecipeName returns the name of a recipe, or "Unknown Recipe" if not found
func GetRecipeName(id string) string {
	if recipe := GetRecipe(id); recipe != nil {
		return recipe.Name
	}
	return "Unknown Recipe"
}

// GetAllRecipes returns a slice of all recipes
func GetAllRecipes() []Recipe {
	result := make([]Recipe, 0, len(Recipes))
	for _, recipe := range Recipes {
		result = append(result, recipe)
	}
	return result
}

// GetRecipesByTech returns recipes that require a specific technology
func GetRecipesByTech(techId string) []Recipe {
	var result []Recipe
	for _, recipe := range Recipes {
		if recipe.RequiredTech == techId {
			result = append(result, recipe)
		}
	}
	return result
}

// GetRecipeByOutput returns the recipe that produces a specific item
func GetRecipeByOutput(itemId string) *Recipe {
	for _, recipe := range Recipes {
		if recipe.OutputItem == itemId {
			return &recipe
		}
	}
	return nil
}

// GetRecipesByMachine returns recipes that can be executed on a specific machine type
func GetRecipesByMachine(machineType string) []Recipe {
	var result []Recipe
	for _, recipe := range Recipes {
		if recipe.MachineType == machineType {
			result = append(result, recipe)
		}
	}
	return result
}

// GetManualRecipes returns recipes that can be done manually (no machine required)
func GetManualRecipes() []Recipe {
	var result []Recipe
	for _, recipe := range Recipes {
		if recipe.MachineType == "" {
			result = append(result, recipe)
		}
	}
	return result
}
