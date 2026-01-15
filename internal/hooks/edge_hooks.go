package hooks

import (
	"fmt"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"ketsuna.com/server/internal/gamedata"
)

// RegisterEdgeRelationHooks registers hooks for edge_relation collection
// to maintain consistency between edges and machine/deposit assignments
func RegisterEdgeRelationHooks(app *pocketbase.PocketBase) {
	app.OnRecordAfterCreateSuccess("edge_relation").BindFunc(func(e *core.RecordEvent) error {
		inputType := e.Record.GetString("input_type")
		inputId := e.Record.GetString("input_id")
		outputType := e.Record.GetString("output_type")
		outputId := e.Record.GetString("output_id")

		// Case: Deposit -> Machine (extractor needs deposit field)
		if inputType == "deposit" && outputType == "machine" {
			machine, err := e.App.FindRecordById("machines", outputId)
			if err == nil {
				machine.Set("deposit", inputId)
				if err := e.App.Save(machine); err != nil {
					e.App.Logger().Error("[EDGE_HOOK] Failed to set deposit on machine", "err", err)
				} else {
					e.App.Logger().Debug("[EDGE_HOOK] Assigned deposit to machine", "machineId", outputId, "depositId", inputId)
				}
			}
		}

		// Case: Machine -> Deposit (assign employees to deposit mining)
		// (This case might not be common, but handle if needed)

		return e.Next()
	})

	// SECURITY: Validate ownership before creation request
	app.OnRecordCreateRequest("edge_relation").BindFunc(func(e *core.RecordRequestEvent) error {
		inputType := e.Record.GetString("input_type")
		outputType := e.Record.GetString("output_type")
		outputId := e.Record.GetString("output_id")
		inputId := e.Record.GetString("input_id")

		// Find the company_id from the nodes being connected
		var companyId string

		// Try to get company from input node
		if inputType == "machine" || inputType == "storage" {
			machine, err := e.App.FindRecordById("machines", inputId)
			if err == nil {
				companyId = machine.GetString("company")
			}
		} else if inputType == "deposit" {
			deposit, err := e.App.FindRecordById("deposits", inputId)
			if err == nil {
				companyId = deposit.GetString("company")
			}
		}

		// If not found via input, try via output node
		if companyId == "" {
			if outputType == "machine" || outputType == "storage" {
				machine, err := e.App.FindRecordById("machines", outputId)
				if err == nil {
					companyId = machine.GetString("company")
				}
			} else if outputType == "company" {
				companyId = outputId
			}
		}

		// Validate ownership (bypass for superuser)
		if e.Auth == nil || !e.Auth.IsSuperuser() {
			if err := ValidateCompanyOwnership(e.App, e.Auth.Id, companyId); err != nil {
				return err
			}
		}

		return e.Next()
	})

	// Validate BEFORE creation: Constraint check
	app.OnRecordCreate("edge_relation").BindFunc(func(e *core.RecordEvent) error {
		inputType := e.Record.GetString("input_type")
		outputType := e.Record.GetString("output_type")
		outputId := e.Record.GetString("output_id")
		inputId := e.Record.GetString("input_id")

		// FORBIDDEN EDGES: Enforce system constraints
		// 1. Company cannot be a SOURCE (input_type = "company" is invalid)
		if inputType == "company" {
			return apis.NewBadRequestError("L'entreprise ne peut pas être source d'une connexion. Les connexions doivent aller VERS l'entreprise, pas depuis.", nil)
		}

		// 2. Deposit cannot connect directly to Company (must go through machines)
		if inputType == "deposit" && outputType == "company" {
			return apis.NewBadRequestError("Les gisements ne peuvent pas être connectés directement à l'entreprise. Connectez d'abord à une machine extractrice, puis au stockage.", nil)
		}

		// 3. Deposit cannot connect directly to Storage (must go through machines)
		if inputType == "deposit" && outputType == "storage" {
			return apis.NewBadRequestError("Les gisements ne peuvent pas être connectés directement au stockage. Connectez d'abord à une machine extractrice.", nil)
		}

		// VALID EDGES: Allow and validate specific connections
		// 4. Deposit -> Machine (extractor)
		if inputType == "deposit" && outputType == "machine" {
			// 0. CHECK COMPATIBILITY
			machine, err := e.App.FindRecordById("machines", outputId)
			if err != nil {
				return err
			}
			deposit, err := e.App.FindRecordById("deposits", inputId)
			if err != nil {
				return err
			}

			itemDef := gamedata.GetItem(machine.GetString("machine_id"))
			if itemDef != nil {
				// Only check if it's an extractor (Product defined, No Recipe)
				// If it has a recipe, it shouldn't be connecting to a deposit anyway (enforced by graph logic now, but let's be safe)
				// Actually, graph logic "allows" connection but ignores it.
				// But we want to forbid the edge creation if it makes no sense.

				isExtractor := itemDef.Product != "" && itemDef.UseRecipe == ""
				if isExtractor {
					targetResource := deposit.GetString("ressource_id")
					if itemDef.Product != targetResource {
						return apis.NewBadRequestError(fmt.Sprintf("Incompatible : Cette machine extrait du '%s' mais le gisement est du '%s'", itemDef.Product, targetResource), nil)
					}
				}
			}

			// Check if this machine already has a deposit connected
			// We look for any EXISTING edge where output_id = machine AND input_type = deposit
			filter := fmt.Sprintf("output_id = '%s' && input_type = 'deposit'", outputId)
			existing, err := e.App.FindRecordsByFilter("edge_relation", filter, "", 1, 0)
			if err != nil {
				return err
			}

			if len(existing) > 0 {
				// Auto-replace behavior: Delete the existing edge(s)
				e.App.Logger().Debug("[EDGE_HOOK] Auto-replacing existing deposit connection", "machineId", outputId, "oldEdge", existing[0].Id)
				for _, oldEdge := range existing {
					if err := e.App.Delete(oldEdge); err != nil {
						e.App.Logger().Error("[EDGE_HOOK] Failed to delete old edge during replacement", "err", err)
						return err
					}
				}
				// Continue to allow creation of new edge
			}

			// Explicitly update machine to new deposit as requested
			// This ensures the machine points to the new deposit immediately
			// Machine/Deposit are already fetched above
			machine.Set("deposit", inputId)
			if err := e.App.Save(machine); err != nil {
				e.App.Logger().Error("[EDGE_HOOK] Failed to update machine deposit during replacement", "err", err)
			} else {
				e.App.Logger().Debug("[EDGE_HOOK] Updated machine deposit", "machineId", outputId, "newDepositId", inputId)
			}
		}
		return e.Next()
	})

	// SECURITY: Validate ownership before delete
	app.OnRecordDeleteRequest("edge_relation").BindFunc(func(e *core.RecordRequestEvent) error {
		record := e.Record

		// Find the company_id from the edge being deleted
		var companyId string

		inputType := record.GetString("input_type")
		inputId := record.GetString("input_id")
		outputType := record.GetString("output_type")
		outputId := record.GetString("output_id")

		// Try to get company from input node
		if inputType == "machine" || inputType == "storage" {
			machine, err := e.App.FindRecordById("machines", inputId)
			if err == nil {
				companyId = machine.GetString("company")
			}
		} else if inputType == "deposit" {
			deposit, err := e.App.FindRecordById("deposits", inputId)
			if err == nil {
				companyId = deposit.GetString("company")
			}
		}

		// If not found via input, try via output node
		if companyId == "" {
			if outputType == "machine" || outputType == "storage" {
				machine, err := e.App.FindRecordById("machines", outputId)
				if err == nil {
					companyId = machine.GetString("company")
				}
			} else if outputType == "company" {
				companyId = outputId
			}
		}

		// Validate ownership (bypass for superuser)
		if e.Auth == nil || !e.Auth.IsSuperuser() {
			if err := ValidateCompanyOwnership(e.App, e.Auth.Id, companyId); err != nil {
				return err
			}
		}

		return e.Next()
	})

	// SECURITY: Validate ownership before update
	app.OnRecordUpdateRequest("edge_relation").BindFunc(func(e *core.RecordRequestEvent) error {
		record := e.Record
		original := record.Original()

		// Prevent transferring edges between companies
		// (edges should not have a company field, but validate via related nodes)

		// Find the company_id from the edge nodes
		var companyId string

		inputType := original.GetString("input_type")
		inputId := original.GetString("input_id")

		// Get company from input node
		if inputType == "machine" || inputType == "storage" {
			machine, err := e.App.FindRecordById("machines", inputId)
			if err == nil {
				companyId = machine.GetString("company")
			}
		} else if inputType == "deposit" {
			deposit, err := e.App.FindRecordById("deposits", inputId)
			if err == nil {
				companyId = deposit.GetString("company")
			}
		}

		// Validate ownership (bypass for superuser)
		if e.Auth == nil || !e.Auth.IsSuperuser() {
			if err := ValidateCompanyOwnership(e.App, e.Auth.Id, companyId); err != nil {
				return err
			}
		}

		return e.Next()
	})

	app.OnRecordAfterDeleteSuccess("edge_relation").BindFunc(func(e *core.RecordEvent) error {
		inputType := e.Record.GetString("input_type")
		outputType := e.Record.GetString("output_type")
		outputId := e.Record.GetString("output_id")

		// Case: Deposit -> Machine (clear deposit field when edge deleted)
		if inputType == "deposit" && outputType == "machine" {
			machine, err := e.App.FindRecordById("machines", outputId)
			if err == nil {
				machine.Set("deposit", "")
				if err := e.App.Save(machine); err != nil {
					e.App.Logger().Error("[EDGE_HOOK] Failed to clear deposit on machine", "err", err)
				} else {
					e.App.Logger().Debug("[EDGE_HOOK] Cleared deposit from machine", "machineId", outputId)
				}
			}
		}

		return e.Next()
	})
}
