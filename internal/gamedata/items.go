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
	Icon            string   `json:"icon,omitempty"`
	Unit            Unit     `json:"unit"`             // kg, l, u
	Minable         bool     `json:"minable"`          // Can be harvested by CEO
	IsExplorable    bool     `json:"is_explorable"`    // Can be found via exploration
	MarketAvailable bool     `json:"market_available"` // If true, can be bought on market
	// Wiki enrichment fields
	Description  string `json:"description,omitempty"`   // Detailed description for wiki
	StrategicTip string `json:"strategic_tip,omitempty"` // Strategic advice for wiki
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
		BasePrice: 2, Minable: true, IsExplorable: false, MarketAvailable: true, Icon: "🪵",
		Description:  "Le bois est la ressource de base par excellence. Facile à récolter manuellement au début, il devient rapidement un pilier de votre économie grâce à son utilisation dans la fabrication de planches et la construction de machines de base.",
		StrategicTip: "Au démarrage, exploitez le bois manuellement pour accumuler du capital initial. Dès que possible, investissez dans une Exploitation Forestière pour automatiser la production et libérer votre temps pour des tâches plus lucratives.",
	},
	"stone": {
		ID: "stone", Name: "Pierre", Type: ItemTypeRessourceBrute, Unit: UnitKg,
		BasePrice: 2.5, Minable: true, IsExplorable: false, MarketAvailable: true, Icon: "🪨",
		Description:  "La pierre brute est un matériau fondamental dans la construction et la fabrication de base. Abondante et facile à extraire, elle constitue la base de nombreux procédés industriels primitifs.",
		StrategicTip: "Bien que peu rentable à la revente, la pierre est essentielle pour certaines recettes de base comme la céramique. Stockez-en une quantité modérée pour éviter les ruptures de production.",
	},
	"silica": {
		ID: "silica", Name: "Silice (Sable)", Type: ItemTypeRessourceBrute, Unit: UnitKg,
		BasePrice: 0.5, Minable: true, IsExplorable: false, MarketAvailable: true, Icon: "🏜️",
		Description:  "La silice, ou sable industriel, est le composant principal du verre et de la céramique technique. Son prix bas cache une valeur stratégique considérable dans les chaînes de production avancées.",
		StrategicTip: "Investissez tôt dans un Extracteur de Sable car la production de verre consomme des quantités massives de silice. Une usine high-tech peut facilement épuiser vos stocks.",
	},
	"iron_ore": {
		ID: "iron_ore", Name: "Minerai de Fer", Type: ItemTypeRessourceBrute, Unit: UnitKg,
		BasePrice: 15, Minable: false, IsExplorable: true, MarketAvailable: true, Icon: "🔩",
		Description:  "Le minerai de fer est la colonne vertébrale de l'industrie lourde. Une fois fondu en lingots, il permet la fabrication d'acier et d'innombrables composants mécaniques essentiels.",
		StrategicTip: "Établissez une ligne de production Fer → Lingot → Acier dès que possible. L'acier est utilisé dans presque toutes les machines avancées et constitue un excellent produit d'exportation.",
	},
	"copper_ore": {
		ID: "copper_ore", Name: "Minerai de Cuivre", Type: ItemTypeRessourceBrute, Unit: UnitKg,
		BasePrice: 15, Minable: false, IsExplorable: true, MarketAvailable: true, Icon: "🟠",
		Description:  "Le cuivre est indispensable pour l'électronique et les câblages. Sa conductivité exceptionnelle en fait un composant clé dans la fabrication de circuits et de moteurs électriques.",
		StrategicTip: "Le cuivre devient critique dès l'entrée dans l'ère de l'électronique. Préparez vos gisements à l'avance car la demande explose avec les circuits simples et les câbles électriques.",
	},
	"coal": {
		ID: "coal", Name: "Charbon", Type: ItemTypeRessourceBrute, Unit: UnitKg,
		BasePrice: 8, Minable: false, IsExplorable: true, MarketAvailable: true, Icon: "🪨",
		Description:  "Le charbon est une source d'énergie fossile et un composant essentiel pour la métallurgie avancée. Il est particulièrement crucial pour la fonte du titane et la production de fibre de carbone.",
		StrategicTip: "Conservez des réserves conséquentes de charbon. Même si sa valeur marchande est modeste, les recettes aérospatiales en consomment des quantités industrielles.",
	},
	"gold_ore": {
		ID: "gold_ore", Name: "Or Brut", Type: ItemTypeRessourceBrute, Unit: UnitKg,
		BasePrice: 100, Minable: false, IsExplorable: true, Icon: "💎",
		Description:  "L'or brut est un métal précieux aux propriétés conductrices exceptionnelles. Il est essentiel pour la fabrication de processeurs haute performance et de systèmes de guidage de précision.",
		StrategicTip: "L'or est volatile mais très rentable. Surveillez le marché pour vendre aux pics de prix, ou stockez-le pour vos propres productions électroniques haut de gamme.",
	},
	"crude_oil": {
		ID: "crude_oil", Name: "Pétrole Brut", Type: ItemTypeRessourceBrute, Unit: UnitL,
		BasePrice: 60, Minable: false, IsExplorable: true, Icon: "🛢️",
		Description:  "Le pétrole brut est la ressource noire qui alimente la révolution industrielle moderne. Raffiné en plastique, il devient le cœur de toute production manufacturière avancée.",
		StrategicTip: "Le pétrole est extrêmement volatile. Construisez des plateformes offshore pour sécuriser un approvisionnement stable et ne dépendre jamais des fluctuations du marché mondial.",
	},
	"lithium": {
		ID: "lithium", Name: "Lithium", Type: ItemTypeRessourceBrute, Unit: UnitKg,
		BasePrice: 20, Minable: false, IsExplorable: false, Icon: "🔋",
		Description:  "Le lithium est le métal léger révolutionnaire qui rend possible le stockage d'énergie moderne. Indispensable pour les batteries, il est au cœur de la transition énergétique.",
		StrategicTip: "Le lithium ne peut pas être exploré - il doit être extrait via des machines spécialisées. Débloquez la technologie d'extraction du lithium dès niveau 10 pour préparer l'ère des batteries.",
	},
	"titanium_ore": {
		ID: "titanium_ore", Name: "Minerai de Titane", Type: ItemTypeRessourceBrute, Unit: UnitKg,
		BasePrice: 200, Minable: false, IsExplorable: true, Icon: "🧊",
		Description:  "Le titane est le métal de l'aérospatiale par excellence. Son rapport résistance/poids inégalé en fait le matériau privilégié pour les moteurs de fusée et les boucliers thermiques.",
		StrategicTip: "Le titane est rare et son extraction coûteuse. Centralisez vos mines de titane et optimisez le rendement de chaque gisement découvert lors des expéditions.",
	},
	"aluminum_ore": {
		ID: "aluminum_ore", Name: "Bauxite (Minerai d'Aluminium)", Type: ItemTypeRessourceBrute, Unit: UnitKg,
		BasePrice: 25, Minable: false, IsExplorable: true, MarketAvailable: true, Icon: "⚪",
		Description:  "La bauxite est le minerai dont on extrait l'aluminium, métal léger essentiel pour l'aviation et l'aérospatiale. Son traitement requiert d'importantes quantités d'énergie.",
		StrategicTip: "L'aluminium est consommé en masse dans la construction de satellites et de fusées. Établissez une chaîne d'approvisionnement robuste avant d'entrer dans l'ère aérospatiale.",
	},
	"hydrogen": {
		ID: "hydrogen", Name: "Hydrogène Liquide", Type: ItemTypeRessourceBrute, Unit: UnitL,
		BasePrice: 150, Minable: false, IsExplorable: false, Icon: "💧",
		Description:  "L'hydrogène liquide est le carburant spatial par excellence. Produit par électrolyse, il est stocké à des températures cryogéniques extrêmes (-253°C).",
		StrategicTip: "L'hydrogène est le composant principal du carburant de fusée. Construisez plusieurs stations d'électrolyse car la synthèse de carburant spatial en consomme des quantités astronomiques.",
	},
	"oxygen": {
		ID: "oxygen", Name: "Oxygène Liquide", Type: ItemTypeRessourceBrute, Unit: UnitL,
		BasePrice: 50, Minable: false, IsExplorable: false, Icon: "🌬️",
		Description:  "L'oxygène liquide (LOX) est le comburant des moteurs de fusée. Combiné à l'hydrogène, il produit une poussée phénoménale capable de vaincre la gravité terrestre.",
		StrategicTip: "L'oxygène est un sous-produit naturel de l'électrolyse. Votre production d'hydrogène générera automatiquement de l'oxygène - planifiez le stockage des deux en parallèle.",
	},

	// -------------------------------------------------------------------------
	// COMPOSANTS - Matériaux transformés
	// -------------------------------------------------------------------------
	"wooden_plank": {
		ID: "wooden_plank", Name: "Planche de bois", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 2.5, Icon: "🪵",
		Description:  "Des planches standardisées, obtenues par le sciage de grumes brutes. C'est le premier matériau de construction élaboré, nécessaire pour les structures simples.",
		StrategicTip: "Ne sous-estimez pas le bois ! Même à haut niveau, les planches restent nécessaires pour certaines maintenances et constructions de base. Gardez une scierie active.",
	},
	"iron_ingot": {
		ID: "iron_ingot", Name: "Lingot de Fer", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 30, Icon: "🔩",
		Description:  "Du fer purifié et coulé en lingots standards. C'est le précurseur indispensable de l'acier et de la plupart des machines industrielles.",
		StrategicTip: "La demande en lingots de fer est constante. Si vous avez un excédent, c'est une valeur refuge sûre à vendre au marché pour un revenu régulier.",
	},
	"copper_ingot": {
		ID: "copper_ingot", Name: "Lingot de Cuivre", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 30, Icon: "🟠",
		Description:  "Le métal rouge, raffiné pour une conductivité optimale. Il est principalement transformé en câbles et circuits électroniques.",
		StrategicTip: "Surveillez les ratios : 2 minerais pour 1 lingot. Assurez-vous que votre extraction suit la cadence de vos fonderies, surtout avant de lancer la production de câbles.",
	},
	"steel": {
		ID: "steel", Name: "Acier", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 60, Icon: "⬛",
		Description:  "Un alliage fer-carbone robuste. L'acier est le matériau de construction par défaut pour toute infrastructure sérieuse et les machineries lourdes.",
		StrategicTip: "Produire de l'acier prend du temps (120s). Il vaut souvent mieux multiplier les Presses à Acier en parallèle plutôt que de tenter d'accélérer une seule ligne.",
	},
	"glass": {
		ID: "glass", Name: "Verre", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 10, Icon: "🪟",
		Description:  "Matériau transparent obtenu par la fusion de la silice. Utilisé pour les écrans, les optiques de précision et les isolations.",
		StrategicTip: "Le verre est fragile et peu cher, mais indispensable pour les Smartphones. Stockez-le près de vos usines High-Tech pour minimiser les délais.",
	},
	"plastic": {
		ID: "plastic", Name: "Plastique", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 80, Icon: "🧱",
		Description:  "Polymère synthétique dérivé du pétrole. Léger, isolant et moulable à l'infini, c'est le matériau roi de l'ère de la consommation de masse.",
		StrategicTip: "Le plastique débloque l'électronique. C'est souvent le premier goulot d'étranglement majeur. Assurez-vous d'avoir une raffinerie dédiée uniquement à sa production.",
	},
	"electric_cable": {
		ID: "electric_cable", Name: "Câble Électrique", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 50, Icon: "🔌",
		Description:  "Conducteur en cuivre isolé par une gaine plastique. Le système nerveux de toute installation électrique.",
		StrategicTip: "Un composant 'volumineux' en termes de quantité requise. Les chaînes d'assemblage les consomment par paquets. Prévoyez large.",
	},
	"gear": {
		ID: "gear", Name: "Engrenage", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 50, Icon: "⚙️",
		Description:  "Pièce mécanique de précision en acier. Indispensable pour transmettre le mouvement dans les machines complexes et les moteurs.",
		StrategicTip: "Les engrenages sont la base de l'automatisation mécanique. Ils sont requis pour les turbopompes, ne négligez pas leur stock.",
	},
	"simple_circuit": {
		ID: "simple_circuit", Name: "Circuit Simple", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 20, Icon: "🔲",
		Description:  "PCB basique intégrant des composants discrets. La première étape vers l'intelligence artificielle et le contrôle numérique.",
		StrategicTip: "Extrêmement demandés pour les Processeurs (x5 pour 1). Votre usine de circuits simples sera probablement la plus sollicitée de votre parc.",
	},
	"processor": {
		ID: "processor", Name: "Processeur", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 1500, Icon: "💻",
		Description:  "Unité de calcul avancée gravée sur silicium avec des contacts en or. Le cerveau des ordinateurs et des systèmes de guidage spatiaux.",
		StrategicTip: "Chaque processeur contient de l'Or et beaucoup de valeur ajoutée. C'est l'un des items les plus denses en valeur par unité de transport.",
	},
	"battery_cell": {
		ID: "battery_cell", Name: "Cellule de Batterie", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 500, Icon: "🔋",
		Description:  "Unité de stockage d'énergie chimique à haute densité basée sur la technologie Lithium-Ion.",
		StrategicTip: "Essentielles pour les Satellites. La production de batteries nécessite une chaîne logistique complexe (Lithium + Plastique). Anticipez les pénuries de Lithium.",
	},

	// -------------------------------------------------------------------------
	// COMPOSANTS INTERMÉDIAIRES (Tier 4-5)
	// -------------------------------------------------------------------------
	"carbon_fiber": {
		ID: "carbon_fiber", Name: "Fibre de Carbone", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 200, Icon: "🖤",
		Description:  "Matériau composite ultra-léger et extrêmement résistant. La fibre de carbone est indispensable pour les structures aérospatiales et les équipements de haute technologie.",
		StrategicTip: "Produite à partir de plastique et de charbon. C'est un composant coûteux, optimisez vos stocks car sa production est lente dans les fours de carbonisation.",
	},
	"ceramic": {
		ID: "ceramic", Name: "Céramique Technique", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 150, Icon: "🏺",
		Description:  "Céramique de haute performance capable de résister à des températures et des pressions extrêmes. Utilisée dans les moteurs et les isolations thermiques.",
		StrategicTip: "Nécessite de la silice et de la pierre. C'est une ressource stable qui sert de base à de nombreux composants de précision.",
	},
	"advanced_alloy": {
		ID: "advanced_alloy", Name: "Alliage Avancé", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 300, Icon: "⚙️",
		Description:  "Mélange complexe de métaux conçu pour des propriétés spécifiques. Plus résistant que l'acier, il est le cœur des turbopompes.",
		StrategicTip: "Consomme de l'acier, du fer et du cuivre. Prévoyez une forge avancée dédiée car ce composant est un goulet d'étranglement pour l'aérospatiale.",
	},
	"sensor": {
		ID: "sensor", Name: "Capteur Électronique", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 800, Icon: "📡",
		Description:  "Dispositif de détection de haute précision. Capable de mesurer des variations infimes, essentiel pour les systèmes de guidage.",
		StrategicTip: "Indispensable pour les systèmes de guidage. Produisez-les en masse dans vos ateliers de précision une fois l'électronique débloquée.",
	},
	"rubber": {
		ID: "rubber", Name: "Caoutchouc Synthétique", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 100, Icon: "⚫",
		Description:  "Matériau élastique produit par polymérisation. Utilisé pour les joints d'étanchéité et les composants hydrauliques.",
		StrategicTip: "Dépend directement du pétrole brut. Une rupture de stock en caoutchouc peut paralyser la production de vos vérins hydrauliques.",
	},
	"hydraulic_cylinder": {
		ID: "hydraulic_cylinder", Name: "Vérin Hydraulique", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 1200, Icon: "🔧",
		Description:  "Actionneur mécanique utilisant la pression hydraulique. Crucial pour les mouvements de précision et les presses industrielles.",
		StrategicTip: "Combine de l'acier, du cuivre et du caoutchouc. C'est un composant complexe à équilibrer logistiquement.",
	},
	"turbopump": {
		ID: "turbopump", Name: "Turbopompe", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 8000, Icon: "🌀",
		Description:  "Pompe rotative à haut débit pour carburants cryogéniques. Le cœur battant d'un moteur de fusée.",
		StrategicTip: "Composant de très haute technologie. Sa construction demande des moteurs électriques et des alliages avancés. C'est un item à forte valeur ajoutée.",
	},
	"pressurized_tank": {
		ID: "pressurized_tank", Name: "Réservoir Pressurisé", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 3000, Icon: "🛢️",
		Description:  "Conteneur renforcé capable de supporter d'énormes pressions internes. Utilisé pour le stockage de gaz liquéfiés.",
		StrategicTip: "Nécessaire pour stocker et manipuler l'hydrogène et l'oxygène liquides. Sans ces réservoirs, pas de carburant spatial possible.",
	},
	"nitrogen": {
		ID: "nitrogen", Name: "Azote Liquide", Type: ItemTypeRessourceBrute, Unit: UnitL,
		BasePrice: 30, Minable: false, IsExplorable: false, Icon: "❄️",
		Description:  "Fluide cryogénique utilisé pour le refroidissement industriel et certains procédés chimiques de synthèse.",
		StrategicTip: "L'azote est utile pour stabiliser vos réactions chimiques complexes. Son prix est bas, mais sa disponibilité peut varier.",
	},
	"sulfuric_acid": {
		ID: "sulfuric_acid", Name: "Acide Sulfurique", Type: ItemTypeComposant, Unit: UnitL,
		BasePrice: 80, Icon: "⚗️",
		Description:  "L'acide sulfurique est un composant chimique essentiel pour le traitement des minerais avancés et la fabrication de batteries. C'est un liquide corrosif qui nécessite une gestion rigoureuse de vos chaînes de production.",
		StrategicTip: "Bien qu'il semble secondaire, l'acide est un catalyseur. Une pénurie bloque immédiatement le raffinage de produits tiers 5 comme le caoutchouc et les batteries.",
	},

	// -------------------------------------------------------------------------
	// PRODUITS FINIS
	// -------------------------------------------------------------------------
	"electric_motor": {
		ID: "electric_motor", Name: "Moteur Électrique", Type: ItemTypeProduitFini, Unit: UnitUnit,
		BasePrice: 2500, Icon: "⚡",
		Description:  "Convertisseur d'énergie électrique en énergie mécanique. Composant vital pour l'automatisation avancée et les pompes industrielles.",
		StrategicTip: "Le moteur est le premier produit fini à haute complexité. Il nécessite de synchroniser 4 lignes de production différentes (Cuivre, Fer, Acier, Plastique).",
	},
	"smartphone": {
		ID: "smartphone", Name: "Smartphone", Type: ItemTypeProduitFini, Unit: UnitUnit,
		BasePrice: 8500, Icon: "📱",
		Description:  "Le summum de la technologie grand public. Concentre des capacités de communication planétaire dans la paume de la main.",
		StrategicTip: "C'est une 'vache à lait'. La production de smartphones est extrêmement rentable si vous maîtrisez la chaîne de l'or et du lithium.",
	},
	"computer": {
		ID: "computer", Name: "Ordinateur", Type: ItemTypeProduitFini, Unit: UnitUnit,
		BasePrice: 12000, Icon: "🖥️",
		Description:  "Station de travail puissante pour le traitement de données et la simulation. Outil indispensable pour la recherche scientifique.",
		StrategicTip: "Plus complexe que le smartphone mais moins volatile. C'est un excellent produit tampon pour stabiliser vos revenus high-tech.",
	},
	// Aerospace Components
	"titanium_ingot": {
		ID: "titanium_ingot", Name: "Lingot de Titane", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 400, Icon: "🧊",
		Description:  "Métal léger et ultra-résistant, raffiné pour les environnements de haute contrainte.",
		StrategicTip: "Le titane est essentiel pour les boucliers thermiques. Stockez-le massivement avant de lancer votre programme spatial.",
	},
	"aluminum_ingot": {
		ID: "aluminum_ingot", Name: "Lingot d'Aluminium", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 50, Icon: "⚪",
		Description:  "Lingot d'aluminium purifié, apprécié pour sa légèreté et sa résistance à la corrosion.",
		StrategicTip: "L'aluminium est le métal de base pour la structure des satellites et des fusées. Sa production est rapide si vous avez assez de bauxite.",
	},
	"rocket_fuel": {
		ID: "rocket_fuel", Name: "Carburant de Fusée (LOX/LH2)", Type: ItemTypeComposant, Unit: UnitL,
		BasePrice: 500, Icon: "🚀",
		Description:  "Mélange cryogénique de haute énergie composé d'oxygène et d'hydrogène liquides.",
		StrategicTip: "La production de carburant demande énormément d'énergie. Assurez-vous que vos générateurs suivent la cadence de vos électrolyseurs.",
	},
	"heat_shield": {
		ID: "heat_shield", Name: "Bouclier Thermique", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 5000, Icon: "🛡️",
		Description:  "Protection composite conçue pour résister aux températures extrêmes de la rentrée atmosphérique.",
		StrategicTip: "Chaque moteur de fusée et chaque capsule nécessite un bouclier. La céramique est l'ingrédient principal ici.",
	},
	"guidance_system": {
		ID: "guidance_system", Name: "Système de Guidage", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 25000, Icon: "🎯",
		Description:  "Ensemble électronique sophistiqué calculant les trajectoires orbitales en temps réel.",
		StrategicTip: "C'est l'un des composants les plus complexes à produire. Nécessite plusieurs processeurs et des capteurs de haute précision.",
	},
	"rocket_engine": {
		ID: "rocket_engine", Name: "Moteur de Fusée", Type: ItemTypeProduitFini, Unit: UnitUnit,
		BasePrice: 100000, Icon: "🔥",
		Description:  "Propulsion cryogénique de haute performance capable de générer des millions de newtons de poussée.",
		StrategicTip: "Le moteur est l'aboutissement de toute votre chaîne logistique. Chaque moteur nécessite une turbopompe et des alliages avancés.",
	},
	"satellite": {
		ID: "satellite", Name: "Satellite", Type: ItemTypeProduitFini, Unit: UnitUnit,
		BasePrice: 500000, Icon: "🛠️",
		Description:  "Engin spatial autonome destiné à l'orbite. Permet les télécommunications globales et l'observation planétaire.",
		StrategicTip: "Le satellite n'est pas une fin en soi, c'est un ingrédient pour la Fusée. Ne les vendez pas tous, stockez-en 5 pour l'assemblage final.",
	},
	"rocket": {
		ID: "rocket", Name: "Fusée", Type: ItemTypeProduitFini, Unit: UnitUnit,
		BasePrice: 5000000, Icon: "🚀",
		Description:  "Véhicule de lancement lourd capable d'atteindre la vitesse de libération. L'aboutissement ultime de votre empire industriel.",
		StrategicTip: "La construction d'une fusée mobilise la totalité de votre industrie pendant des jours. C'est l'objectif final. Assurez-vous d'avoir des stocks massifs de carburant avant de commencer l'assemblage.",
	},

	// -------------------------------------------------------------------------
	// MACHINES - Tier 1 (Niveau 1+)
	// -------------------------------------------------------------------------
	"forestry_machine": {
		ID: "forestry_machine", Name: "Exploitation Forestière", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 1000, Product: "wood", ProductQuantity: 15, ProductionTime: 120,
		MaxEmployee: 2, EnergyType: EnergyTypeManuel, Icon: "/icons/forestry_machine.png",
		MarketAvailable: true,
		Description:     "Installation sylvicole de base permettant l'abattage et le débitage systématique des arbres. Elle automatise la récolte de bois brute.",
		StrategicTip:    "Votre premier investissement prioritaire. Une seule machine suffit souvent à alimenter 2 ou 3 scieries en début de partie.",
	},
	"basic_mining_machine": {
		ID: "basic_mining_machine", Name: "Extraction Minière de base", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 2500, Product: "stone", ProductQuantity: 15, ProductionTime: 120,
		MaxEmployee: 2, EnergyType: EnergyTypeManuel, Icon: "/icons/basic_mining_machine.png",
		MarketAvailable: true,
		Description:     "Excavatrice de surface rudimentaire conçue pour l'extraction de pierre et de sédiments simples.",
		StrategicTip:    "Moins prioritaire que le bois, mais nécessaire dès que vous voulez étendre votre base avec des sols en pierre.",
	},
	"sawmill": {
		ID: "sawmill", Name: "Scierie", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 1500, UseRecipe: "wooden_plank_recipe", ProductionTime: 20,
		MaxEmployee: 2, EnergyType: EnergyTypeManuel, Icon: "/icons/sawmill.png",
		MarketAvailable: true,
		Description:     "Atelier de découpe mécanique transformant les grumes brutes en planches utilisables.",
		StrategicTip:    "La scierie est très rapide (20s). Attention à ne pas surproduire des planches si votre entrepôt est plein, cela gaspillerait votre bois brut.",
	},
	"charcoal_mine": {
		ID: "charcoal_mine", Name: "Mine de Charbon", Type: ItemTypeMachine, Unit: UnitUnit,
		ProductionTime: 120, Product: "coal", ProductQuantity: 10,
		MaxEmployee: 2,
		BasePrice:   2500, EnergyType: EnergyTypeManuel, Icon: "/icons/charcoal_mine.png",
		MarketAvailable: true,
		Description:     "Installation d'extraction de charbon. Fournit le combustible nécessaire pour les fonderies et la chimie.",
		StrategicTip:    "Le charbon est indispensable pour l'acier et le titane. Ne vendez pas votre surplus, stockez-le pour vos forges.",
	},
	"sand_extractor": {
		ID: "sand_extractor", Name: "Extracteur de Sable", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 1500, Product: "silica", ProductQuantity: 20, ProductionTime: 120,
		MaxEmployee: 2, EnergyType: EnergyTypeManuel, Icon: "/icons/sand_extractor.png",
		MarketAvailable: true,
		Description:     "Machine de dragage et de tri pour l'extraction de silice pure à partir du sable.",
		StrategicTip:    "La silice est un composant de base pour le verre. Une seule machine suffit pour un démarrage rapide sur la tech verrerie.",
	},

	// -------------------------------------------------------------------------
	// MACHINES - Tier 2 (Niveau 3+)
	// -------------------------------------------------------------------------
	"iron_foundry": {
		ID: "iron_foundry", Name: "Fonderie Simple", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 5000, UseRecipe: "iron_ingot_recipe", ProductionTime: 40,
		MaxEmployee: 2, EnergyType: EnergyTypeManuel, Icon: "/icons/furnace.png",
		MarketAvailable: true,
		Description:     "Haut-fourneau primitif atteignant les températures de fusion du fer et du cuivre.",
		StrategicTip:    "Cette machine est polyvalente (fer et cuivre). Utilisez des metadata pour changer sa recette selon vos besoins du moment.",
	},
	"copper_foundry": {
		ID: "copper_foundry", Name: "Fonderie Cuivre", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 5000, UseRecipe: "copper_ingot_recipe", ProductionTime: 40,
		MaxEmployee: 2, EnergyType: EnergyTypeManuel, Icon: "/icons/furnace.png",
		MarketAvailable: true,
		Description:     "Une fonderie spécialisée dans le traitement du cuivre pour une pureté maximale.",
		StrategicTip:    "Bien que polyvalente, dédier une fonderie uniquement au cuivre évite les mélanges de stocks indésirables dans vos lignes de production.",
	},
	"copper_extractor": {
		ID: "copper_extractor", Name: "Extraction Minière de Cuivre", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 20000, Product: "copper_ore", ProductQuantity: 15, ProductionTime: 120,
		MaxEmployee: 3, EnergyType: EnergyTypeManuel, Icon: "/icons/mining_extractor.png",
		MarketAvailable: true,
		Description:     "Excavatrice de moyenne puissance optimisée pour l'extraction de filons de cuivre.",
		StrategicTip:    "Le cuivre est consommé très rapidement par l'électronique. Prévoyez d'augmenter votre capacité d'extraction dès le Tier 3.",
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
		Description:     "Foreuse industrielle conçue pour broyer les gisements de fer les plus denses.",
		StrategicTip:    "Le fer est la colonne vertébrale de votre industrie. Ne négligez jamais cette ressource sous peine de bloquer toute votre progression.",
	},

	// -------------------------------------------------------------------------
	// MACHINES - Tier 3 (Niveau 6+)
	// -------------------------------------------------------------------------
	"glass_furnace": {
		ID: "glass_furnace", Name: "Four à Verre", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 8500, UseRecipe: "glass_recipe", ProductionTime: 60,
		MaxEmployee: 2, EnergyType: EnergyTypeManuel, Icon: "/icons/glass_furnace.png",
		MarketAvailable: true,
		Description:     "Four à haute température conçu pour transformer la silice en verre de haute qualité.",
		StrategicTip:    "Le verre est léger mais volumineux. Stockez-le à proximité de vos usines d'ordinateurs et de smartphones.",
	},
	"steel_press": {
		ID: "steel_press", Name: "Presse à Acier", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 15000, UseRecipe: "steel_recipe", ProductionTime: 120,
		MaxEmployee: 3, EnergyType: EnergyTypeManuel, Icon: "/icons/steel_press.png",
		MarketAvailable: true,
		Description:     "Machine de compression lourde permettant de forger l'alliage fer-carbone en lingots d'acier.",
		StrategicTip:    "La production d'acier est lente. Multiplier ces presses est souvent plus efficace que d'augmenter le nombre d'employés sur une seule machine.",
	},
	"gold_mine": {
		ID: "gold_mine", Name: "Mine d'Or", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 50000, Product: "gold_ore", ProductQuantity: 5, ProductionTime: 180,
		MaxEmployee: 3, EnergyType: EnergyTypeManuel, Icon: "/icons/gold_mine.png",
		MarketAvailable: true,
		Description:     "Installation d'extraction de métaux précieux. Indispensable pour l'électronique de pointe.",
		StrategicTip:    "L'or est rare et sa production est lente. Ne l'utilisez que pour les composants indispensables comme les processeurs.",
	},

	// -------------------------------------------------------------------------
	// MACHINES - Tier 4 (Niveau 10+)
	// -------------------------------------------------------------------------
	"oil_refinery": {
		ID: "oil_refinery", Name: "Raffinerie", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 45000, UseRecipe: "plastic_recipe", ProductionTime: 120,
		MaxEmployee: 5, EnergyType: EnergyTypeManuel, Icon: "🏭",
		MarketAvailable: true,
		Description:     "Complexe pétrochimique fractionnant le pétrole brut en polymères plastiques utiles.",
		StrategicTip:    "Goulot d'étranglement classique du Tier 4. Prévoyez de l'espace pour au moins 2 ou 3 raffineries si vous visez les circuits électroniques.",
	},
	"petrol_pumpjack": {
		ID: "petrol_pumpjack", Name: "Pompe à Pétrole", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 45000, Product: "crude_oil", ProductQuantity: 15, ProductionTime: 120,
		MaxEmployee: 1, EnergyType: EnergyTypeManuel, Icon: "/icons/petrol_pumpjack.png",
		MarketAvailable: true,
		Description:     "Chevalet de pompage pour l'extraction de pétrole brut des gisements terrestres.",
		StrategicTip:    "Une extraction régulière de pétrole est la clé pour stabiliser votre production de plastique.",
	},
	"lithium_extractor": {
		ID: "lithium_extractor", Name: "Extracteur de Lithium", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 75000, Product: "lithium", ProductQuantity: 10, ProductionTime: 150,
		MaxEmployee: 3, EnergyType: EnergyTypeManuel, Icon: "/icons/lithium_extractor.png",
		MarketAvailable: true,
		Description:     "Machine de forage spécialisée pour l'extraction des sels de lithium.",
		StrategicTip:    "Primordial pour les batteries. Assurez-vous d'avoir un flux constant de lithium avant de lancer la production de batteries haute capacité.",
	},
	"assembly_line": {
		ID: "assembly_line", Name: "Ligne d'Assemblage", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 85000, UseRecipe: "electric_motor_recipe", ProductionTime: 360,
		MaxEmployee: 8, EnergyType: EnergyTypeManuel, Icon: "🏭",
		MarketAvailable: true,
		Description:     "Chaîne de montage automatisée capable d'assembler des produits finis complexes à partir de composants multiples.",
		StrategicTip:    "C'est la machine 'reine'. Elle peut fabriquer presque tout (Moteurs, Batteries, Câbles...). Ne sous-estimez pas son coût en espace et en employés (8 max).",
	},
	"oil_platform": {
		ID: "oil_platform", Name: "Plateforme Pétrolière Offshore", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 500000, Product: "crude_oil", ProductQuantity: 2000, ProductionTime: 200,
		MaxEmployee: 10, EnergyType: EnergyTypeManuel, Icon: "/icons/oil_platform.png",
		MarketAvailable: false,
		Description:     "Structure monumentale capable d'extraire des quantités massives de pétrole en haute mer.",
		StrategicTip:    "C'est un investissement colossal qui résoudra vos problèmes de pétrole pour le reste de la partie. Nécessite une logistique de transport solide.",
	},
	"reinforced_steel": {
		ID: "reinforced_steel", Name: "Acier Renforcé", Type: ItemTypeComposant, Unit: UnitUnit,
		BasePrice: 150, Icon: "🛡️",
		MarketAvailable: false,
		Description:     "Acier traité thermiquement et allié pour une résistance structurelle supérieure.",
		StrategicTip:    "Nécessaire pour les structures de fusée et les machines de Tier 5. Un composant de transition crucial.",
	},

	// -------------------------------------------------------------------------
	// MACHINES - Tier 5 (Niveau 15+)
	// -------------------------------------------------------------------------
	"hightech_factory": {
		ID: "hightech_factory", Name: "Usine High-Tech", Type: ItemTypeMachine, Unit: UnitUnit,
		MarketAvailable: true,
		BasePrice:       250000, UseRecipe: "smartphone_recipe", ProductionTime: 600,
		MaxEmployee: 12, NeedEnergy: 200, EnergyType: EnergyTypeManuel, Icon: "🏢",
		Description:  "Complexe industriel ultra-propre conçu pour l'assemblage de produits électroniques grand public.",
		StrategicTip: "C'est ici que vous générez vos plus gros profits avec les smartphones. Optimisez l'approvisionnement en processeurs et verre.",
	},

	// -------------------------------------------------------------------------
	// MACHINES - Tier 5.5 Intermédiaire (Niveau 18-22)
	// -------------------------------------------------------------------------
	"carbonization_furnace": {
		ID: "carbonization_furnace", Name: "Four de Carbonisation", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 120000, UseRecipe: "carbon_fiber_recipe", ProductionTime: 180,
		MaxEmployee: 4, NeedEnergy: 80, EnergyType: EnergyTypeManuel, Icon: "🖤",
		MarketAvailable: true,
		Description:     "Four spécialisé traitant les polymères à haute température sous atmosphère contrôlée pour produire de la fibre de carbone.",
		StrategicTip:    "Surveillez votre stock de plastique. Un four de carbonisation à l'arrêt peut retarder tout votre programme aérospatial.",
	},
	"ceramic_kiln": {
		ID: "ceramic_kiln", Name: "Four à Céramique", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 80000, UseRecipe: "ceramic_recipe", ProductionTime: 120,
		MarketAvailable: true,
		MaxEmployee:     3, NeedEnergy: 60, EnergyType: EnergyTypeManuel, Icon: "🏺",
		Description:  "Four industriel pour le frittage de poudres de silice et d'alumine en composants céramiques.",
		StrategicTip: "La céramique est demandée pour plusieurs composants de Tier 5. Prévoyez une production constante mais modérée.",
	},
	"precision_workshop": {
		ID: "precision_workshop", Name: "Atelier de Précision", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 200000, UseRecipe: "sensor_recipe", ProductionTime: 240,
		MarketAvailable: true,
		MaxEmployee:     6, NeedEnergy: 100, EnergyType: EnergyTypeManuel, Icon: "🔬",
		Description:  "Atelier équipé d'outils de mesure micrométriques pour la fabrication de capteurs et d'instruments de bord.",
		StrategicTip: "C'est la base de vos systèmes de guidage. Un atelier de précision bien approvisionné en composants électroniques est vital.",
	},
	"rubber_factory": {
		ID: "rubber_factory", Name: "Usine de Caoutchouc", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 90000, UseRecipe: "rubber_recipe", ProductionTime: 90,
		MarketAvailable: true,
		MaxEmployee:     4, NeedEnergy: 50, EnergyType: EnergyTypeManuel, Icon: "⚫",
		Description:  "Installation chimique pour la vulcanisation et le traitement des polymères de caoutchouc.",
		StrategicTip: "Assurez-vous d'avoir une ligne de pétrole brut dédiée pour ne jamais manquer de caoutchouc pour vos vérins.",
	},
	"hydraulic_press": {
		ID: "hydraulic_press", Name: "Presse Hydraulique", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 150000, UseRecipe: "hydraulic_cylinder_recipe", ProductionTime: 150,
		MarketAvailable: true,
		MaxEmployee:     4, NeedEnergy: 80, EnergyType: EnergyTypeManuel, Icon: "🔧",
		Description:  "Presse de grande puissance pour l'assemblage et le façonnage de vérins et composants lourds.",
		StrategicTip: "Indispensable pour vos lignes d'assemblage avancées. Nécessite beaucoup de caoutchouc et d'acier.",
	},
	"advanced_forge": {
		ID: "advanced_forge", Name: "Forge Avancée", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 250000, UseRecipe: "advanced_alloy_recipe", ProductionTime: 200,
		MarketAvailable: true,
		MaxEmployee:     5, NeedEnergy: 120, EnergyType: EnergyTypeManuel, Icon: "🔥",
		Description:  "Unité de fusion spécialisée dans la création d'alliages de haute performance.",
		StrategicTip: "Produit les fameuses Turbopompes. C'est le cœur de votre programme spatial, protégez sa chaîne logistique.",
	},

	// -------------------------------------------------------------------------
	// MACHINES - Tier 6 Aerospace (Niveau 25+)
	// -------------------------------------------------------------------------
	"chemical_plant": {
		ID: "chemical_plant", Name: "Usine Chimique", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 500000, UseRecipe: "rocket_fuel_recipe", ProductionTime: 300,
		MarketAvailable: true,
		MaxEmployee:     6, NeedEnergy: 150, EnergyType: EnergyTypeManuel, Icon: "⚗️",
		Description:  "Complexe industriel polyvalent pour la synthèse de produits chimiques complexes et de carburant de fusée.",
		StrategicTip: "C'est ici que vous produisez votre carburant. Sa proximité avec vos sources d'oxygène et d'hydrogène est un atout majeur.",
	},
	"titanium_foundry": {
		ID: "titanium_foundry", Name: "Fonderie de Titane", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 350000, UseRecipe: "titanium_ingot_recipe", ProductionTime: 180,
		MarketAvailable: true,
		MaxEmployee:     4, NeedEnergy: 100, EnergyType: EnergyTypeManuel, Icon: "🔥",
		Description:  "Fonderie de haute technologie capable d'atteindre les températures nécessaires au raffinage du titane.",
		StrategicTip: "Le titane est requis pour vos boucliers thermiques. Assurez-vous d'avoir un stock constant de lingots.",
	},
	"aluminum_foundry": {
		ID: "aluminum_foundry", Name: "Fonderie d'Aluminium", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 150000, UseRecipe: "aluminum_ingot_recipe", ProductionTime: 60,
		MarketAvailable: true,
		MaxEmployee:     3, EnergyType: EnergyTypeManuel, Icon: "🔥",
		Description:  "Installation spécialisée dans le traitement de la bauxite et la fonte de l'aluminium.",
		StrategicTip: "Indispensable pour les structures aérospatiales. Sa production est rapide si vous avez assez de bauxite.",
	},
	"titanium_mine": {
		ID: "titanium_mine", Name: "Mine de Titane", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 400000, Product: "titanium_ore", ProductQuantity: 5, ProductionTime: 300,
		MarketAvailable: true,
		MaxEmployee:     5, EnergyType: EnergyTypeManuel, Icon: "⛏️",
		Description:  "Foreuse géante conçue pour l'extraction de minerais de titane à grande profondeur.",
		StrategicTip: "Une production lente mais cruciale. Prévoyez de renforcer vos effectifs miniers une fois le titane débloqué.",
	},
	"electrolysis_plant": {
		ID: "electrolysis_plant", Name: "Station d'Électrolyse", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 300000, UseRecipe: "electrolysis_recipe", ProductionTime: 120,
		MarketAvailable: true,
		MaxEmployee:     4, NeedEnergy: 200, EnergyType: EnergyTypeManuel, Icon: "⚡",
		Description:  "Installation utilisant l'électricité pour décomposer l'eau en oxygène et hydrogène.",
		StrategicTip: "Gros consommateur d'énergie. Assurez-vous d'avoir une infrastructure électrique solide avant d'en placer plusieurs.",
	},
	"aerospace_factory": {
		ID: "aerospace_factory", Name: "Usine Aérospatiale", Type: ItemTypeMachine, Unit: UnitUnit,
		BasePrice: 2000000, UseRecipe: "rocket_engine_recipe", ProductionTime: 1800,
		MarketAvailable: true,
		MaxEmployee:     20, NeedEnergy: 500, EnergyType: EnergyTypeManuel, Icon: "🏭",
		Description:  "Complexe de haute technologie conçu pour l'assemblage de composants aérospatiaux géants.",
		StrategicTip: "C'est ici qu'on assemble les moteurs de fusée. Une seule usine bien approvisionnée suffit au début.",
	},
	"rocket_launch_pad": {
		ID: "rocket_launch_pad", Name: "Pas de Tir", Type: ItemTypeMachine, Unit: UnitUnit,
		MarketAvailable: true,
		BasePrice:       50000000, UseRecipe: "rocket_recipe", ProductionTime: 7200,
		MaxEmployee: 50, NeedEnergy: 1000, EnergyType: EnergyTypeManuel, Icon: "🚀",
		Description:  "Infrastructure monumentale capable de supporter l'assemblage et le lancement d'une fusée orbitale.",
		StrategicTip: "L'aboutissement de votre empire. Nécessite des ressources colossales et une main-d'œuvre massive (50 employés).",
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
		Description:  "Structure de stockage de base pour les ressources solides et les composants.",
		StrategicTip: "Placez vos entrepôts au croisement de plusieurs lignes de production pour tamponner vos stocks.",
	},
	"fluid_tank_small": {
		ID: "fluid_tank_small", Name: "Citerne Standard", Type: ItemTypeStockage, Unit: UnitUnit,
		BasePrice: 7500, Icon: "🛢️", MarketAvailable: true,
		Metadata: &MachineMetadata{
			SupportedStorageTypes: []Unit{UnitL},
		},
		Description:  "Citerne étanche optimisée pour le stockage sécurisé des liquides et gaz compressés.",
		StrategicTip: "Indispensable pour tamponner votre production de pétrole brut et de carburant de fusée.",
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
