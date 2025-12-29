package hooks

import (
	"math/rand"
	"time"

	"github.com/pocketbase/pocketbase"
)

// package-local RNG (preferred over global rand.Seed as of Go 1.20)
var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

// RegisterHooks registers a subset of game hooks (companies, employees, inventory, recipes)
// and starts a simple economy ticker (cron-like) to simulate the JS hooks behavior.
func RegisterHooks(app *pocketbase.PocketBase) {
	invLogic := NewInventoryLogic(app)
	ecoLogic := NewEconomyLogic(app, invLogic)

	registerCompanyHooks(app)
	registerEmployeeHooks(app)
	registerCompanyTechHooks(app)
	registerStockHooks(app)
	registerShareholderHooks(app)
	registerInventoryHooks(app)
	registerRecipeHooks(app)
	registerMachineHooks(app, invLogic)

	// Register API Endpoints
	RegisterEndpoints(app, invLogic, ecoLogic)

	// Start Background Jobs
	startEconomyTicker(app, ecoLogic)
}

func startEconomyTicker(app *pocketbase.PocketBase, eco *EconomyLogic) {
	// Start minute ticker for production
	go func() {
		t := time.NewTicker(1 * time.Minute)
		defer t.Stop()
		for range t.C {
			// Logic moved to economy_cron.go, but we need to fetch companies here to call logic on them
			// Or we could move the loop to economy_cron.go?
			// Let's keep the loop here for now as it's the "Cron" entrypoint, calling logic.
			// Actually, let's keep it simple.
			// We can export a function `StartEconomyBackgroundJobs(app, eco)`?
			// But for now, just inline the calls to the new methods.

			companies, err := app.FindRecordsByFilter("companies", "", "", 0, 0)
			if err == nil {
				for _, c := range companies {
					eco.ProcessCompanyEconomy(c.Id)
				}
			}
			eco.UpdateStockPrices()
		}
	}()

	// Start daily ticker for market prices
	go func() {
		t := time.NewTicker(24 * time.Hour)
		defer t.Stop()
		for range t.C {
			eco.UpdateMarketPrices()
			eco.DeductDailyPayroll()
		}
	}()
}
