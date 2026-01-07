package gamedata

// =============================================================================
// GAME CONSTANTS - Lazy Evaluation Rules
// =============================================================================

const (
	// Deposit Capacity
	EmployeesPerDepositSize  = 5 // Max employees = size * 5
	MachinesPerDepositSize   = 1 // Max machines = size * 1
	MachineEquivalentWorkers = 5 // 1 machine = 5 worker equivalent
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
