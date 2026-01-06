package hooks

import (
	"fmt"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
	"ketsuna.com/server/internal/gamedata"
)

// ProcessDepositHarvesting handles harvesting for employees assigned directly to deposits
// Each employee harvests resources based on their mining stat
func (l *EconomyLogic) ProcessDepositHarvesting(companyId string) error {
	// Get all employees assigned to deposits for this company
	employees, err := l.app.FindRecordsByFilter(
		"employees",
		fmt.Sprintf("employer = '%s' && deposit != ''", companyId),
		"",
		1000,
		0,
	)
	if err != nil || len(employees) == 0 {
		return nil
	}

	// Group employees by deposit for efficiency
	depositEmployees := make(map[string][]*core.Record)
	for _, emp := range employees {
		depositId := emp.GetString("deposit")
		depositEmployees[depositId] = append(depositEmployees[depositId], emp)
	}

	// Process each deposit
	for depositId, emps := range depositEmployees {
		err := l.processDepositHarvest(companyId, depositId, emps)
		if err != nil {
			l.app.Logger().Error("[HARVEST] Error processing deposit", "depositId", depositId, "err", err)
		}
	}

	return nil
}

// processDepositHarvest handles harvesting for a single deposit
func (l *EconomyLogic) processDepositHarvest(companyId string, depositId string, employees []*core.Record) error {
	deposit, err := l.app.FindRecordById("deposits", depositId)
	if err != nil {
		return err
	}

	remaining := deposit.GetFloat("quantity")
	if remaining <= 0 {
		// Deposit is empty - unassign all employees
		for _, emp := range employees {
			emp.Set("deposit", "")
			l.app.Save(emp)
		}
		return nil
	}

	resourceId := deposit.GetString("ressource")
	if resourceId == "" {
		return fmt.Errorf("deposit has no resource")
	}

	// Get resource item from static gamedata for production time
	resource := gamedata.GetItem(resourceId)
	if resource == nil {
		return fmt.Errorf("unknown resource: %s", resourceId)
	}

	// Production time in seconds (default 60s if not set)
	productionTime := resource.ProductionTime
	if productionTime <= 0 {
		productionTime = 60
	}

	// Process each employee
	totalHarvested := 0.0
	for _, emp := range employees {
		harvested := l.processEmployeeHarvest(emp, productionTime)
		totalHarvested += harvested
	}

	if totalHarvested <= 0 {
		return nil
	}

	// Cap by remaining quantity
	if totalHarvested > remaining {
		totalHarvested = remaining
	}

	// Update deposit quantity
	newRemaining := remaining - totalHarvested
	deposit.Set("quantity", newRemaining)
	if err := l.app.Save(deposit); err != nil {
		return err
	}

	// Add to inventory
	err = l.inventory.UpdateInventory(l.app, companyId, resourceId, int(totalHarvested))
	if err != nil {
		l.app.Logger().Error("[HARVEST] Failed to update inventory", "err", err)
	}

	return nil
}

// processEmployeeHarvest processes harvesting for a single employee
// Returns the quantity harvested
func (l *EconomyLogic) processEmployeeHarvest(emp *core.Record, productionTime int) float64 {
	// Check if employee has started harvesting
	harvestStarted := emp.GetDateTime("updated") // Use updated as proxy for last action

	// In a more sophisticated system, we'd track harvest_started_at per employee
	// For simplicity, we use a time-based check on each cron tick
	now := time.Now()

	// Calculate base quantity: 1 + (mining / 5)
	miningSkill := emp.GetFloat("mining")
	baseQty := 1.0 + (miningSkill / 5.0)

	// Check if enough time has passed since last update
	lastUpdate := harvestStarted.Time()
	if lastUpdate.IsZero() {
		// First time - just mark and wait
		emp.Set("updated", types.NowDateTime())
		l.app.Save(emp)
		return 0
	}

	elapsed := now.Sub(lastUpdate).Seconds()
	if elapsed < float64(productionTime) {
		return 0 // Not enough time passed
	}

	// Calculate number of harvest cycles completed
	cycles := int(elapsed / float64(productionTime))
	if cycles <= 0 {
		return 0
	}

	// Max 10 cycles per tick to prevent overflow on first run
	if cycles > 10 {
		cycles = 10
	}

	harvested := baseQty * float64(cycles)

	// Apply efficiency modifier
	efficiency := emp.GetFloat("efficiency")
	if efficiency > 0 {
		harvested *= (efficiency / 100.0)
	}

	// Touch employee to reset timer
	emp.Set("updated", types.NowDateTime())
	l.app.Save(emp)

	return harvested
}
