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
// EMPLOYEE ENERGY CALCULATIONS (DETERMINISTIC)
// =============================================================================

// CalculateEmployeeProductivity returns the total "Effective Work Seconds"
// produced by an employee over a time period, considering their Work/Rest cycle.
// Cycle: 24m Work (100%->0% efficiency), 24m Rest (0% efficiency).
// Efficiency is linear with Energy. Avg efficiency during work is 50%.
func (c *LazyCalculator) CalculateEmployeeProductivity(emp *core.Record, start, end time.Time) float64 {
	// Offset determines the phase of the employee
	// Use Created timestamp to randomize offset (or ID hash)
	// simple: offset = created.Unix()
	created := emp.GetDateTime("created").Time()
	offset := int64(created.Unix())

	totalSeconds := end.Sub(start).Seconds()
	if totalSeconds <= 0 {
		return 0
	}

	// We integrate the productivity over the window.
	// Function: P(t) = Energy(t)/100.
	// Work Phase (0-24m): Energy(t) = 100 * (1 - t/WorkDuration) -> P(t) = 1 - t/Duration.
	// Rest Phase (24-48m): Energy(t) = Recovery... but Productivity is 0!
	// So we only care about Work Phase.

	// Productivity integral for full Work Phase = 0.5 * Duration.
	// Avg Productivity over full Cycle = (0.5 * Work + 0 * Rest) / Total = 0.5 * 24 / 48 = 0.25.

	// For long periods (> 1 cycle), we can approximate:
	// Total = totalSeconds * 0.25.
	// But for short/precise periods, we should calculate exact overlap.

	cycleTotal := float64(gamedata.EnergyCycleTotal)

	// Start time in cycle
	startT := float64(start.Unix() + offset)
	endT := float64(end.Unix() + offset)

	// Number of full cycles fully contained
	// It's hard to integrate analytically with mod.
	// Step-wise integration?
	// Given delta is usually small (minutes) or large (hours).

	// Simple integration:
	effectiveSeconds := 0.0

	// Helper to integrate [t1, t2] within a single cycle [0, Total]
	integrateCycle := func(t1, t2 float64) float64 {
		// Clamp to Work Phase [0, WorkDuration]
		workDur := float64(gamedata.EnergyWorkDuration)

		// Segment [t1, t2] overlaps with [0, workDur]
		start := math.Max(t1, 0)
		end := math.Min(t2, workDur)

		if start >= end {
			return 0
		}

		// Integral of (1 - t/workDur) dt from start to end
		// = [t - t^2/(2*workDur)] from start to end
		valEnd := end - (end*end)/(2*workDur)
		valStart := start - (start*start)/(2*workDur)

		return valEnd - valStart
	}

	// We can iterate cycles since start
	current := startT
	for current < endT {
		cycleStart := math.Floor(current/cycleTotal) * cycleTotal
		relStart := current - cycleStart

		amountRemainingInCycle := cycleTotal - relStart
		amountToProcess := math.Min(endT-current, amountRemainingInCycle)
		relEnd := relStart + amountToProcess

		effectiveSeconds += integrateCycle(relStart, relEnd)

		current += amountToProcess
	}

	return effectiveSeconds
}

// CalculateEmployeeMaintenance returns the total "Effective Maintenance Seconds"
// produced by an employee over a time period.
// Maintenance happens during REST phase. Assumed 100% distinct from Work.
// Efficiency: Assuming simple constant efficiency during Rest?
// Or does Energy recover 0->100 imply Maintenance capability 0->100?
// Usually maintenance is fixed rate?
// Let's assume Maintenance Power is constant during Rest Phase.
func (c *LazyCalculator) CalculateEmployeeMaintenance(emp *core.Record, start, end time.Time) float64 {
	created := emp.GetDateTime("created").Time()
	offset := int64(created.Unix())

	workDur := float64(gamedata.EnergyWorkDuration)
	cycleTotal := float64(gamedata.EnergyCycleTotal)

	startT := float64(start.Unix() + offset)
	endT := float64(end.Unix() + offset)

	effectiveSeconds := 0.0

	integrateRest := func(t1, t2 float64) float64 {
		// Rest phase is [WorkDuration, CycleTotal]
		start := math.Max(t1, workDur)
		end := math.Min(t2, cycleTotal)

		if start >= end {
			return 0
		}
		return end - start // Constant 1.0 efficiency during rest
	}

	current := startT
	for current < endT {
		cycleStart := math.Floor(current/cycleTotal) * cycleTotal
		relStart := current - cycleStart
		relEnd := relStart + math.Min(endT-current, cycleTotal-relStart)

		effectiveSeconds += integrateRest(relStart, relEnd)

		current += (relEnd - relStart)
	}

	return effectiveSeconds
}

// GetCurrentEnergy returns the current energy (0-100) and state (Work/Rest)
func (c *LazyCalculator) GetCurrentEnergy(emp *core.Record) (float64, string) {
	created := emp.GetDateTime("created").Time()
	offset := int64(created.Unix())
	now := time.Now().Unix() + offset

	cyclePos := math.Mod(float64(now), float64(gamedata.EnergyCycleTotal))
	workDur := float64(gamedata.EnergyWorkDuration)

	if cyclePos < workDur {
		// Work Phase: 100 -> 0
		pct := 1.0 - (cyclePos / workDur)
		return pct * 100.0, "working"
	} else {
		// Rest Phase: 0 -> 100
		restDur := float64(gamedata.EnergyRestDuration)
		pct := (cyclePos - workDur) / restDur
		return pct * 100.0, "resting"
	}
}

// CalculateMachineDurability calculates current durability after maintenance
func (c *LazyCalculator) CalculateMachineDurability(
	currentDurability float64,
	lastUpdate time.Time,
	employees []*core.Record,
) float64 {
	if lastUpdate.IsZero() {
		return currentDurability
	}

	totalMaintenance := 0.0
	now := time.Now()

	for _, emp := range employees {
		maintSkill := emp.GetInt("maintenance")
		if maintSkill > 0 {
			// Effective seconds handling maintenance
			effSeconds := c.CalculateEmployeeMaintenance(emp, lastUpdate, now)

			// Maintenance Power = Skill * Seconds / Interval
			// e.g. Skill 10, 60 seconds -> 600 points?
			// Check constant: MaintenanceIntervalSeconds (10s)
			// Implementation: per 10s act, apply skill?
			// So Total Points = Skill * (Seconds / 10)

			points := float64(maintSkill) * (effSeconds / float64(gamedata.MaintenanceIntervalSeconds))
			totalMaintenance += points
		}
	}

	newDurability := currentDurability + totalMaintenance
	if newDurability > float64(gamedata.MachineDurabilityOnPlace) {
		newDurability = float64(gamedata.MachineDurabilityOnPlace)
	}

	return newDurability
}

// =============================================================================
// DEPOSIT CAPACITY CALCULATIONS
// =============================================================================

// DepositCapacityInfo (Unchanged)
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

	employees, _ := c.app.FindRecordsByFilter("employees", "deposit = '"+depositId+"'", "", 0, 0)
	machines, _ := c.app.FindRecordsByFilter("machines", "deposit = '"+depositId+"'", "", 0, 0)

	currentEmp := len(employees)
	currentMach := len(machines)

	// Check equivalent logic?
	// The rule "1 machine = 5 employees" is for "usedSlots".
	// But GetDepositCapacity returns raw counts.
	// We should probably return "Slots" info or validation.
	// But keeping existing struct for now.

	return &DepositCapacityInfo{
		MaxEmployees:     maxEmp,
		MaxMachines:      maxMach,
		CurrentEmployees: currentEmp,
		CurrentMachines:  currentMach,
		CanAddEmployees:  maxEmp - currentEmp,
		CanAddMachines:   maxMach - currentMach,
	}, nil
}
