package hooks

import (
	"fmt"
	"math"
	"strings"
)

// ProductionDetail holds info about a machine's production for finance report
type ProductionDetail struct {
	Machine    string  `json:"machine"`
	Product    string  `json:"product"`
	Quantity   int     `json:"quantity"`
	UnitPrice  float64 `json:"unit_price"`
	TotalValue float64 `json:"total_value"`
}

// FinanceBreakdown matches the JSON structure returned by the JS endpoint
type FinanceBreakdown struct {
	Success   bool   `json:"success"`
	CompanyId string `json:"companyId"`
	Breakdown struct {
		ProductionValue    float64            `json:"production_value"`
		ReserveValue       float64            `json:"reserve_value"`
		TotalRevenue       float64            `json:"total_revenue"`
		MachineMaintenance float64            `json:"machine_maintenance"`
		Payroll            float64            `json:"payroll"`
		TotalCosts         float64            `json:"total_costs"`
		NetProfit          float64            `json:"net_profit"`
		HourlyProfit       float64            `json:"hourly_profit"`
		HasSalesEmployee   bool               `json:"has_sales_employee"`
		MachineCount       int                `json:"machine_count"`
		EmployeeCount      int                `json:"employee_count"`
		ProductionDetails  []ProductionDetail `json:"production_details"`
		DailyPayroll       float64            `json:"daily_payroll"`
	} `json:"breakdown"`
	Warning   *string `json:"warning"`
	DailyView struct {
		RevenueBase       float64 `json:"revenue_base"`
		RevenueEmployees  float64 `json:"revenue_employees"`
		CostMaintenance   float64 `json:"cost_maintenance"`
		CostPayroll       float64 `json:"cost_payroll"`
		TotalRevenue      float64 `json:"total_revenue"`
		TotalCost         float64 `json:"total_cost"`
		Profit            float64 `json:"profit"`
	} `json:"daily_view"`
	HourlyNet  float64 `json:"hourly_net"`
	DailyNet   float64 `json:"daily_net"`
	MonthlyNet float64 `json:"monthly_net"`
}

// CalculateCompanyFinance creates the financial report for a company
func (l *EconomyLogic) CalculateCompanyFinance(companyId string) (*FinanceBreakdown, error) {
	company, err := l.app.FindRecordById("companies", companyId)
	if err != nil {
		return nil, fmt.Errorf("company introuvable")
	}

	// 2. Load related data
	employees, _ := l.app.FindRecordsByFilter("employees", fmt.Sprintf("employer = '%s'", companyId), "", 0, 0)
	assignedMachines, _ := l.app.FindRecordsByFilter("machines", fmt.Sprintf("company = '%s'", companyId), "", 0, 0)


	// --- CALCUL DES COÛTS FIXES (par 24h) ---
	// A. Coût des machines (7€ par machine par 24h)
	monthlyMachineCost := float64(len(assignedMachines) * 7)

	// B. Coût salarial (30 fois le salaire quotidien car 24h = 1 mois)
	monthlyPayrollCost := 0.0
	hasSalesEmployee := false

	for _, emp := range employees {
		dailySalary := float64(emp.GetInt("salary"))
		monthlyPayrollCost += dailySalary * 30

		job := emp.GetString("poste")
		jobLower := strings.ToLower(job)
		if strings.Contains(jobLower, "vendeur") || strings.Contains(jobLower, "commercial") {
			hasSalesEmployee = true
		}
	}

	// --- CALCUL DE LA PRODUCTION (par 24h) ---
	monthlyProductionValue := 0.0
	productionDetails := []ProductionDetail{}

	// Calculer la production uniquement si on a les moyens de vendre
	if hasSalesEmployee || len(employees) == 0 {
		for _, assignment := range assignedMachines {
			machineItemId := assignment.GetString("machine")
			if machineItemId == "" {
				continue
			}

			machineItem, err := l.app.FindRecordById("items", machineItemId)
			if err != nil {
				continue
			}

			// Vérifier si la machine a un employé assigné
			assignedEmployeeIds := assignment.GetStringSlice("employees")
			hasEmployee := len(assignedEmployeeIds) > 0

			if !hasEmployee {
				continue // Pas de production sans employé
			}

			// Calcul efficacité
			totalEfficiency := 0.0
			for _, eid := range assignedEmployeeIds {
				// Find employee in loaded list
				for _, e := range employees {
					if e.Id == eid {
						eff := e.GetFloat("efficiency")
						if eff == 0 {
							eff = 1.0
						}
						totalEfficiency += eff
						break
					}
				}
			}
			if totalEfficiency <= 0 {
				continue
			}

			// Récupérer le produit et sa quantité
			productId := machineItem.GetString("product")
			if productId == "" {
				continue
			}

			productItem, err := l.app.FindRecordById("items", productId)
			if err != nil {
				continue
			}

			productQty := float64(machineItem.GetInt("product_quantity"))
			if productQty == 0 {
				productQty = 1
			}

			// Apply efficiency
			productQty = math.Floor(productQty * totalEfficiency)

			// Prix de vente = base_price / 2
			basePrice := productItem.GetFloat("base_price")
			sellPrice := basePrice / 2

			// Production sur 24h (1440 minutes réelles = 24h de jeu)
			monthlyProductionQty := int(productQty * 1440)
			monthlyRevenue := float64(monthlyProductionQty) * sellPrice

			monthlyProductionValue += monthlyRevenue

			productionDetails = append(productionDetails, ProductionDetail{
				Machine:    machineItem.GetString("name"),
				Product:    productItem.GetString("name"),
				Quantity:   monthlyProductionQty,
				UnitPrice:  sellPrice,
				TotalValue: monthlyRevenue,
			})
		}
	}

	// --- CALCUL VALEUR RESERVE (Liquidation 24h) ---
	reserveRevenue := 0.0
	reserves, _ := l.app.FindRecordsByFilter("reserve", fmt.Sprintf("company='%s'", companyId), "", 0, 0)
	for _, res := range reserves {
		itemId := res.GetString("item")
		qty := res.GetInt("quantity")
		if qty > 0 {
			item, err := l.app.FindRecordById("items", itemId)
			if err == nil {
				reserveRevenue += float64(qty) * item.GetFloat("base_price")
			}
		}
	}

	// --- AGRÉGATION FINALE ---
	totalMonthlyRevenue := monthlyProductionValue + reserveRevenue
	totalMonthlyCosts := monthlyMachineCost + monthlyPayrollCost
	monthlyNetProfit := totalMonthlyRevenue - totalMonthlyCosts

	// Calcul du profit horaire (pour l'affichage)
	hourlyNetProfit := monthlyNetProfit / 24

	var warning *string
	if !hasSalesEmployee && len(employees) > 0 {
		w := "Aucun employé commercial détecté. Les revenus de production ne sont pas comptabilisés."
		warning = &w
	}

	resp := &FinanceBreakdown{
		Success:    true,
		CompanyId:  companyId,
		Warning:    warning,
		HourlyNet:  hourlyNetProfit,
		DailyNet:   monthlyNetProfit / 30,
		MonthlyNet: monthlyNetProfit,
	}

	resp.Breakdown.ProductionValue = monthlyProductionValue
	resp.Breakdown.ReserveValue = reserveRevenue
	resp.Breakdown.TotalRevenue = totalMonthlyRevenue
	resp.Breakdown.MachineMaintenance = monthlyMachineCost
	resp.Breakdown.Payroll = monthlyPayrollCost
	resp.Breakdown.TotalCosts = totalMonthlyCosts
	resp.Breakdown.NetProfit = monthlyNetProfit
	resp.Breakdown.HourlyProfit = hourlyNetProfit
	resp.Breakdown.HasSalesEmployee = hasSalesEmployee
	resp.Breakdown.MachineCount = len(assignedMachines)
	resp.Breakdown.EmployeeCount = len(employees)
	resp.Breakdown.ProductionDetails = productionDetails
	resp.Breakdown.DailyPayroll = monthlyPayrollCost

	resp.DailyView.RevenueEmployees = monthlyProductionValue + reserveRevenue
	resp.DailyView.CostMaintenance = monthlyMachineCost
	resp.DailyView.CostPayroll = monthlyPayrollCost
	resp.DailyView.TotalRevenue = totalMonthlyRevenue
	resp.DailyView.TotalCost = totalMonthlyCosts
	resp.DailyView.Profit = monthlyNetProfit

	return resp, nil
}
