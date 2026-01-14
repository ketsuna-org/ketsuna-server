package hooks

import (
	"fmt"
)

func (l *EconomyLogic) DeductDailyPayroll() {
	companies, _ := l.app.FindRecordsByFilter("companies", "", "", 0, 0)
	for _, company := range companies {
		employees, _ := l.app.FindRecordsByFilter("employees", fmt.Sprintf("employer = '%s'", company.Id), "", 0, 0)
		machines, _ := l.app.FindRecordsByFilter("machines", fmt.Sprintf("company = '%s'", company.Id), "", 0, 0)

		monthlyCost := 0.0

		for _, emp := range employees {
			monthlyCost += float64(emp.GetInt("salary") * 30)
		}

		monthlyCost += float64(len(machines) * 7)

		currentBalance := company.GetFloat("balance")
		newBalance := currentBalance - monthlyCost
		if newBalance < 0 {
			newBalance = 0
		}

		company.Set("balance", newBalance)
		l.app.Save(company)
		if monthlyCost > 0 {
			RecordCompanyStatistic(l.app, company.Id, "payroll", "money_out", monthlyCost) // Using "payroll" as item_id
		}

		if monthlyCost > 0 {
			l.app.Logger().Debug("[PAYROLL] Deducted", "company", company.GetString("name"), "amount", monthlyCost, "employees", len(employees), "machines", len(machines))
		}
	}
}
