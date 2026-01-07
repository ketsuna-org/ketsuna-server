package gamedata

// ItemType represents the category of an item
type ItemType string

const (
	ItemTypeRessourceBrute ItemType = "Ressource Brute"
	ItemTypeComposant      ItemType = "Composant"
	ItemTypeProduitFini    ItemType = "Produit Fini"
	ItemTypeMachine        ItemType = "Machine"
	ItemTypeStockage       ItemType = "Stockage"
)

// EnergyType represents the energy source type
type EnergyType string

const (
	EnergyTypeSoleil      EnergyType = "Soleil"
	EnergyTypeElectricite EnergyType = "Electricité"
	EnergyTypeFossile     EnergyType = "Fossile"
	EnergyTypeManuel      EnergyType = "Manuel"
)

// Item represents a static game item definition
type Item struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Type            ItemType   `json:"type"`
	BasePrice       float64    `json:"base_price"`
	Volatility      float64    `json:"volatility"`
	Product         string     `json:"product,omitempty"`          // For machines: what item they produce
	ProductQuantity int        `json:"product_quantity,omitempty"` // Quantity produced per cycle
	UseRecipe       string     `json:"use_recipe,omitempty"`       // Recipe ID for production
	ProductionTime  int        `json:"production_time,omitempty"`  // Seconds per production cycle
	MaxEmployee     int        `json:"max_employee,omitempty"`     // Max workers per machine
	CanStore        []string   `json:"can_store,omitempty"`        // Item IDs this storage can hold
	ProduceEnergy   float64    `json:"produce_energy,omitempty"`   // Energy produced per cycle
	CanConsume      []string   `json:"can_consume,omitempty"`      // Fuel item IDs
	CanStoreEnergy  float64    `json:"can_store_energy,omitempty"` // Battery capacity
	NeedEnergy      float64    `json:"need_energy,omitempty"`      // Energy required to operate
	EnergyType      EnergyType `json:"energy_type,omitempty"`
	Minable         bool       `json:"minable"`       // Can be harvested by CEO
	IsExplorable    bool       `json:"is_explorable"` // Can be found via exploration
	Icon            string     `json:"icon,omitempty"`
}

// =============================================================================
// STATIC ITEM DATABASE
// =============================================================================

// Items is the static database of all game items indexed by ID
var Items = map[string]Item{
	// -------------------------------------------------------------------------
	// RESSOURCES BRUTES - Matériaux de base
	// -------------------------------------------------------------------------
	"wood": {
		ID: "wood", Name: "Bois", Type: ItemTypeRessourceBrute,
		BasePrice: 2, Volatility: 0, Minable: true, IsExplorable: false, Icon: "🪵",
	},
	"stone": {
		ID: "stone", Name: "Pierre", Type: ItemTypeRessourceBrute,
		BasePrice: 3, Volatility: 0, Minable: true, IsExplorable: false, Icon: "🪨",
	},
	"silica": {
		ID: "silica", Name: "Silice (Sable)", Type: ItemTypeRessourceBrute,
		BasePrice: 6.63, Volatility: 0.10, Minable: true, IsExplorable: false, Icon: "🏜️",
	},
	"iron_ore": {
		ID: "iron_ore", Name: "Minerai de Fer", Type: ItemTypeRessourceBrute,
		BasePrice: 13.45, Volatility: 0.15, Minable: false, IsExplorable: true, Icon: "🔩",
	},
	"copper_ore": {
		ID: "copper_ore", Name: "Minerai de Cuivre", Type: ItemTypeRessourceBrute,
		BasePrice: 21.19, Volatility: 0.20, Minable: false, IsExplorable: true, Icon: "🟠",
	},
	"coal": {
		ID: "coal", Name: "Charbon", Type: ItemTypeRessourceBrute,
		BasePrice: 8.27, Volatility: 0.25, Minable: false, IsExplorable: true, Icon: "🪨",
	},
	"gold_ore": {
		ID: "gold_ore", Name: "Or Brut", Type: ItemTypeRessourceBrute,
		BasePrice: 72.86, Volatility: 0.40, Minable: false, IsExplorable: true, Icon: "💎",
	},
	"crude_oil": {
		ID: "crude_oil", Name: "Pétrole Brut", Type: ItemTypeRessourceBrute,
		BasePrice: 56.82, Volatility: 0.55, Minable: false, IsExplorable: true, Icon: "🛢️",
	},
	"lithium": {
		ID: "lithium", Name: "Lithium", Type: ItemTypeRessourceBrute,
		BasePrice: 16.94, Volatility: 0.60, Minable: false, IsExplorable: false, Icon: "🔋",
	},

	// -------------------------------------------------------------------------
	// COMPOSANTS - Matériaux transformés
	// -------------------------------------------------------------------------
	"wooden_plank": {
		ID: "wooden_plank", Name: "Planche de bois", Type: ItemTypeComposant,
		BasePrice: 15.45, Volatility: 0.10, Icon: "🪵",
	},
	"iron_ingot": {
		ID: "iron_ingot", Name: "Lingot de Fer", Type: ItemTypeComposant,
		BasePrice: 41.12, Volatility: 0.15, Icon: "🔩",
	},
	"copper_ingot": {
		ID: "copper_ingot", Name: "Lingot de Cuivre", Type: ItemTypeComposant,
		BasePrice: 55.83, Volatility: 0.18, Icon: "🟠",
	},
	"steel": {
		ID: "steel", Name: "Acier", Type: ItemTypeComposant,
		BasePrice: 98.21, Volatility: 0.12, Icon: "⬛",
	},
	"glass": {
		ID: "glass", Name: "Verre", Type: ItemTypeComposant,
		BasePrice: 42.42, Volatility: 0.15, Icon: "🪟",
	},
	"plastic": {
		ID: "plastic", Name: "Plastique", Type: ItemTypeComposant,
		BasePrice: 64.99, Volatility: 0.30, Icon: "🧱",
	},
	"electric_cable": {
		ID: "electric_cable", Name: "Câble Électrique", Type: ItemTypeComposant,
		BasePrice: 81.14, Volatility: 0.18, Icon: "🔌",
	},
	"gear": {
		ID: "gear", Name: "Engrenage", Type: ItemTypeComposant,
		BasePrice: 61.49, Volatility: 0.10, Icon: "⚙️",
	},
	"simple_circuit": {
		ID: "simple_circuit", Name: "Circuit Simple", Type: ItemTypeComposant,
		BasePrice: 268.83, Volatility: 0.35, Icon: "🔲",
	},
	"processor": {
		ID: "processor", Name: "Processeur", Type: ItemTypeComposant,
		BasePrice: 1560.83, Volatility: 0.50, Icon: "💻",
	},
	"battery_cell": {
		ID: "battery_cell", Name: "Cellule de Batterie", Type: ItemTypeComposant,
		BasePrice: 985.43, Volatility: 0.45, Icon: "🔋",
	},

	// -------------------------------------------------------------------------
	// PRODUITS FINIS
	// -------------------------------------------------------------------------
	"electric_motor": {
		ID: "electric_motor", Name: "Moteur Électrique", Type: ItemTypeProduitFini,
		BasePrice: 2500, Volatility: 0.25, Icon: "⚡",
	},
	"smartphone": {
		ID: "smartphone", Name: "Smartphone", Type: ItemTypeProduitFini,
		BasePrice: 8500, Volatility: 0.45, Icon: "📱",
	},
	"computer": {
		ID: "computer", Name: "Ordinateur", Type: ItemTypeProduitFini,
		BasePrice: 12000, Volatility: 0.40, Icon: "🖥️",
	},

	// -------------------------------------------------------------------------
	// MACHINES - Tier 1 (Niveau 1+)
	// -------------------------------------------------------------------------
	"forestry_machine": {
		ID: "forestry_machine", Name: "Exploitation Forestière", Type: ItemTypeMachine,
		BasePrice: 1042, Product: "wood", ProductQuantity: 2, ProductionTime: 120,
		MaxEmployee: 2, EnergyType: EnergyTypeManuel, Icon: "/icons/forestry_machine.png",
	},
	"basic_mining_machine": {
		ID: "basic_mining_machine", Name: "Extraction Minière de base", Type: ItemTypeMachine,
		BasePrice: 2876, Product: "iron_ore", ProductQuantity: 3, ProductionTime: 50,
		MaxEmployee: 2, EnergyType: EnergyTypeManuel, Icon: "/icons/basic_mining_machine.png",
	},
	"sawmill": {
		ID: "sawmill", Name: "Scierie", Type: ItemTypeMachine,
		BasePrice: 1500, UseRecipe: "wooden_plank_recipe", ProductionTime: 20,
		MaxEmployee: 2, EnergyType: EnergyTypeManuel, Icon: "/icons/sawmill.png",
	},
	"solar_panel": {
		ID: "solar_panel", Name: "Panneau Solaire", Type: ItemTypeMachine,
		BasePrice: 2500, ProduceEnergy: 10, EnergyType: EnergyTypeSoleil, Icon: "/icons/solar_panel.png",
	},
	"charcoal_mine": {
		ID: "charcoal_mine", Name: "Mine de Charbon", Type: ItemTypeMachine,
		BasePrice: 2500, ProduceEnergy: 10, EnergyType: EnergyTypeSoleil, Icon: "/icons/charcoal_mine.png",
	},

	// -------------------------------------------------------------------------
	// MACHINES - Tier 2 (Niveau 3+)
	// -------------------------------------------------------------------------
	"iron_foundry": {
		ID: "iron_foundry", Name: "Fonderie Simple", Type: ItemTypeMachine,
		BasePrice: 4108, UseRecipe: "iron_ingot_recipe", ProductionTime: 5,
		MaxEmployee: 2, NeedEnergy: 5, EnergyType: EnergyTypeElectricite, Icon: "/icons/furnace.png",
	},
	"copper_foundry": {
		ID: "copper_foundry", Name: "Fonderie Cuivre", Type: ItemTypeMachine,
		BasePrice: 4735, UseRecipe: "copper_ingot_recipe", ProductionTime: 5,
		MaxEmployee: 2, NeedEnergy: 5, EnergyType: EnergyTypeElectricite, Icon: "/icons/furnace.png",
	},
	"copper_extractor": {
		ID: "copper_extractor", Name: "Extraction Minière de Cuivre", Type: ItemTypeMachine,
		BasePrice: 20527, Product: "copper_ore", ProductQuantity: 3, ProductionTime: 60,
		MaxEmployee: 3, NeedEnergy: 8, EnergyType: EnergyTypeElectricite, Icon: "/icons/mining_extractor.png",
	},
	"iron_extractor": {
		ID: "iron_extractor", Name: "Extraction Minière de Fer", Type: ItemTypeMachine,
		BasePrice: 18234, Product: "iron_ore", ProductQuantity: 5, ProductionTime: 50,
		MaxEmployee: 3, NeedEnergy: 8, EnergyType: EnergyTypeElectricite, Icon: "/icons/mining_extractor.png",
	},

	// -------------------------------------------------------------------------
	// MACHINES - Tier 3 (Niveau 6+)
	// -------------------------------------------------------------------------
	"thermal_plant": {
		ID: "thermal_plant", Name: "Central Thermique", Type: ItemTypeMachine,
		BasePrice: 17928, ProduceEnergy: 100, CanConsume: []string{"coal"},
		EnergyType: EnergyTypeFossile, MaxEmployee: 4, Icon: "🔥",
	},
	"glass_furnace": {
		ID: "glass_furnace", Name: "Four à Verre", Type: ItemTypeMachine,
		BasePrice: 8500, UseRecipe: "glass_recipe", ProductionTime: 60,
		MaxEmployee: 2, NeedEnergy: 15, EnergyType: EnergyTypeElectricite, Icon: "🔥",
	},
	"steel_press": {
		ID: "steel_press", Name: "Presse à Acier", Type: ItemTypeMachine,
		BasePrice: 15000, UseRecipe: "steel_recipe", ProductionTime: 120,
		MaxEmployee: 3, NeedEnergy: 25, EnergyType: EnergyTypeElectricite, Icon: "⚙️",
	},

	// -------------------------------------------------------------------------
	// MACHINES - Tier 4 (Niveau 10+)
	// -------------------------------------------------------------------------
	"oil_refinery": {
		ID: "oil_refinery", Name: "Raffinerie", Type: ItemTypeMachine,
		BasePrice: 45000, UseRecipe: "plastic_recipe", ProductionTime: 120,
		MaxEmployee: 5, NeedEnergy: 50, EnergyType: EnergyTypeElectricite, Icon: "🏭",
	},
	"petrol_pumpjack": {
		ID: "petrol_pumpjack", Name: "Pompe à Pétrole", Type: ItemTypeMachine,
		BasePrice: 45000, UseRecipe: "crude_oil_recipe", ProductionTime: 120,
		MaxEmployee: 5, NeedEnergy: 50, EnergyType: EnergyTypeElectricite, Icon: "/icons/petrol_pumpjack.png",
	},
	"assembly_line": {
		ID: "assembly_line", Name: "Ligne d'Assemblage", Type: ItemTypeMachine,
		BasePrice: 85000, UseRecipe: "electric_motor_recipe", ProductionTime: 360,
		MaxEmployee: 8, NeedEnergy: 100, EnergyType: EnergyTypeElectricite, Icon: "🏭",
	},

	// -------------------------------------------------------------------------
	// MACHINES - Tier 5 (Niveau 15+)
	// -------------------------------------------------------------------------
	"hightech_factory": {
		ID: "hightech_factory", Name: "Usine High-Tech", Type: ItemTypeMachine,
		BasePrice: 250000, UseRecipe: "smartphone_recipe", ProductionTime: 600,
		MaxEmployee: 12, NeedEnergy: 200, EnergyType: EnergyTypeElectricite, Icon: "🏢",
	},
	"wind_turbine": {
		ID: "wind_turbine", Name: "Éolienne (Production de 4 MW)", Type: ItemTypeMachine,
		BasePrice: 250000, ProduceEnergy: 4, EnergyType: EnergyTypeElectricite, Icon: "🏢",
	},
}

// GetItem returns an item by ID, returns nil if not found
func GetItem(id string) *Item {
	if item, ok := Items[id]; ok {
		return &item
	}
	return nil
}

// GetItemName returns the name of an item, or "Unknown Item" if not found
func GetItemName(id string) string {
	if item := GetItem(id); item != nil {
		return item.Name
	}
	return "Unknown Item"
}

// GetAllItems returns a slice of all items
func GetAllItems() []Item {
	result := make([]Item, 0, len(Items))
	for _, item := range Items {
		result = append(result, item)
	}
	return result
}

// GetItemsByType returns all items of a specific type
func GetItemsByType(itemType ItemType) []Item {
	var result []Item
	for _, item := range Items {
		if item.Type == itemType {
			result = append(result, item)
		}
	}
	return result
}

// GetMarketItems returns items available in the market (excludes "Produit Fini")
func GetMarketItems() []Item {
	var result []Item
	for _, item := range Items {
		if item.Type != ItemTypeProduitFini {
			result = append(result, item)
		}
	}
	return result
}

// GetExplorableItems returns items that can be found via exploration
func GetExplorableItems() []Item {
	var result []Item
	for _, item := range Items {
		if item.IsExplorable {
			result = append(result, item)
		}
	}
	return result
}

// GetMinableItems returns items that can be harvested by CEO
func GetMinableItems() []Item {
	var result []Item
	for _, item := range Items {
		if item.Minable {
			result = append(result, item)
		}
	}
	return result
}

// GetItemByName returns an item by its display name, returns nil if not found
// This is used for reconciling database records (which have names) to gamedata IDs
func GetItemByName(name string) *Item {
	for _, item := range Items {
		if item.Name == name {
			return &item
		}
	}
	return nil
}
