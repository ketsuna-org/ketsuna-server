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
	StrategicTip  string         `json:"strategic_tip,omitempty"` // Strategic advice for wiki
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
		Description:  "Cette technologie marque la fin du travail purement manuel. Elle permet la construction de machines d'extraction autonomes pour le bois et la pierre.",
		StrategicTip: "C'est la toute première tech. Ne dépensez pas tout votre argent dans les machines tout de suite : gardez du cash pour payer vos premiers employés.",
		Cost:         5000, RequiredLevel: 1,
		UnlockTime:   300, // 5 minutes
		ItemUnlocked: []string{"forestry_machine", "basic_mining_machine", "sawmill", "wooden_plank", "sand_extractor", "quarry"},
		Icon:         "⚙️",
	},

	// -------------------------------------------------------------------------
	// TIER 2 - Ressources (Niveau 2-3)
	// -------------------------------------------------------------------------
	"basic_metallurgy": {
		ID: "basic_metallurgy", Name: "Métallurgie Fondamentale", Category: "resource",
		Description:  "L'art de purifier les minerais par le feu. Ouvre la voie à la fabrication d'outils en métal et de structures plus résistantes.",
		StrategicTip: "Débloque les fonderies. Préparez vos stocks de minerais (Fer, Cuivre) PENDANT la recherche pour lancer la production de lingots immédiatement après le déblocage.",
		Cost:         75000, RequiredLevel: 2,
		UnlockTime:    600, // 10 minutes
		Prerequisites: []string{"basic_automation"},
		ItemUnlocked:  []string{"iron_foundry", "copper_foundry", "iron_ingot", "copper_ingot", "iron_extractor", "copper_extractor"},
		Icon:          "🔥",
	},
	"advanced_mining": {
		ID: "advanced_mining", Name: "Extraction Avancée", Category: "resource",
		Description:  "Techniques d'extraction pour des minerais plus profonds.",
		StrategicTip: "Cette technologie est la clé pour accéder au charbon, carburant essentiel pour vos futures forges d'acier.",
		Cost:         120000, RequiredLevel: 3,
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
		Description:  "Transformer la silice en verre.",
		StrategicTip: "Le verre est requis pour le high-tech. Lancez cette recherche tôt si vous visez une montée en gamme rapide vers les smartphones.",
		Cost:         250000, RequiredLevel: 5,
		UnlockTime:    1200, // 20 minutes
		Prerequisites: []string{"basic_metallurgy"},
		ItemUnlocked:  []string{"glass_furnace", "glass"},
		Icon:          "🪟",
	},
	"gold_mining": {
		ID: "gold_mining", Name: "Extraction Aurifère", Category: "resource",
		Description:  "Techniques d'extraction spécialisées pour les gisements d'or.",
		StrategicTip: "L'or est votre ressource la plus précieuse avant l'aérospatiale. Protégez vos gisements et évitez de le vendre brut au marché.",
		Cost:         200000, RequiredLevel: 5,
		UnlockTime:    1200, // 20 minutes
		Prerequisites: []string{"basic_metallurgy"},
		ItemUnlocked:  []string{"gold_mine"},
		Icon:          "💎",
	},
	"steel_production": {
		ID: "steel_production", Name: "Forge d'Acier", Category: "industry",
		Description:  "Un procédé d'alliage avancé permettant de produire de l'acier, un matériau indispensable à l'industrie lourde et à la mécanique de précision.",
		StrategicTip: "L'acier est le premier grand 'mur' de ressource. Il faut 3 lingots de fer pour 1 acier. Assurez-vous d'avoir triplé votre production de fer avant de vous lancer.",
		Cost:         500000, RequiredLevel: 7,
		UnlockTime:    2700, // 45 minutes
		Prerequisites: []string{"advanced_mining"},
		ItemUnlocked:  []string{"steel_press", "steel", "gear"},
		Icon:          "⚙️",
	},
	"first_automatisation": {
		ID: "first_automatisation", Name: "Optimisation de l'Assemblage", Category: "industry",
		Description:  "Techniques avancées pour coordonner plusieurs machines d'assemblage et optimiser les flux de composants.",
		StrategicTip: "Cette technologie permet de mieux gérer vos premières lignes d'assemblage. Assurez-vous d'avoir un stock de fer et de cuivre constant pour alimenter vos nouveaux moteurs électriques.",
		Cost:         3000000, RequiredLevel: 10,
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
		Description:  "Raffiner le pétrole brut permet de créer des polymères synthétiques. Le plastique est léger, isolant et essentiel pour l'électronique.",
		StrategicTip: "Le pétrole est une ressource volatile. Si vous n'avez pas encore d'extraction propre, achetez du brut quand le cours est bas en prévision de cette recherche.",
		Cost:         3500000, RequiredLevel: 10,
		UnlockTime:    3600, // 1 heure
		Prerequisites: []string{"steel_production"},
		ItemUnlocked:  []string{"oil_refinery", "plastic", "crude_oil", "electric_cable", "petrol_pumpjack"},
		Icon:          "🛢️",
	},
	"lithium_extraction": {
		ID: "lithium_extraction", Name: "Extraction du Lithium", Category: "resource",
		Description:  "Extraire le lithium, composant essentiel des batteries modernes.",
		StrategicTip: "Primordial pour l'autonomie énergétique. Couplé au plastique, il vous permettra de dominer le marché des batteries.",
		Cost:         500000, RequiredLevel: 10,
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
		Description:  "Construire une immense plateforme pour extraire le pétrole en haute mer.",
		StrategicTip: "Une fois débloquée, cette technologie rend caduque l'extraction terrestre. Prévoyez un grand espace de stockage pour l'énorme débit.",
		Cost:         2000000, RequiredLevel: 12,
		UnlockTime:    43200, // 12 heures
		Prerequisites: []string{"plastic_era"},
		RequiredItems: []RequiredItem{{ItemID: "petrol_pumpjack", Quantity: 100}},
		ItemUnlocked:  []string{"oil_platform"},
		Icon:          "🏗️",
	},
	"advanced_steel_tech": {
		ID: "advanced_steel_tech", Name: "Acier Renforcé", Category: "industry",
		Description:  "Développer un acier de haute qualité pour les constructions avancées.",
		StrategicTip: "L'acier renforcé est le pont vers le Tier 5. Ne négligez pas cette recherche si vous voulez construire des usines high-tech.",
		Cost:         800000, RequiredLevel: 10,
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
		ID: "assembly_line_tech", Name: "Ligne d'assemblage", Category: "industry",
		Description:  "Assembler des composants complexes en produits finis.",
		StrategicTip: "La ligne d'assemblage multiplie vos possibilités. C'est ici que votre empire industriel commence à devenir véritablement complexe.",
		Cost:         25000000, RequiredLevel: 15,
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
		Description:  "La maîtrise du silicium et du cuivre permet la création de composants logiques. C'est l'aube de l'ère de l'information.",
		StrategicTip: "L'électronique consomme énormément de cuivre et de plastique. C'est le moment de revoir toute votre logistique pour éviter la congestion.",
		Cost:         20000000, RequiredLevel: 18,
		UnlockTime:    10800, // 3 heures
		Prerequisites: []string{"assembly_line_tech"},
		ItemUnlocked:  []string{"simple_circuit", "processor"},
		Icon:          "🔲",
	},
	"hightech_manufacturing": {
		ID: "hightech_manufacturing", Name: "Manufacture High-Tech", Category: "tech",
		Description:  "Produire des appareils électroniques avancés.",
		StrategicTip: "L'étape finale avant l'espace. Les smartphones et ordinateurs sont vos sources de revenus les plus stables à ce stade.",
		Cost:         50000000, RequiredLevel: 20,
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
		Description:  "Développer des matériaux composites et céramiques haute performance.",
		StrategicTip: "La fibre de carbone demande beaucoup de plastique. Assurez-vous que vos raffineries tournent à plein régime.",
		Cost:         25000000, RequiredLevel: 18,
		UnlockTime:    10800, // 3 heures
		Prerequisites: []string{"electronics"},
		ItemUnlocked:  []string{"carbon_fiber", "ceramic", "carbonization_furnace", "ceramic_kiln"},
		Icon:          "🖤",
	},
	"precision_engineering": {
		ID: "precision_engineering", Name: "Ingénierie de Précision", Category: "tech",
		Description:  "Techniques de fabrication de haute précision pour l'instrumentation.",
		StrategicTip: "Les capteurs sont indispensables pour les satellites. Ne négligez pas l'atelier de précision.",
		Cost:         40000000, RequiredLevel: 19,
		UnlockTime:    14400, // 4 heures
		Prerequisites: []string{"advanced_materials"},
		ItemUnlocked:  []string{"sensor", "precision_workshop", "hydraulic_cylinder", "hydraulic_press"},
		Icon:          "🔬",
	},
	"industrial_chemistry": {
		ID: "industrial_chemistry", Name: "Chimie Industrielle", Category: "industry",
		Description:  "Processus chimiques avancés pour les synthèses industrielles.",
		StrategicTip: "L'acide sulfurique est le catalyseur de toute votre industrie chimique avancée. Produisez-en massivement dès maintenant.",
		Cost:         35000000, RequiredLevel: 20,
		UnlockTime:    18000, // 5 heures
		Prerequisites: []string{"precision_engineering"},
		ItemUnlocked:  []string{"rubber", "rubber_factory", "sulfuric_acid", "nitrogen"},
		Icon:          "⚗️",
	},
	"advanced_metallurgy": {
		ID: "advanced_metallurgy", Name: "Métallurgie Avancée", Category: "industry",
		Description:  "Alliages spéciaux pour applications critiques.",
		StrategicTip: "Les alliages avancés sont requis pour les turbopompes. C'est l'un des composants les plus longs à fabriquer du Tier 6.",
		Cost:         50000000, RequiredLevel: 21,
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
		Description:  "Extraire et raffiner l'aluminium à partir de la bauxite.",
		StrategicTip: "L'aluminium est léger et indispensable pour l'aérospatiale. Il se traite rapidement, mais nécessite beaucoup de bauxite.",
		Cost:         20000000, RequiredLevel: 22,
		UnlockTime:    25200, // 7 heures
		Prerequisites: []string{"advanced_metallurgy"},
		ItemUnlocked:  []string{"aluminum_ore", "aluminum_ingot", "aluminum_foundry"},
		Icon:          "⚪",
	},
	"titanium_metallurgy": {
		ID: "titanium_metallurgy", Name: "Métallurgie du Titane", Category: "resource",
		Description:  "Techniques avancées pour travailler le titane, métal essentiel pour l'aérospatiale.",
		StrategicTip: "Le titane a une production lente. Améliorez vos mines de titane dès que possible pour ne pas ralentir votre programme spatial.",
		Cost:         80000000, RequiredLevel: 25,
		UnlockTime:    36000, // 10 heures
		Prerequisites: []string{"aluminum_processing"},
		ItemUnlocked:  []string{"titanium_ore", "titanium_ingot", "titanium_foundry", "titanium_mine", "heat_shield"},
		Icon:          "🧊",
	},
	"cryogenics": {
		ID: "cryogenics", Name: "Cryogénie Industrielle", Category: "industry",
		Description:  "Maîtriser les techniques de liquéfaction des gaz pour produire du carburant spatial.",
		StrategicTip: "La cryogénie demande une infrastructure électrique massive. Assurez-vous d'avoir assez d'énergie avant d'installer vos électrolyseurs.",
		Cost:         100000000, RequiredLevel: 26,
		UnlockTime:    43200, // 12 heures
		Prerequisites: []string{"titanium_metallurgy"},
		ItemUnlocked:  []string{"hydrogen", "oxygen", "electrolysis_plant", "chemical_plant", "rocket_fuel"},
		Icon:          "❄️",
	},
	"aerospace_engineering": {
		ID: "aerospace_engineering", Name: "Ingénierie Aérospatiale", Category: "tech",
		Description:  "Concevoir et construire des systèmes spatiaux avancés.",
		StrategicTip: "L'assemblage des moteurs de fusée est complexe. Centralisez vos stocks de turbopompes et d'alliages avancés près de vos usines aérospatiales.",
		Cost:         200000000, RequiredLevel: 28,
		UnlockTime:    64800, // 18 heures
		Prerequisites: []string{"cryogenics"},
		ItemUnlocked:  []string{"guidance_system", "rocket_engine", "aerospace_factory", "satellite"},
		Icon:          "🛰️",
	},
	"rocket_science": {
		ID: "rocket_science", Name: "Science des Fusées", Category: "tech",
		Description:  "L'ingénierie ultime. Combiner des milliers de composants dans un véhicule capable de quitter l'atmosphère terrestre.",
		StrategicTip: "C'est la dernière ligne droite. La recherche est longue (48h), profitez-en pour stocker les composants les plus chers (guidage, boucliers thermiques) en masse.",
		Cost:         500000000, RequiredLevel: 30,
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
