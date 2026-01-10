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

// Unit represents the unit of measurement for an item
type Unit string

const (
	UnitKg   Unit = "kg"
	UnitL    Unit = "l"
	UnitUnit Unit = "u"
)

// EnergyType represents the energy source type
type EnergyType string

const (
	EnergyTypeManuel EnergyType = "Manuel"
)

// MachineMetadata contains machine-specific configuration
// Supports multi-recipe machines and durability tracking
type MachineMetadata struct {
	AvailableRecipes      []string   `json:"available_recipes,omitempty"`       // List of recipe IDs this machine can execute
	DefaultProduct        string     `json:"default_product,omitempty"`         // For extractors: direct product without recipe
	ProductQuantity       int        `json:"product_quantity,omitempty"`        // Quantity produced per cycle
	ProductionTime        int        `json:"production_time,omitempty"`         // Seconds per production cycle
	MaxEmployee           int        `json:"max_employee,omitempty"`            // Max workers assignable
	MaxMaintenance        int        `json:"max_maintenance,omitempty"`         // Max maintenance workers (default 1)
	NeedEnergy            float64    `json:"need_energy,omitempty"`             // Energy required to operate
	EnergyType            EnergyType `json:"energy_type,omitempty"`             // Energy source type
	DurabilityPerCycle    float64    `json:"durability_per_cycle,omitempty"`    // Durability loss per cycle (default 1)
	ProduceEnergy         float64    `json:"produce_energy,omitempty"`          // For generators: energy produced
	CanConsume            []string   `json:"can_consume,omitempty"`             // For generators: fuel item IDs
	SupportedStorageTypes []Unit     `json:"supported_storage_types,omitempty"` // "kg", "l", "u"
	CanStoreItems         []string   `json:"can_store_items,omitempty"`         // Specific items it can store (optional filter)
}

// Item represents a static game item definition
type Item struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Type            ItemType `json:"type"`
	BasePrice       float64  `json:"base_price"`
	Volatility      float64  `json:"volatility,omitempty"`
	Icon            string   `json:"icon,omitempty"`
	Unit            Unit     `json:"unit"`             // kg, l, u
	Minable         bool     `json:"minable"`          // Can be harvested by CEO
	IsExplorable    bool     `json:"is_explorable"`    // Can be found via exploration
	MarketAvailable bool     `json:"market_available"` // If true, can be bought on market
	// Machine-specific fields (deprecated, use Metadata)
	Product         string     `json:"product,omitempty"`
	ProductQuantity int        `json:"product_quantity,omitempty"`
	UseRecipe       string     `json:"use_recipe,omitempty"`
	ProductionTime  int        `json:"production_time,omitempty"`
	MaxEmployee     int        `json:"max_employee,omitempty"`
	CanStore        []string   `json:"can_store,omitempty"`
	ProduceEnergy   float64    `json:"produce_energy,omitempty"`
	CanConsume      []string   `json:"can_consume,omitempty"`
	CanStoreEnergy  float64    `json:"can_store_energy,omitempty"`
	NeedEnergy      float64    `json:"need_energy,omitempty"`
	EnergyType      EnergyType `json:"energy_type,omitempty"`
	// New: Structured metadata for machines
	Metadata *MachineMetadata `json:"metadata,omitempty"`
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
		ID: "wood", Name: "Bois", Type: ItemTypeRessourceBrute, Unit: UnitKg,
		BasePrice: 2, Volatility: 0, Minable: true, IsExplorable: false, MarketAvailable: true, Icon: "🪵",
	},
	"stone": {
		ID: "stone", Name: "Pierre", Type: ItemTypeRessourceBrute, Unit: UnitKg,
		BasePrice: 2.5, Volatility: 0, Minable: true, IsExplorable: false, MarketAvailable: true, Icon: "🪨",
	},
	"silica": {
		ID: "silica", Name: "Silice (Sable)", Type: ItemTypeRessourceBrute, Unit: UnitKg,
		BasePrice: 0.5, Volatility: 0.10, Minable: true, IsExplorable: false, MarketAvailable: true, Icon: "🏜️",
	},
	"iron_ore": {
		ID: "iron_ore", Name: "Minerai de Fer", Type: ItemTypeRessourceBrute, Unit: UnitKg,
		BasePrice: 15, Volatility: 0.15, Minable: false, IsExplorable: true, MarketAvailable: true, Icon: "🔩",
	},
	"copper_ore": {
		ID: "copper_ore", Name: "Minerai de Cuivre", Type: ItemTypeRessourceBrute, Unit: UnitKg,
		BasePrice: 15, Volatility: 0.20, Minable: false, IsExplorable: true, MarketAvailable: true, Icon: "🟠",
	},
	"coal": {
		ID: "coal", Name: "Charbon", Type: ItemTypeRessourceBrute, Unit: UnitKg,
		BasePrice: 8, Volatility: 0.25, Minable: false, IsExplorable: true, MarketAvailable: true, Icon: "🪨",
	},
	"gold_ore": {
		ID: "gold_ore", Name: "Or Brut", Type: ItemTypeRessourceBrute, Unit: UnitKg,
		BasePrice: 100, Volatility: 0.40, Minable: false, IsExplorable: true, Icon: "💎",
	},
	"crude_oil": {
		ID: "crude_oil", Name: "Pétrole Brut", Type: ItemTypeRessourceBrute, Unit: UnitL,
		BasePrice: 60, Volatility: 0.55, Minable: false, IsExplorable: true, Icon: "🛢️",
	},
	"lithium": {
		ID: "lithium", Name: "Lithium", Type: ItemTypeRessourceBrute, Unit: UnitKg,
		BasePrice: 20, Volatility: 0.60, Minable: false, IsExplorable: false, Icon: "🔋",
	},
	"titanium_ore": {
		ID: "titanium_ore", Name: "Minerai de Titane", Type: ItemTypeRessourceBrute, Unit: UnitKg,
		BasePrice: 200, Volatility: 0.35, Minable: false, IsExplorable: true, Icon: "🧊",
	},
	"aluminum_ore": {
		ID: "aluminum_ore", Name: "Bauxite (Minerai d'Aluminium)", Type: ItemTypeRessourceBrute, Unit: UnitKg,
		BasePrice: 25, Volatility: 0.20, Minable: false, IsExplorable: true, MarketAvailable: true, Icon: "⚪",
	},
	"hydrogen": {
		ID: "hydrogen", Name: "Hydrogène Liquide", Type: ItemTypeRessourceBrute, Unit: UnitL,
		BasePrice: 150, Volatility: 0.40, Minable: false, IsExplorable: false, Icon: "💧",
	},
	"oxygen": {
		ID: "oxygen", Name: "Oxygène Liquide", Type: ItemTypeRessourceBrute, Unit: UnitL,
		BasePrice: 50, Volatility: 0.25, Minable: false, IsExplorable: false, Icon: "🌬️",
	},

	// -------------------------------------------------------------------------
	// COMPOSANTS - Matériaux transformés
	// -------------------------------------------------------------------------
	"wooden_plank": {
		ID: "wooden_plank", Name: "Planche de bois", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 2.5, Volatility: 0.10, Icon: "🪵",
	},
	"iron_ingot": {
		ID: "iron_ingot", Name: "Lingot de Fer", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 30, Volatility: 0.15, Icon: "🔩",
	},
	"copper_ingot": {
		ID: "copper_ingot", Name: "Lingot de Cuivre", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 30, Volatility: 0.18, Icon: "🟠",
	},
	"steel": {
		ID: "steel", Name: "Acier", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 60, Volatility: 0.12, Icon: "⬛",
	},
	"glass": {
		ID: "glass", Name: "Verre", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 10, Volatility: 0.15, Icon: "🪟",
	},
	"plastic": {
		ID: "plastic", Name: "Plastique", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 80, Volatility: 0.30, Icon: "🧱",
	},
	"electric_cable": {
		ID: "electric_cable", Name: "Câble Électrique", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 50, Volatility: 0.18, Icon: "🔌",
	},
	"gear": {
		ID: "gear", Name: "Engrenage", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 50, Volatility: 0.10, Icon: "⚙️",
	},
	"simple_circuit": {
		ID: "simple_circuit", Name: "Circuit Simple", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 20, Volatility: 0.35, Icon: "🔲",
	},
	"processor": {
		ID: "processor", Name: "Processeur", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 1500, Volatility: 0.50, Icon: "💻",
	},
	"battery_cell": {
		ID: "battery_cell", Name: "Cellule de Batterie", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 500, Volatility: 0.45, Icon: "🔋",
	},

	// -------------------------------------------------------------------------
	// COMPOSANTS INTERMÉDIAIRES (Tier 4-5)
	// -------------------------------------------------------------------------
	"carbon_fiber": {
		ID: "carbon_fiber", Name: "Fibre de Carbone", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 200, Volatility: 0.30, Icon: "🖤",
	},
	"ceramic": {
		ID: "ceramic", Name: "Céramique Technique", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 150, Volatility: 0.20, Icon: "🏺",
	},
	"advanced_alloy": {
		ID: "advanced_alloy", Name: "Alliage Avancé", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 300, Volatility: 0.25, Icon: "⚙️",
	},
	"sensor": {
		ID: "sensor", Name: "Capteur Électronique", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 800, Volatility: 0.35, Icon: "📡",
	},
	"rubber": {
		ID: "rubber", Name: "Caoutchouc Synthétique", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 100, Volatility: 0.25, Icon: "⚫",
	},
	"hydraulic_cylinder": {
		ID: "hydraulic_cylinder", Name: "Vérin Hydraulique", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 1200, Volatility: 0.20, Icon: "🔧",
	},
	"turbopump": {
		ID: "turbopump", Name: "Turbopompe", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 8000, Volatility: 0.35, Icon: "🌀",
	},
	"pressurized_tank": {
		ID: "pressurized_tank", Name: "Réservoir Pressurisé", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 3000, Volatility: 0.25, Icon: "🛢️",
	},
	"nitrogen": {
		ID: "nitrogen", Name: "Azote Liquide", Type: ItemTypeRessourceBrute, Unit: UnitL,
		BasePrice: 30, Volatility: 0.15, Minable: false, IsExplorable: false, Icon: "❄️",
	},
	"sulfuric_acid": {
		ID: "sulfuric_acid", Name: "Acide Sulfurique", Type: ItemTypeComposant, Unit: UnitL,
		BasePrice: 80, Volatility: 0.20, Icon: "⚗️",
	},

	// -------------------------------------------------------------------------
	// PRODUITS FINIS
	// -------------------------------------------------------------------------
	"electric_motor": {
		ID: "electric_motor", Name: "Moteur Électrique", Type: ItemTypeProduitFini, Unit: UnitUnit,
		BasePrice: 2500, Volatility: 0.25, Icon: "⚡",
	},
	"smartphone": {
		ID: "smartphone", Name: "Smartphone", Type: ItemTypeProduitFini, Unit: UnitUnit,
		BasePrice: 8500, Volatility: 0.45, Icon: "📱",
	},
	"computer": {
		ID: "computer", Name: "Ordinateur", Type: ItemTypeProduitFini, Unit: UnitUnit,
		BasePrice: 12000, Volatility: 0.40, Icon: "🖥️",
	},
	// Aerospace Components
	"titanium_ingot": {
		ID: "titanium_ingot", Name: "Lingot de Titane", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 400, Volatility: 0.30, Icon: "🧊",
	},
	"aluminum_ingot": {
		ID: "aluminum_ingot", Name: "Lingot d'Aluminium", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 50, Volatility: 0.18, Icon: "⚪",
	},
	"rocket_fuel": {
		ID: "rocket_fuel", Name: "Carburant de Fusée (LOX/LH2)", Type: ItemTypeComposant, Unit: UnitL,
		BasePrice: 500, Volatility: 0.50, Icon: "🚀",
	},
	"heat_shield": {
		ID: "heat_shield", Name: "Bouclier Thermique", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 5000, Volatility: 0.25, Icon: "🛡️",
	},
	"guidance_system": {
		ID: "guidance_system", Name: "Système de Guidage", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 25000, Volatility: 0.40, Icon: "🎯",
	},
	"rocket_engine": {
		ID: "rocket_engine", Name: "Moteur de Fusée", Type: ItemTypeProduitFini, Unit: UnitUnit,
		BasePrice: 100000, Volatility: 0.35, Icon: "🔥",
	},
	"satellite": {
		ID: "satellite", Name: "Satellite", Type: ItemTypeProduitFini, Unit: UnitUnit,
		BasePrice: 500000, Volatility: 0.45, Icon: "🛠️",
	},
	"rocket": {
		ID: "rocket", Name: "Fusée", Type: ItemTypeProduitFini, Unit: UnitUnit,
		BasePrice: 5000000, Volatility: 0.60, Icon: "🚀",
	},

	// -------------------------------------------------------------------------
	// MACHINES - Tier 1 (Niveau 1+)
	// -------------------------------------------------------------------------
	"forestry_machine": {
		ID: "forestry_machine", Name: "Exploitation Forestière", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 1000, Product: "wood", ProductQuantity: 15, ProductionTime: 120,
		MaxEmployee: 2, EnergyType: EnergyTypeManuel, Icon: "/icons/forestry_machine.png",
		MarketAvailable: true,
	},
	"basic_mining_machine": {
		ID: "basic_mining_machine", Name: "Extraction Minière de base", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 2500, Product: "stone", ProductQuantity: 15, ProductionTime: 120,
		MaxEmployee: 2, EnergyType: EnergyTypeManuel, Icon: "/icons/basic_mining_machine.png",
		MarketAvailable: true,
	},
	"sawmill": {
		ID: "sawmill", Name: "Scierie", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 1500, UseRecipe: "wooden_plank_recipe", ProductionTime: 20,
		MaxEmployee: 2, EnergyType: EnergyTypeManuel, Icon: "/icons/sawmill.png",
		MarketAvailable: true,
	},
	"charcoal_mine": {
		ID: "charcoal_mine", Name: "Mine de Charbon", Type: ItemTypeMachine, Unit: UnitUnit,
		ProductionTime: 120, Product: "coal", ProductQuantity: 10,
		MaxEmployee: 2,
		BasePrice:   2500, EnergyType: EnergyTypeManuel, Icon: "/icons/charcoal_mine.png",
		MarketAvailable: true,
	},
	"sand_extractor": {
		ID: "sand_extractor", Name: "Extracteur de Sable", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 1500, Product: "silica", ProductQuantity: 20, ProductionTime: 120,
		MaxEmployee: 2, EnergyType: EnergyTypeManuel, Icon: "/icons/sand_extractor.png",
		MarketAvailable: true,
	},

	// -------------------------------------------------------------------------
	// MACHINES - Tier 2 (Niveau 3+)
	// -------------------------------------------------------------------------
	"iron_foundry": {
		ID: "iron_foundry", Name: "Fonderie Simple", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 5000, UseRecipe: "iron_ingot_recipe", ProductionTime: 40,
		MaxEmployee: 2, EnergyType: EnergyTypeManuel, Icon: "/icons/furnace.png",
		MarketAvailable: true,
	},
	"copper_foundry": {
		ID: "copper_foundry", Name: "Fonderie Cuivre", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 5000, UseRecipe: "copper_ingot_recipe", ProductionTime: 40,
		MaxEmployee: 2, EnergyType: EnergyTypeManuel, Icon: "/icons/furnace.png",
		MarketAvailable: true,
	},
	"copper_extractor": {
		ID: "copper_extractor", Name: "Extraction Minière de Cuivre", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 20000, Product: "copper_ore", ProductQuantity: 15, ProductionTime: 120,
		MaxEmployee: 3, EnergyType: EnergyTypeManuel, Icon: "/icons/mining_extractor.png",
		MarketAvailable: true,
	},
	"iron_extractor": {
		ID:              "iron_extractor",
		Name:            "Extraction Minière de Fer",
		Type:            ItemTypeMachine,
		Unit:            UnitUnit,
		BasePrice:       20000,
		Product:         "iron_ore",
		ProductQuantity: 15,
		ProductionTime:  120,
		MaxEmployee:     3,
		EnergyType:      EnergyTypeManuel,
		Icon:            "/icons/mining_extractor.png",
		MarketAvailable: true,
	},

	// -------------------------------------------------------------------------
	// MACHINES - Tier 3 (Niveau 6+)
	// -------------------------------------------------------------------------
	"glass_furnace": {
		ID: "glass_furnace", Name: "Four à Verre", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 8500, UseRecipe: "glass_recipe", ProductionTime: 60,
		MaxEmployee: 2, EnergyType: EnergyTypeManuel, Icon: "🔥",
		MarketAvailable: true,
	},
	"steel_press": {
		ID: "steel_press", Name: "Presse à Acier", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 15000, UseRecipe: "steel_recipe", ProductionTime: 120,
		MaxEmployee: 3, EnergyType: EnergyTypeManuel, Icon: "⚙️",
		MarketAvailable: true,
	},
	"gold_mine": {
		ID: "gold_mine", Name: "Mine d'Or", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 50000, Product: "gold_ore", ProductQuantity: 5, ProductionTime: 180,
		MaxEmployee: 3, EnergyType: EnergyTypeManuel, Icon: "/icons/gold_mine.png",
		MarketAvailable: true,
	},

	// -------------------------------------------------------------------------
	// MACHINES - Tier 4 (Niveau 10+)
	// -------------------------------------------------------------------------
	"oil_refinery": {
		ID: "oil_refinery", Name: "Raffinerie", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 45000, UseRecipe: "plastic_recipe", ProductionTime: 120,
		MaxEmployee: 5, EnergyType: EnergyTypeManuel, Icon: "🏭",
		MarketAvailable: true,
	},
	"petrol_pumpjack": {
		ID: "petrol_pumpjack", Name: "Pompe à Pétrole", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 45000, Product: "crude_oil", ProductQuantity: 15, ProductionTime: 120,
		MaxEmployee: 1, EnergyType: EnergyTypeManuel, Icon: "/icons/petrol_pumpjack.png",
		MarketAvailable: true,
	},
	"lithium_extractor": {
		ID: "lithium_extractor", Name: "Extracteur de Lithium", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 75000, Product: "lithium", ProductQuantity: 10, ProductionTime: 150,
		MaxEmployee: 3, EnergyType: EnergyTypeManuel, Icon: "/icons/lithium_extractor.png",
		MarketAvailable: true,
	},
	"assembly_line": {
		ID: "assembly_line", Name: "Ligne d'Assemblage", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 85000, UseRecipe: "electric_motor_recipe", ProductionTime: 360,
		MaxEmployee: 8, EnergyType: EnergyTypeManuel, Icon: "🏭",
	},
	"oil_platform": {
		ID: "oil_platform", Name: "Plateforme Pétrolière Offshore", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 500000, Product: "crude_oil", ProductQuantity: 2000, ProductionTime: 200,
		MaxEmployee: 10, EnergyType: EnergyTypeManuel, Icon: "🏗️",
		MarketAvailable: false,
	},
	"reinforced_steel": {
		ID: "reinforced_steel", Name: "Acier Renforcé", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 150, Volatility: 0.15, Icon: "🛡️",
		MarketAvailable: false,
	},

	// -------------------------------------------------------------------------
	// MACHINES - Tier 5 (Niveau 15+)
	// -------------------------------------------------------------------------
	"hightech_factory": {
		ID: "hightech_factory", Name: "Usine High-Tech", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 250000, UseRecipe: "smartphone_recipe", ProductionTime: 600,
		MaxEmployee: 12, NeedEnergy: 200, EnergyType: EnergyTypeManuel, Icon: "🏢",
	},

	// -------------------------------------------------------------------------
	// MACHINES - Tier 5.5 Intermédiaire (Niveau 18-22)
	// -------------------------------------------------------------------------
	"carbonization_furnace": {
		ID: "carbonization_furnace", Name: "Four de Carbonisation", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 120000, UseRecipe: "carbon_fiber_recipe", ProductionTime: 180,
		MaxEmployee: 4, NeedEnergy: 80, EnergyType: EnergyTypeManuel, Icon: "🖤",
	},
	"ceramic_kiln": {
		ID: "ceramic_kiln", Name: "Four à Céramique", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 80000, UseRecipe: "ceramic_recipe", ProductionTime: 120,
		MaxEmployee: 3, NeedEnergy: 60, EnergyType: EnergyTypeManuel, Icon: "🏺",
	},
	"precision_workshop": {
		ID: "precision_workshop", Name: "Atelier de Précision", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 200000, UseRecipe: "sensor_recipe", ProductionTime: 240,
		MaxEmployee: 6, NeedEnergy: 100, EnergyType: EnergyTypeManuel, Icon: "🔬",
	},
	"rubber_factory": {
		ID: "rubber_factory", Name: "Usine de Caoutchouc", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 90000, UseRecipe: "rubber_recipe", ProductionTime: 90,
		MaxEmployee: 4, NeedEnergy: 50, EnergyType: EnergyTypeManuel, Icon: "⚫", MarketAvailable: true,
	},
	"hydraulic_press": {
		ID: "hydraulic_press", Name: "Presse Hydraulique", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 150000, UseRecipe: "hydraulic_cylinder_recipe", ProductionTime: 150,
		MaxEmployee: 4, NeedEnergy: 80, EnergyType: EnergyTypeManuel, Icon: "🔧",
	},
	"advanced_forge": {
		ID: "advanced_forge", Name: "Forge Avancée", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 250000, UseRecipe: "advanced_alloy_recipe", ProductionTime: 200,
		MaxEmployee: 5, NeedEnergy: 120, EnergyType: EnergyTypeManuel, Icon: "🔥",
	},

	// -------------------------------------------------------------------------
	// MACHINES - Tier 6 Aerospace (Niveau 25+)
	// -------------------------------------------------------------------------
	"chemical_plant": {
		ID: "chemical_plant", Name: "Usine Chimique", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 500000, UseRecipe: "rocket_fuel_recipe", ProductionTime: 300,
		MaxEmployee: 6, NeedEnergy: 150, EnergyType: EnergyTypeManuel, Icon: "⚗️",
	},
	"titanium_foundry": {
		ID: "titanium_foundry", Name: "Fonderie de Titane", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 350000, UseRecipe: "titanium_ingot_recipe", ProductionTime: 180,
		MaxEmployee: 4, NeedEnergy: 100, EnergyType: EnergyTypeManuel, Icon: "🔥",
	},
	"aluminum_foundry": {
		ID: "aluminum_foundry", Name: "Fonderie d'Aluminium", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 150000, UseRecipe: "aluminum_ingot_recipe", ProductionTime: 60,
		MaxEmployee: 3, EnergyType: EnergyTypeManuel, Icon: "🔥", MarketAvailable: true,
	},
	"titanium_mine": {
		ID: "titanium_mine", Name: "Mine de Titane", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 400000, Product: "titanium_ore", ProductQuantity: 5, ProductionTime: 300,
		MaxEmployee: 5, EnergyType: EnergyTypeManuel, Icon: "⛏️",
	},
	"electrolysis_plant": {
		ID: "electrolysis_plant", Name: "Station d'Électrolyse", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 300000, UseRecipe: "electrolysis_recipe", ProductionTime: 120,
		MaxEmployee: 4, NeedEnergy: 200, EnergyType: EnergyTypeManuel, Icon: "⚡",
	},
	"aerospace_factory": {
		ID: "aerospace_factory", Name: "Usine Aérospatiale", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 2000000, UseRecipe: "rocket_engine_recipe", ProductionTime: 1800,
		MaxEmployee: 20, NeedEnergy: 500, EnergyType: EnergyTypeManuel, Icon: "🏭",
	},
	"rocket_launch_pad": {
		ID: "rocket_launch_pad", Name: "Pas de Tir", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 50000000, UseRecipe: "rocket_recipe", ProductionTime: 7200,
		MaxEmployee: 50, NeedEnergy: 1000, EnergyType: EnergyTypeManuel, Icon: "🚀",
	},
	// -------------------------------------------------------------------------
	// STOCKAGE
	// -------------------------------------------------------------------------
	"warehouse_small": {
		ID: "warehouse_small", Name: "Petit Entrepôt", Type: ItemTypeStockage, Unit: UnitUnit,
		BasePrice: 5000, Icon: "📦", MarketAvailable: true,
		Metadata: &MachineMetadata{
			SupportedStorageTypes: []Unit{UnitKg, UnitUnit},
		},
	},
	"fluid_tank_small": {
		ID: "fluid_tank_small", Name: "Citerne Standard", Type: ItemTypeStockage, Unit: UnitUnit,
		BasePrice: 7500, Icon: "🛢️", MarketAvailable: true,
		Metadata: &MachineMetadata{
			SupportedStorageTypes: []Unit{UnitL},
		},
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

// GetMarketItems returns items available for purchase on the market
func GetMarketItems() []Item {
	var result []Item
	for _, item := range Items {
		if item.MarketAvailable {
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
