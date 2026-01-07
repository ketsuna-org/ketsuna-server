package gamedata

// =============================================================================
// GAME CONSTANTS - Lazy Evaluation Rules
// =============================================================================

const (
	// Deposit Capacity
	EmployeesPerDepositSize  = 5 // Max employees = size * 5
	MachinesPerDepositSize   = 1 // Max machines = size * 1
	MachineEquivalentWorkers = 5 // 1 machine = 5 worker equivalent

	// Energy Cycle (synchronized for all employees)
	EnergyWorkDuration = 1440 // 24 minutes in seconds
	EnergyRestDuration = 1440 // 24 minutes in seconds
	EnergyCycleTotal   = 2880 // 48 minutes total cycle

	// Machine Durability
	MachineDurabilityOnPlace   = 1000 // Durability when machine is placed
	MaintenanceIntervalSeconds = 10   // Maintenance skill applied every 10s

	// Default harvest cycle (can be overridden by item ProductionTime)
	DefaultHarvestCycle = 20 // seconds
)

// GetMaxEmployeesForDeposit returns max employees allowed for a deposit size
func GetMaxEmployeesForDeposit(size int) int {
	return size * EmployeesPerDepositSize
}

// GetMaxMachinesForDeposit returns max machines allowed for a deposit size
func GetMaxMachinesForDeposit(size int) int {
	return size * MachinesPerDepositSize
}

// GetDepositWorkerCapacity returns total worker equivalent capacity
// (employees + machines * MachineEquivalentWorkers)
func GetDepositWorkerCapacity(employees, machines int) int {
	return employees + (machines * MachineEquivalentWorkers)
}
