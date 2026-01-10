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
		Cost:        5000, RequiredLevel: 1,
		UnlockTime:   300, // 5 minutes
		ItemUnlocked: []string{"forestry_machine", "basic_mining_machine", "sawmill", "wooden_plank", "sand_extractor", "quarry"},
		Icon:         "⚙️",
	},

	// -------------------------------------------------------------------------
	// TIER 2 - Ressources (Niveau 2-3)
	// -------------------------------------------------------------------------
	"basic_metallurgy": {
		ID: "basic_metallurgy", Name: "Métallurgie Fondamentale", Category: "resource",
		Description: "Transformer les minerais en lingots utilisables.",
		Cost:        75000, RequiredLevel: 2,
		UnlockTime:    600, // 10 minutes
		Prerequisites: []string{"basic_automation"},
		ItemUnlocked:  []string{"iron_foundry", "copper_foundry", "iron_ingot", "copper_ingot", "iron_extractor", "copper_extractor"},
		Icon:          "🔥",
	},
	"advanced_mining": {
		ID: "advanced_mining", Name: "Extraction Avancée", Category: "resource",
		Description: "Techniques d'extraction pour des minerais plus profonds.",
		Cost:        120000, RequiredLevel: 3,
		UnlockTime:    900, // 15 minutes
		Prerequisites: []string{"basic_metallurgy"},
		ItemUnlocked:  []string{"iron_ore", "copper_ore", "coal"},
		Icon:          "⛏️",
	},

	// -------------------------------------------------------------------------
	// TIER 3 - Industrie (Niveau 5-7)
	// -------------------------------------------------------------------------
	"glass_production": {
		ID: "glass_production", Name: "Verrerie Industrielle", Category: "industry",
		Description: "Transformer la silice en verre.",
		Cost:        250000, RequiredLevel: 5,
		UnlockTime:    1200, // 20 minutes
		Prerequisites: []string{"basic_metallurgy"},
		ItemUnlocked:  []string{"glass_furnace", "glass"},
		Icon:          "🪟",
	},
	"gold_mining": {
		ID: "gold_mining", Name: "Extraction Aurifère", Category: "resource",
		Description: "Techniques d'extraction spécialisées pour les gisements d'or.",
		Cost:        200000, RequiredLevel: 5,
		UnlockTime:    1200, // 20 minutes
		Prerequisites: []string{"basic_metallurgy"},
		ItemUnlocked:  []string{"gold_mine"},
		Icon:          "💎",
	},
	"steel_production": {
		ID: "steel_production", Name: "Forge d'Acier", Category: "industry",
		Description: "Produire de l'acier à partir de lingots de fer.",
		Cost:        500000, RequiredLevel: 7,
		UnlockTime:    2700, // 45 minutes
		Prerequisites: []string{"advanced_mining"},
		ItemUnlocked:  []string{"steel_press", "steel", "gear"},
		Icon:          "⚙️",
	},
	"first_automatisation": {
		ID: "first_automatisation", Name: "Un début d'automatisation.", Category: "industry",
		Description: "Un début d'automatisation.",
		Cost:        3000000, RequiredLevel: 10,
		UnlockTime:    3600, // 1 heure
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
		Cost:        3500000, RequiredLevel: 10,
		UnlockTime:    3600, // 1 heure
		Prerequisites: []string{"steel_production"},
		ItemUnlocked:  []string{"oil_refinery", "plastic", "crude_oil", "electric_cable", "petrol_pumpjack"},
		Icon:          "🛢️",
	},
	"lithium_extraction": {
		ID: "lithium_extraction", Name: "Extraction du Lithium", Category: "resource",
		Description: "Extraire le lithium, composant essentiel des batteries modernes.",
		Cost:        500000, RequiredLevel: 10,
		UnlockTime:    2700, // 45 minutes
		Prerequisites: []string{"plastic_era"},
		ItemUnlocked:  []string{"lithium_extractor"},
		Icon:          "🔋",
	},

	// -------------------------------------------------------------------------
	// TIER 4.5 - Technologies Avancées avec Temps et Items Requis
	// -------------------------------------------------------------------------
	"oil_platform_tech": {
		ID: "oil_platform_tech", Name: "Plateforme Pétrolière Offshore", Category: "industry",
		Description: "Construire une immense plateforme pour extraire le pétrole en haute mer. Nécessite 100 pompes à pétrole.",
		Cost:        2000000, RequiredLevel: 12,
		UnlockTime:    43200, // 12 heures
		Prerequisites: []string{"plastic_era"},
		RequiredItems: []RequiredItem{{ItemID: "petrol_pumpjack", Quantity: 100}},
		ItemUnlocked:  []string{"oil_platform"},
		Icon:          "🏗️",
	},
	"advanced_steel_tech": {
		ID: "advanced_steel_tech", Name: "Acier Renforcé", Category: "industry",
		Description: "Développer un acier de haute qualité pour les constructions avancées. Nécessite 50 aciers.",
		Cost:        800000, RequiredLevel: 10,
		UnlockTime:    5400, // 1h30
		Prerequisites: []string{"steel_production"},
		RequiredItems: []RequiredItem{{ItemID: "steel", Quantity: 50}},
		ItemUnlocked:  []string{"reinforced_steel"},
		Icon:          "⚙️",
	},

	// -------------------------------------------------------------------------
	// TIER 5 - Assemblage (Niveau 15)
	// -------------------------------------------------------------------------
	"assembly_line_tech": {
		ID: "assembly_line_tech", Name: "Ligne d'assemblage de premier Niveau.", Category: "industry",
		Description: "Assembler des composants complexes en produits finis.",
		Cost:        25000000, RequiredLevel: 15,
		UnlockTime:    7200, // 2 heures
		Prerequisites: []string{"plastic_era"},
		ItemUnlocked:  []string{"assembly_line", "electric_motor", "lithium", "battery_cell"},
		Icon:          "🏭",
	},

	// -------------------------------------------------------------------------
	// TIER 6 - High-Tech (Niveau 18-20)
	// -------------------------------------------------------------------------
	"electronics": {
		ID: "electronics", Name: "Électronique Avancée", Category: "tech",
		Description: "Fabriquer des circuits et processeurs.",
		Cost:        20000000, RequiredLevel: 18,
		UnlockTime:    10800, // 3 heures
		Prerequisites: []string{"assembly_line_tech"},
		ItemUnlocked:  []string{"simple_circuit", "processor"},
		Icon:          "🔲",
	},
	"hightech_manufacturing": {
		ID: "hightech_manufacturing", Name: "Manufacture High-Tech", Category: "tech",
		Description: "Produire des appareils électroniques avancés.",
		Cost:        50000000, RequiredLevel: 20,
		UnlockTime:    14400, // 4 heures
		Prerequisites: []string{"electronics"},
		ItemUnlocked:  []string{"hightech_factory", "smartphone", "computer"},
		Icon:          "📱",
	},

	// -------------------------------------------------------------------------
	// TIER 6.5 - Technologies Intermédiaires (Niveau 18-22)
	// -------------------------------------------------------------------------
	"advanced_materials": {
		ID: "advanced_materials", Name: "Matériaux Avancés", Category: "industry",
		Description: "Développer des matériaux composites et céramiques haute performance.",
		Cost:        25000000, RequiredLevel: 18,
		UnlockTime:    10800, // 3 heures
		Prerequisites: []string{"electronics"},
		ItemUnlocked:  []string{"carbon_fiber", "ceramic", "carbonization_furnace", "ceramic_kiln"},
		Icon:          "🖤",
	},
	"precision_engineering": {
		ID: "precision_engineering", Name: "Ingénierie de Précision", Category: "tech",
		Description: "Techniques de fabrication de haute précision pour l'instrumentation.",
		Cost:        40000000, RequiredLevel: 19,
		UnlockTime:    14400, // 4 heures
		Prerequisites: []string{"advanced_materials"},
		ItemUnlocked:  []string{"sensor", "precision_workshop", "hydraulic_cylinder", "hydraulic_press"},
		Icon:          "🔬",
	},
	"industrial_chemistry": {
		ID: "industrial_chemistry", Name: "Chimie Industrielle", Category: "industry",
		Description: "Processus chimiques avancés pour les synthèses industrielles.",
		Cost:        35000000, RequiredLevel: 20,
		UnlockTime:    18000, // 5 heures
		Prerequisites: []string{"precision_engineering"},
		ItemUnlocked:  []string{"rubber", "rubber_factory", "sulfuric_acid", "nitrogen"},
		Icon:          "⚗️",
	},
	"advanced_metallurgy": {
		ID: "advanced_metallurgy", Name: "Métallurgie Avancée", Category: "industry",
		Description: "Alliages spéciaux pour applications critiques.",
		Cost:        50000000, RequiredLevel: 21,
		UnlockTime:    21600, // 6 heures
		Prerequisites: []string{"industrial_chemistry"},
		ItemUnlocked:  []string{"advanced_alloy", "advanced_forge", "turbopump", "pressurized_tank"},
		Icon:          "⚙️",
	},

	// -------------------------------------------------------------------------
	// TIER 7 - Aerospace (Niveau 22-28)
	// -------------------------------------------------------------------------
	"aluminum_processing": {
		ID: "aluminum_processing", Name: "Traitement de l'Aluminium", Category: "resource",
		Description: "Extraire et raffiner l'aluminium à partir de la bauxite.",
		Cost:        20000000, RequiredLevel: 22,
		UnlockTime:    25200, // 7 heures
		Prerequisites: []string{"advanced_metallurgy"},
		ItemUnlocked:  []string{"aluminum_ore", "aluminum_ingot", "aluminum_foundry"},
		Icon:          "⚪",
	},
	"titanium_metallurgy": {
		ID: "titanium_metallurgy", Name: "Métallurgie du Titane", Category: "resource",
		Description: "Techniques avancées pour travailler le titane, métal essentiel pour l'aérospatiale.",
		Cost:        80000000, RequiredLevel: 25,
		UnlockTime:    36000, // 10 heures
		Prerequisites: []string{"aluminum_processing"},
		ItemUnlocked:  []string{"titanium_ore", "titanium_ingot", "titanium_foundry", "titanium_mine", "heat_shield"},
		Icon:          "🧊",
	},
	"cryogenics": {
		ID: "cryogenics", Name: "Cryogénie Industrielle", Category: "industry",
		Description: "Maîtriser les techniques de liquéfaction des gaz pour produire du carburant spatial.",
		Cost:        100000000, RequiredLevel: 26,
		UnlockTime:    43200, // 12 heures
		Prerequisites: []string{"titanium_metallurgy"},
		ItemUnlocked:  []string{"hydrogen", "oxygen", "electrolysis_plant", "chemical_plant", "rocket_fuel"},
		Icon:          "❄️",
	},
	"aerospace_engineering": {
		ID: "aerospace_engineering", Name: "Ingénierie Aérospatiale", Category: "tech",
		Description: "Concevoir et construire des systèmes spatiaux avancés.",
		Cost:        200000000, RequiredLevel: 28,
		UnlockTime:    64800, // 18 heures
		Prerequisites: []string{"cryogenics"},
		ItemUnlocked:  []string{"guidance_system", "rocket_engine", "aerospace_factory", "satellite"},
		Icon:          "🛰️",
	},
	"rocket_science": {
		ID: "rocket_science", Name: "Science des Fusées", Category: "tech",
		Description: "L'aboutissement de toute la recherche : construire et lancer des fusées !",
		Cost:        500000000, RequiredLevel: 30,
		UnlockTime:    172800, // 48 heures
		Prerequisites: []string{"aerospace_engineering"},
		RequiredItems: []RequiredItem{
			{ItemID: "rocket_engine", Quantity: 10},
			{ItemID: "satellite", Quantity: 5},
		},
		ItemUnlocked: []string{"rocket", "rocket_launch_pad"},
		Icon:         "🚀",
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
