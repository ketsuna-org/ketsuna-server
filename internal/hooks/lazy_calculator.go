package hooks

import (
	"math"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"ketsuna.com/server/internal/gamedata"
)

// LazyCalculator handles all lazy evaluation calculations
// for the game economy without modifying state
type LazyCalculator struct {
	app *pocketbase.PocketBase
}

// NewLazyCalculator creates a new lazy calculator instance
func NewLazyCalculator(app *pocketbase.PocketBase) *LazyCalculator {
	return &LazyCalculator{app: app}
}

// =============================================================================
// EMPLOYEE ENERGY CALCULATIONS
// =============================================================================

// CalculateGlobalEnergyPercent calculates the current energy for ALL employees
// Energy is synchronized: 24 min work (100→0) + 24 min rest (0→100)
// Returns 0-100 based on current time position in the cycle
func (c *LazyCalculator) CalculateGlobalEnergyPercent() float64 {
	now := time.Now()
	// Use midnight as cycle reference for synchronization
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	secondsSinceMidnight := now.Sub(midnight).Seconds()

	// Calculate position in current 48-minute cycle
	cyclePosition := math.Mod(secondsSinceMidnight, float64(gamedata.EnergyCycleTotal))

	if cyclePosition < float64(gamedata.EnergyWorkDuration) {
		// Work phase: 100% → 0%
		progress := cyclePosition / float64(gamedata.EnergyWorkDuration)
		return 100.0 * (1.0 - progress)
	} else {
		// Rest phase: 0% → 100%
		restProgress := (cyclePosition - float64(gamedata.EnergyWorkDuration)) / float64(gamedata.EnergyRestDuration)
		return 100.0 * restProgress
	}
}

// IsWorkPhase returns true if employees are currently in work phase
func (c *LazyCalculator) IsWorkPhase() bool {
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	secondsSinceMidnight := now.Sub(midnight).Seconds()
	cyclePosition := math.Mod(secondsSinceMidnight, float64(gamedata.EnergyCycleTotal))
	return cyclePosition < float64(gamedata.EnergyWorkDuration)
}

// IsMaintenancePhase returns true if maintenance workers are active
// (inverse of work phase - maintenance works when workers rest)
func (c *LazyCalculator) IsMaintenancePhase() bool {
	return !c.IsWorkPhase()
}

// =============================================================================
// MACHINE DURABILITY CALCULATIONS
// =============================================================================

// CalculateMachineDurability calculates current durability after maintenance
// maintenanceSkill: sum of maintenance stat from assigned maintenance employees
// lastUpdate: when durability was last calculated
// currentDurability: stored durability value
func (c *LazyCalculator) CalculateMachineDurability(
	currentDurability float64,
	lastUpdate time.Time,
	maintenanceSkill int,
) float64 {
	if lastUpdate.IsZero() {
		return currentDurability
	}

	// Maintenance only works during rest phase
	if !c.IsMaintenancePhase() {
		return currentDurability
	}

	now := time.Now()
	delta := now.Sub(lastUpdate).Seconds()

	// Calculate maintenance intervals elapsed
	intervals := int(delta / float64(gamedata.MaintenanceIntervalSeconds))
	if intervals <= 0 {
		return currentDurability
	}

	// Add maintenance per interval
	repaired := currentDurability + float64(intervals*maintenanceSkill)

	// Cap at max durability
	if repaired > float64(gamedata.MachineDurabilityOnPlace) {
		repaired = float64(gamedata.MachineDurabilityOnPlace)
	}

	return repaired
}

// =============================================================================
// DEPOSIT CAPACITY CALCULATIONS
// =============================================================================

// DepositCapacityInfo contains capacity information for a deposit
type DepositCapacityInfo struct {
	MaxEmployees     int
	MaxMachines      int
	CurrentEmployees int
	CurrentMachines  int
	CanAddEmployees  int
	CanAddMachines   int
}

// GetDepositCapacity calculates capacity limits for a deposit
func (c *LazyCalculator) GetDepositCapacity(depositId string) (*DepositCapacityInfo, error) {
	deposit, err := c.app.FindRecordById("deposits", depositId)
	if err != nil {
		return nil, err
	}

	size := deposit.GetInt("size")
	if size <= 0 {
		size = 1
	}

	maxEmp := gamedata.GetMaxEmployeesForDeposit(size)
	maxMach := gamedata.GetMaxMachinesForDeposit(size)

	// Count current assignments
	employees, _ := c.app.FindRecordsByFilter("employees",
		"deposit = '"+depositId+"'", "", 0, 0)
	machines, _ := c.app.FindRecordsByFilter("machines",
		"deposit = '"+depositId+"'", "", 0, 0)

	currentEmp := len(employees)
	currentMach := len(machines)

	return &DepositCapacityInfo{
		MaxEmployees:     maxEmp,
		MaxMachines:      maxMach,
		CurrentEmployees: currentEmp,
		CurrentMachines:  currentMach,
		CanAddEmployees:  maxEmp - currentEmp,
		CanAddMachines:   maxMach - currentMach,
	}, nil
}

// =============================================================================
// PRODUCTION CALCULATIONS
// =============================================================================

// ProductionInput contains all data needed for production calculation
type ProductionInput struct {
	Machine          *core.Record
	ItemDef          *gamedata.Item
	Employees        []*core.Record
	MaintenanceEmps  []*core.Record
	Deposit          *core.Record
	LastProductionAt time.Time
}

// LazyProductionResult contains the result of a production calculation
type LazyProductionResult struct {
	Quantity         float64
	ItemID           string
	CyclesCompleted  int
	DurabilityLoss   float64
	EffectiveWorkers float64
	CanProduce       bool
	BlockReason      string
}

// CalculateMachineProduction calculates what a machine has produced since last check
// This is a PURE calculation - does not modify any state
func (c *LazyCalculator) CalculateMachineProduction(input *ProductionInput) *LazyProductionResult {
	result := &LazyProductionResult{
		ItemID:     input.ItemDef.Product,
		CanProduce: true,
	}

	// Check 1: Durability
	durability := input.Machine.GetFloat("durability")
	if durability <= 0 {
		result.CanProduce = false
		result.BlockReason = "Machine hors service (durabilité 0)"
		return result
	}

	// Check 2: Global energy
	energy := c.CalculateGlobalEnergyPercent()
	if energy <= 0 {
		result.CanProduce = false
		result.BlockReason = "Employés au repos"
		return result
	}

	// Check 3: Calculate time delta
	if input.LastProductionAt.IsZero() {
		return result // First run, no production yet
	}

	now := time.Now()
	delta := now.Sub(input.LastProductionAt).Seconds()

	cycleTime := float64(input.ItemDef.ProductionTime)
	if cycleTime <= 0 {
		cycleTime = float64(gamedata.DefaultHarvestCycle)
	}

	cycles := int(delta / cycleTime)
	if cycles <= 0 {
		return result
	}

	result.CyclesCompleted = cycles

	// Calculate effective workers (employees + machine bonus)
	totalMiningPower := 0.0
	for _, emp := range input.Employees {
		mining := emp.GetInt("mining")
		if mining > 0 {
			totalMiningPower += float64(mining)
		}
	}

	// Machine itself counts as equivalent workers
	if len(input.Employees) > 0 || totalMiningPower > 0 {
		// Machine provides base production boost
		result.EffectiveWorkers = totalMiningPower
	}

	// Calculate production
	baseProduction := float64(input.ItemDef.ProductQuantity)
	if baseProduction <= 0 {
		baseProduction = 1
	}

	// Energy efficiency modifier
	energyModifier := energy / 100.0

	// Total = cycles * base * mining_bonus * energy
	result.Quantity = float64(cycles) * baseProduction * (1 + totalMiningPower/10) * energyModifier

	// Durability loss
	result.DurabilityLoss = float64(cycles)

	return result
}

// GetTotalMiningPower returns total mining stat from employees
func (c *LazyCalculator) GetTotalMiningPower(employees []*core.Record) int {
	total := 0
	for _, emp := range employees {
		total += emp.GetInt("mining")
	}
	return total
}

// GetTotalMaintenanceSkill returns total maintenance stat from employees
func (c *LazyCalculator) GetTotalMaintenanceSkill(employees []*core.Record) int {
	total := 0
	for _, emp := range employees {
		total += emp.GetInt("maintenance")
	}
	return total
}
