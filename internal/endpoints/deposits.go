package endpoints

import (
	"fmt"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func registerDepositEndpoints(app *pocketbase.PocketBase, e *core.ServeEvent) {
	// NOTE: Deposit employee assignment endpoints have been removed.
	// Employees are no longer used in the mining system - only for explorations.
	// Deposits are now pure sources for mining machines via edges.

	// POST /api/deposits/sell - Sell a deposit for 1$ (permanently removes it)
	e.Router.POST("/api/deposits/sell", func(c *core.RequestEvent) error {
		authRecord := c.Auth
		if authRecord == nil {
			return apis.NewUnauthorizedError("Non connecté", nil)
		}

		type req struct {
			DepositId string `json:"depositId"`
		}
		var data req
		if err := c.BindBody(&data); err != nil {
			return apis.NewBadRequestError("Données invalides", err)
		}

		if data.DepositId == "" {
			return apis.NewBadRequestError("depositId requis", nil)
		}

		deposit, err := app.FindRecordById("deposits", data.DepositId)
		if err != nil {
			return apis.NewBadRequestError("Gisement introuvable", err)
		}

		companyId := authRecord.GetString("active_company")
		if deposit.GetString("company") != companyId {
			return apis.NewForbiddenError("Ce gisement ne vous appartient pas", nil)
		}

		return app.RunInTransaction(func(txApp core.App) error {
			// 1. Delete all edges connected to this deposit
			edges, _ := txApp.FindRecordsByFilter("edge_relation",
				fmt.Sprintf("input_id = '%s' || output_id = '%s'", data.DepositId, data.DepositId),
				"", 0, 0)
			for _, edge := range edges {
				txApp.Delete(edge)
			}

			// 2. Delete the deposit record
			if err := txApp.Delete(deposit); err != nil {
				return apis.NewBadRequestError("Erreur lors de la suppression", err)
			}

			// 3. Add +1$ to company balance
			company, err := txApp.FindRecordById("companies", companyId)
			if err != nil {
				return apis.NewBadRequestError("Entreprise introuvable", err)
			}
			currentBalance := company.GetFloat("balance")
			company.Set("balance", currentBalance+1)
			if err := txApp.Save(company); err != nil {
				return apis.NewBadRequestError("Erreur mise à jour solde", err)
			}

			return c.JSON(200, map[string]interface{}{
				"success":    true,
				"message":    "Gisement vendu pour 1 ₭",
				"newBalance": currentBalance + 1,
			})
		})
	})
}
