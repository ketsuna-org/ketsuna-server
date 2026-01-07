package hooks

import (
	"time"
)

type EnergyStatus struct {
	EnergyProduced  float64   `json:"energyProduced"`
	EnergyDemand    float64   `json:"energyDemand"`
	EnergyStored    float64   `json:"energyStored"`
	MaxEnergyStored float64   `json:"maxEnergyStored"`
	EnergyRatio     float64   `json:"energyRatio"`
	ProductionSpeed float64   `json:"productionSpeed"`
	IsSolarActive   bool      `json:"isSolarActive"`
	LastUpdated     time.Time `json:"lastUpdated"`
}

// CalculateEnergyStatus computes a company's energy balance using GraphTraversal
func (l *EconomyLogic) CalculateEnergyStatus(companyId string) (EnergyStatus, error) {
	gt := NewGraphTraversal(l.app)

	balance, err := gt.CalculateEnergyBalance(companyId, nil)
	if err != nil {
		return EnergyStatus{}, err
	}

	// Map GraphTraversal.EnergyBalance to EnergyStatus
	status := EnergyStatus{
		EnergyProduced:  balance.Available,
		EnergyDemand:    balance.Demand,
		EnergyStored:    balance.StoredEnergy,
		MaxEnergyStored: balance.MaxStoredEnergy,
		EnergyRatio:     balance.Ratio,
		ProductionSpeed: balance.Ratio,
		IsSolarActive:   IsSolarProductionActive(), // Helper from graph_traversal.go
		LastUpdated:     time.Now(),
	}

	// To get pure "Produced" vs "Stored", I might need to inspect balance more closely.
	// But usually "Available" is what we display as "Source".
	// Or we split it.
	// In graph_traversal.go:
	// balance.Available += itemDef.ProduceEnergy.
	// balance.StoredEnergy += machine.GetFloat("stored_energy").
	// So they are distinct in the struct!
	status.EnergyProduced = balance.Available

	return status, nil
}
