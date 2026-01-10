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

	// -------------------------------------------------------------------------
	// COMPOSANTS INTERMÉDIAIRES RECIPES
	// -------------------------------------------------------------------------
	"carbon_fiber_recipe": {
		ID: "carbon_fiber_recipe", Name: "Fabrication Fibre de Carbone",
		OutputItem: "carbon_fiber", OutputQuantity: 2, ProductionTime: 180,
		RequiredTech: "advanced_materials", MachineType: "carbonization_furnace",
		Inputs: []RecipeIngredient{
			{ItemID: "plastic", Quantity: 5},
			{ItemID: "coal", Quantity: 10},
		},
		Icon: "🖤",
	},
	"ceramic_recipe": {
		ID: "ceramic_recipe", Name: "Fabrication Céramique Technique",
		OutputItem: "ceramic", OutputQuantity: 3, ProductionTime: 120,
		RequiredTech: "advanced_materials", MachineType: "ceramic_kiln",
		Inputs: []RecipeIngredient{
			{ItemID: "silica", Quantity: 20},
			{ItemID: "stone", Quantity: 10},
		},
		Icon: "🏺",
	},
	"rubber_recipe": {
		ID: "rubber_recipe", Name: "Synthèse Caoutchouc",
		OutputItem: "rubber", OutputQuantity: 5, ProductionTime: 90,
		RequiredTech: "industrial_chemistry", MachineType: "rubber_factory",
		Inputs: []RecipeIngredient{
			{ItemID: "crude_oil", Quantity: 5},
			{ItemID: "plastic", Quantity: 2},
		},
		Icon: "⚫",
	},
	"sensor_recipe": {
		ID: "sensor_recipe", Name: "Fabrication Capteur",
		OutputItem: "sensor", OutputQuantity: 1, ProductionTime: 240,
		RequiredTech: "precision_engineering", MachineType: "precision_workshop",
		Inputs: []RecipeIngredient{
			{ItemID: "simple_circuit", Quantity: 3},
			{ItemID: "copper_ingot", Quantity: 5},
			{ItemID: "glass", Quantity: 2},
		},
		Icon: "📡",
	},
	"hydraulic_cylinder_recipe": {
		ID: "hydraulic_cylinder_recipe", Name: "Fabrication Vérin Hydraulique",
		OutputItem: "hydraulic_cylinder", OutputQuantity: 2, ProductionTime: 150,
		RequiredTech: "precision_engineering", MachineType: "hydraulic_press",
		Inputs: []RecipeIngredient{
			{ItemID: "steel", Quantity: 5},
			{ItemID: "rubber", Quantity: 3},
			{ItemID: "copper_ingot", Quantity: 2},
		},
		Icon: "🔧",
	},
	"advanced_alloy_recipe": {
		ID: "advanced_alloy_recipe", Name: "Fabrication Alliage Avancé",
		OutputItem: "advanced_alloy", OutputQuantity: 2, ProductionTime: 200,
		RequiredTech: "advanced_metallurgy", MachineType: "advanced_forge",
		Inputs: []RecipeIngredient{
			{ItemID: "steel", Quantity: 5},
			{ItemID: "iron_ingot", Quantity: 5},
			{ItemID: "copper_ingot", Quantity: 3},
		},
		Icon: "⚙️",
	},
	"turbopump_recipe": {
		ID: "turbopump_recipe", Name: "Fabrication Turbopompe",
		OutputItem: "turbopump", OutputQuantity: 1, ProductionTime: 360,
		RequiredTech: "advanced_metallurgy", MachineType: "assembly_line",
		Inputs: []RecipeIngredient{
			{ItemID: "advanced_alloy", Quantity: 5},
			{ItemID: "gear", Quantity: 10},
			{ItemID: "electric_motor", Quantity: 1},
		},
		Icon: "🌀",
	},
	"pressurized_tank_recipe": {
		ID: "pressurized_tank_recipe", Name: "Fabrication Réservoir Pressurisé",
		OutputItem: "pressurized_tank", OutputQuantity: 1, ProductionTime: 240,
		RequiredTech: "advanced_metallurgy", MachineType: "advanced_forge",
		Inputs: []RecipeIngredient{
			{ItemID: "advanced_alloy", Quantity: 10},
			{ItemID: "rubber", Quantity: 5},
		},
		Icon: "🛢️",
	},

	// -------------------------------------------------------------------------
	// AEROSPACE RECIPES
	// -------------------------------------------------------------------------
	"aluminum_ingot_recipe": {
		ID: "aluminum_ingot_recipe", Name: "Fonte de l'Aluminium",
		OutputItem: "aluminum_ingot", OutputQuantity: 2, ProductionTime: 60,
		RequiredTech: "aluminum_processing", MachineType: "aluminum_foundry",
		Inputs: []RecipeIngredient{{ItemID: "aluminum_ore", Quantity: 3}},
		Icon:   "⚪",
	},
	"titanium_ingot_recipe": {
		ID: "titanium_ingot_recipe", Name: "Fonte du Titane",
		OutputItem: "titanium_ingot", OutputQuantity: 1, ProductionTime: 180,
		RequiredTech: "titanium_metallurgy", MachineType: "titanium_foundry",
		Inputs: []RecipeIngredient{
			{ItemID: "titanium_ore", Quantity: 5},
			{ItemID: "coal", Quantity: 10},
		},
		Icon: "🧊",
	},
	"electrolysis_recipe": {
		ID: "electrolysis_recipe", Name: "Électrolyse de l'Eau",
		OutputItem: "hydrogen", OutputQuantity: 10, ProductionTime: 120,
		RequiredTech: "cryogenics", MachineType: "electrolysis_plant",
		Inputs: []RecipeIngredient{}, // Generates from energy + water (simplified)
		Icon:   "💧",
	},
	"rocket_fuel_recipe": {
		ID: "rocket_fuel_recipe", Name: "Synthèse de Carburant Spatial",
		OutputItem: "rocket_fuel", OutputQuantity: 5, ProductionTime: 300,
		RequiredTech: "cryogenics", MachineType: "chemical_plant",
		Inputs: []RecipeIngredient{
			{ItemID: "hydrogen", Quantity: 20},
			{ItemID: "crude_oil", Quantity: 10},
		},
		Icon: "🚀",
	},
	"heat_shield_recipe": {
		ID: "heat_shield_recipe", Name: "Fabrication Bouclier Thermique",
		OutputItem: "heat_shield", OutputQuantity: 1, ProductionTime: 600,
		RequiredTech: "titanium_metallurgy", MachineType: "titanium_foundry",
		Inputs: []RecipeIngredient{
			{ItemID: "titanium_ingot", Quantity: 10},
			{ItemID: "reinforced_steel", Quantity: 5},
		},
		Icon: "🛡️",
	},
	"guidance_system_recipe": {
		ID: "guidance_system_recipe", Name: "Système de Guidage Avancé",
		OutputItem: "guidance_system", OutputQuantity: 1, ProductionTime: 900,
		RequiredTech: "aerospace_engineering", MachineType: "hightech_factory",
		Inputs: []RecipeIngredient{
			{ItemID: "processor", Quantity: 5},
			{ItemID: "simple_circuit", Quantity: 20},
			{ItemID: "gold_ore", Quantity: 10},
		},
		Icon: "🎯",
	},
	"rocket_engine_recipe": {
		ID: "rocket_engine_recipe", Name: "Construction Moteur de Fusée",
		OutputItem: "rocket_engine", OutputQuantity: 1, ProductionTime: 1800,
		RequiredTech: "aerospace_engineering", MachineType: "aerospace_factory",
		Inputs: []RecipeIngredient{
			{ItemID: "titanium_ingot", Quantity: 50},
			{ItemID: "aluminum_ingot", Quantity: 100},
			{ItemID: "reinforced_steel", Quantity: 30},
			{ItemID: "electric_motor", Quantity: 5},
		},
		Icon: "🔥",
	},
	"satellite_recipe": {
		ID: "satellite_recipe", Name: "Assemblage Satellite",
		OutputItem: "satellite", OutputQuantity: 1, ProductionTime: 3600,
		RequiredTech: "aerospace_engineering", MachineType: "aerospace_factory",
		Inputs: []RecipeIngredient{
			{ItemID: "aluminum_ingot", Quantity: 50},
			{ItemID: "processor", Quantity: 10},
			{ItemID: "guidance_system", Quantity: 2},
			{ItemID: "battery_cell", Quantity: 20},
		},
		Icon: "🛰️",
	},
	"rocket_recipe": {
		ID: "rocket_recipe", Name: "Construction de Fusée",
		OutputItem: "rocket", OutputQuantity: 1, ProductionTime: 7200,
		RequiredTech: "rocket_science", MachineType: "rocket_launch_pad",
		Inputs: []RecipeIngredient{
			{ItemID: "rocket_engine", Quantity: 5},
			{ItemID: "heat_shield", Quantity: 20},
			{ItemID: "guidance_system", Quantity: 3},
			{ItemID: "rocket_fuel", Quantity: 1000},
			{ItemID: "titanium_ingot", Quantity: 200},
			{ItemID: "aluminum_ingot", Quantity: 500},
		},
		Icon: "🚀",
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
