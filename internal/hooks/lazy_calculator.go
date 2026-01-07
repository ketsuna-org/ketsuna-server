package hooks

import (
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
// CalculateEmployeeProductivity returns the total "Effective Work Seconds".
// SIMPLIFICATION: Work/Rest cycle is removed. Employees are always 100% efficient.
func (c *LazyCalculator) CalculateEmployeeProductivity(emp *core.Record, start, end time.Time) float64 {
	totalSeconds := end.Sub(start).Seconds()
	if totalSeconds < 0 {
		return 0
	}
	return totalSeconds
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
