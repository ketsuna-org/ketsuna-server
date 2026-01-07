package hooks

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
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
					e.App.Logger().Info("[EDGE_HOOK] Assigned deposit to machine", "machineId", outputId, "depositId", inputId)
				}
			}
		}

		// Case: Machine -> Deposit (assign employees to deposit mining)
		// (This case might not be common, but handle if needed)

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
					e.App.Logger().Info("[EDGE_HOOK] Cleared deposit from machine", "machineId", outputId)
				}
			}
		}

		return e.Next()
	})
}
