package hooks

import (
	"github.com/pocketbase/pocketbase"
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

// NOTE: Employee productivity and deposit capacity functions have been removed.
// Employees are no longer used in the mining system - they are only used in explorations.
