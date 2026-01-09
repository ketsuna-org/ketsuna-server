package gamedata

// RequiredItem represents an item requirement for unlocking a technology
type RequiredItem struct {
	ItemID   string `json:"item_id"`
	Quantity int    `json:"quantity"`
}

// Technology represents a researchable technology
type Technology struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	Cost          float64        `json:"cost"`           // Cost in money to unlock
	RequiredLevel int            `json:"required_level"` // Minimum company level
	UnlockTime    int            `json:"unlock_time"`    // Seconds to unlock (0 = instant)
	RequiredItems []RequiredItem `json:"required_items"` // Items consumed from inventory to unlock
	ItemUnlocked  []string       `json:"item_unlocked"`  // Item IDs unlocked by this tech
	Prerequisites []string       `json:"prerequisites"`  // Tech IDs required before this one
	Category      string         `json:"category"`       // general, resource, industry, tech
	Icon          string         `json:"icon,omitempty"`
}

// =============================================================================
// STATIC TECHNOLOGY DATABASE
// =============================================================================

var Technologies = map[string]Technology{
	// -------------------------------------------------------------------------
	// TIER 1 - Général (Niveau 1)
	// -------------------------------------------------------------------------
	"basic_automation": {
		ID: "basic_automation", Name: "Un début d'automatisation", Category: "general",
		Description: "Débloquer les premières machines pour automatiser la production.",
		Cost:        1000, RequiredLevel: 1,
		ItemUnlocked: []string{"forestry_machine", "basic_mining_machine", "sawmill", "wooden_plank"},
		Icon:         "⚙️",
	},
	"solar_power": {
		ID: "solar_power", Name: "Énergie Solaire", Category: "general",
		Description: "Exploiter l'énergie du soleil pour alimenter vos machines.",
		Cost:        2500, RequiredLevel: 1,
		ItemUnlocked: []string{"solar_panel"},
		Icon:         "☀️",
	},

	// -------------------------------------------------------------------------
	// TIER 2 - Ressources (Niveau 2-3)
	// -------------------------------------------------------------------------
	"basic_metallurgy": {
		ID: "basic_metallurgy", Name: "Métallurgie Fondamentale", Category: "resource",
		Description: "Transformer les minerais en lingots utilisables.",
		Cost:        20000, RequiredLevel: 2,
		Prerequisites: []string{"basic_automation"},
		ItemUnlocked:  []string{"iron_foundry", "copper_foundry", "iron_ingot", "copper_ingot", "iron_extractor", "copper_extractor"},
		Icon:          "🔥",
	},
	"advanced_mining": {
		ID: "advanced_mining", Name: "Extraction Avancée", Category: "resource",
		Description: "Techniques d'extraction pour des minerais plus profonds.",
		Cost:        35000, RequiredLevel: 3,
		Prerequisites: []string{"basic_metallurgy"},
		ItemUnlocked:  []string{"iron_ore", "copper_ore", "coal"},
		Icon:          "⛏️",
	},

	// -------------------------------------------------------------------------
	// TIER 3 - Industrie (Niveau 6)
	// -------------------------------------------------------------------------
	"thermal_power": {
		ID: "thermal_power", Name: "L'Éveil de la Dynamo", Category: "industry",
		Description: "Produire de l'électricité à partir du charbon.",
		Cost:        100000, RequiredLevel: 6,
		Prerequisites: []string{"advanced_mining"},
		ItemUnlocked:  []string{"thermal_plant"},
		Icon:          "🔥",
	},
	"glass_production": {
		ID: "glass_production", Name: "Verrerie Industrielle", Category: "industry",
		Description: "Transformer la silice en verre.",
		Cost:        75000, RequiredLevel: 5,
		Prerequisites: []string{"basic_metallurgy"},
		ItemUnlocked:  []string{"glass_furnace", "glass"},
		Icon:          "🪟",
	},
	"steel_production": {
		ID: "steel_production", Name: "Forge d'Acier", Category: "industry",
		Description: "Produire de l'acier à partir de lingots de fer.",
		Cost:        150000, RequiredLevel: 7,
		Prerequisites: []string{"thermal_power"},
		ItemUnlocked:  []string{"steel_press", "steel", "gear"},
		Icon:          "⚙️",
	},
	"first_automatisation": {
		ID: "first_automatisation", Name: "Un début d'automatisation.", Category: "industry",
		Description: "Un début d'automatisation.",
		Cost:        1000000, RequiredLevel: 10,
		Prerequisites: []string{"steel_production"},
		ItemUnlocked:  []string{"assembly_line", "electric_motor", "lithium", "battery_cell"},
		Icon:          "🏭",
	},

	// -------------------------------------------------------------------------
	// TIER 4 - Ère du Plastique (Niveau 10)
	// -------------------------------------------------------------------------
	"plastic_era": {
		ID: "plastic_era", Name: "Ère du Plastique", Category: "industry",
		Description: "Raffiner le pétrole en plastique, matériau révolutionnaire.",
		Cost:        1000000, RequiredLevel: 10,
		Prerequisites: []string{"steel_production"},
		ItemUnlocked:  []string{"oil_refinery", "plastic", "crude_oil", "electric_cable", "petrol_pumpjack"},
		Icon:          "🛢️",
	},

	// -------------------------------------------------------------------------
	// TIER 4.5 - Technologies Avancées avec Temps et Items Requis
	// -------------------------------------------------------------------------
	"oil_platform_tech": {
		ID: "oil_platform_tech", Name: "Plateforme Pétrolière Offshore", Category: "industry",
		Description: "Construire une immense plateforme pour extraire le pétrole en haute mer. Nécessite 100 pompes à pétrole.",
		Cost:        500000, RequiredLevel: 12,
		UnlockTime:    36000, // 10 heure
		Prerequisites: []string{"plastic_era"},
		RequiredItems: []RequiredItem{{ItemID: "petrol_pumpjack", Quantity: 100}},
		ItemUnlocked:  []string{"oil_platform"},
		Icon:          "🏗️",
	},
	"advanced_steel_tech": {
		ID: "advanced_steel_tech", Name: "Acier Renforcé", Category: "industry",
		Description: "Développer un acier de haute qualité pour les constructions avancées. Nécessite 50 aciers.",
		Cost:        250000, RequiredLevel: 10,
		UnlockTime:    1800, // 30 minutes
		Prerequisites: []string{"steel_production"},
		RequiredItems: []RequiredItem{{ItemID: "steel", Quantity: 50}},
		ItemUnlocked:  []string{"reinforced_steel"},
		Icon:          "⚙️",
	},

	// -------------------------------------------------------------------------
	// TIER 5 - Assemblage (Niveau 15)
	// -------------------------------------------------------------------------
	"assembly_line_tech": {
		ID: "assembly_line_tech", Name: "Ligne d'assemble de premier Niveau.", Category: "industry",
		Description: "Assembler des composants complexes en produits finis.",
		Cost:        10000000, RequiredLevel: 15,
		Prerequisites: []string{"plastic_era"},
		ItemUnlocked:  []string{"assembly_line", "electric_motor", "lithium", "battery_cell"},
		Icon:          "🏭",
	},

	// -------------------------------------------------------------------------
	// TIER 6 - High-Tech (Niveau 20)
	// -------------------------------------------------------------------------
	"electronics": {
		ID: "electronics", Name: "Électronique Avancée", Category: "tech",
		Description: "Fabriquer des circuits et processeurs.",
		Cost:        8000000, RequiredLevel: 18,
		Prerequisites: []string{"assembly_line_tech"},
		ItemUnlocked:  []string{"simple_circuit", "processor"},
		Icon:          "🔲",
	},
	"hightech_manufacturing": {
		ID: "hightech_manufacturing", Name: "Manufacture High-Tech", Category: "tech",
		Description: "Produire des appareils électroniques avancés.",
		Cost:        15000000, RequiredLevel: 20,
		Prerequisites: []string{"electronics"},
		ItemUnlocked:  []string{"hightech_factory", "smartphone", "computer"},
		Icon:          "📱",
	},
	"green_energy": {
		ID: "green_energy", Name: "Transition Industrielle (Énergie Verte)", Category: "tech",
		Description: "Sources d'énergie renouvelables avancées.",
		Cost:        15000000, RequiredLevel: 20,
		Prerequisites: []string{"electronics"},
		ItemUnlocked:  []string{}, // Future: wind turbines, advanced solar
		Icon:          "🌱",
	},
}

// GetTechnology returns a technology by ID, returns nil if not found
func GetTechnology(id string) *Technology {
	if tech, ok := Technologies[id]; ok {
		return &tech
	}
	return nil
}

// GetTechnologyName returns the name of a technology, or "Unknown Technology" if not found
func GetTechnologyName(id string) string {
	if tech := GetTechnology(id); tech != nil {
		return tech.Name
	}
	return "Unknown Technology"
}

// GetAllTechnologies returns a slice of all technologies
func GetAllTechnologies() []Technology {
	result := make([]Technology, 0, len(Technologies))
	for _, tech := range Technologies {
		result = append(result, tech)
	}
	return result
}

// GetTechnologiesByLevel returns technologies available at a specific level
func GetTechnologiesByLevel(level int) []Technology {
	var result []Technology
	for _, tech := range Technologies {
		if tech.RequiredLevel <= level {
			result = append(result, tech)
		}
	}
	return result
}

// GetTechnologiesByCategory returns technologies in a specific category
func GetTechnologiesByCategory(category string) []Technology {
	var result []Technology
	for _, tech := range Technologies {
		if tech.Category == category {
			result = append(result, tech)
		}
	}
	return result
}

// IsItemUnlockedByTech checks if an item ID is unlocked by any technology
func IsItemUnlockedByTech(itemId string) (bool, string) {
	for _, tech := range Technologies {
		for _, unlocked := range tech.ItemUnlocked {
			if unlocked == itemId {
				return true, tech.ID
			}
		}
	}
	return false, ""
}

// GetTechnologyByName returns a technology by its display name, returns nil if not found
// This is used for reconciling database records (which have names) to gamedata IDs
func GetTechnologyByName(name string) *Technology {
	for _, tech := range Technologies {
		if tech.Name == name {
			return &tech
		}
	}
	return nil
}
