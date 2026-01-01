package hooks

import (
	"fmt"
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

func IsSolarProductionActive() bool {
	now := time.Now().UTC()
	hour := now.Hour()
	// Solar active between 08:00 and 19:00 UTC (09:00-20:00 local roughly)
	return hour >= 8 && hour < 19
}

// CalculateEnergyStatus computes a company's energy balance
func (l *EconomyLogic) CalculateEnergyStatus(companyId string) (EnergyStatus, error) {
	status := EnergyStatus{
		ProductionSpeed: 1.0, // Default to full speed if no energy demand
		EnergyRatio:     1.0,
		IsSolarActive:   IsSolarProductionActive(),
	}

	// Fetch all machines assigned to company
	// Note: We might want to optimize this by only fetching necessary fields
	machines, err := l.app.FindRecordsByFilter(
		"machines",
		fmt.Sprintf("company = '%s'", companyId),
		"",
		1000,
		0,
	)
	if err != nil {
		return status, err
	}

	if len(machines) == 0 {
		return status, nil
	}

	// Expand 'machine' item relation to get energy stats
	for _, m := range machines {
		l.app.ExpandRecord(m, []string{"machine", "employees"}, nil)
	}

	// We need employee efficiency data... it's complicated because efficiency is on Employee record.
	// But `ExpandRecord` "employees" gives us the Employee records.

	// Pre-fetch all employees to avoid N+1 is hard with Expand.
	// Actually Expand "employees" (relation multiple) works.

	for _, assignment := range machines {
		machineItem := assignment.ExpandedOne("machine")
		if machineItem == nil {
			continue
		}

		employees := assignment.ExpandedAll("employees")

		// Calculate efficiency from assigned employees
		// Assuming 'employees' relation contains the assigned employees
		totalEfficiency := 0.0
		for _, emp := range employees {
			totalEfficiency += emp.GetFloat("efficiency")
		}
		if totalEfficiency <= 0 {
			totalEfficiency = 1.0 // Default for machines without employees
		}

		// Check if this machine produces energy
		produceEnergy := machineItem.GetFloat("produce_energy")
		if produceEnergy > 0 {
			energyType := machineItem.GetString("energy_type")

			// Solar only produces during 8h-18h UTC
			if energyType == "Soleil" && !status.IsSolarActive {
				// Solar panel inactive, no production
			} else {
				// Base Idea: Efficiency multiplies production
				// If 1 employee with 1.2 eff -> 1.2 * Base
				status.EnergyProduced += produceEnergy * totalEfficiency
			}
		}

		// Check if this machine stores energy
		canStoreEnergy := machineItem.GetFloat("can_store_energy")
		if canStoreEnergy > 0 {
			status.MaxEnergyStored += canStoreEnergy
			status.EnergyStored += assignment.GetFloat("stored_energy")
		}

		// Check if this machine consumes energy
		needEnergy := machineItem.GetFloat("need_energy")
		if needEnergy > 0 {
			energyType := machineItem.GetString("energy_type")
			// Solar-powered machines don't consume grid electricity
			if energyType != "Soleil" {
				// Calculate Cycle Duration to normalize Power (Energy/Time)
				cycleDuration := 1.0 // Default 1 second (Energy is per second)
				
				// 1. Check Recipe
				recipeId := machineItem.GetString("use_recipe")
				if recipeId != "" {
					recipe, err := l.app.FindRecordById("recipes", recipeId)
					if err == nil {
						dur := recipe.GetFloat("production_time")
						if dur > 1 {
							cycleDuration = dur
						}
					}
				} else {
					// 2. Check Machine Base Production Time (Passive)
					dur := machineItem.GetFloat("production_time")
					if dur > 1 {
						cycleDuration = dur
					}
				}

				effectiveDemand := needEnergy / cycleDuration
				status.EnergyDemand += effectiveDemand
			}
		}
	}

	// Cap stored energy at max
	if status.EnergyStored > status.MaxEnergyStored {
		status.EnergyStored = status.MaxEnergyStored
	}

	// Calculate energy ratio
	if status.EnergyDemand > 0 {
		// Available energy = production + stored buffer
		availableEnergy := status.EnergyProduced + status.EnergyStored
		status.EnergyRatio = availableEnergy / status.EnergyDemand

		if status.EnergyRatio > 1.0 {
			status.EnergyRatio = 1.0
		}
		if status.EnergyRatio < 0.0 {
			status.EnergyRatio = 0.0
		}

		status.ProductionSpeed = status.EnergyRatio
	}

	return status, nil
}
