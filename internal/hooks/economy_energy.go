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
