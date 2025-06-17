package validation

import (
	"fmt"

	"github.com/arnavsurve/dropstep/internal"
	"github.com/arnavsurve/dropstep/internal/handlers"
	"github.com/arnavsurve/dropstep/internal/logging"
)

func ValidateRequiredInputs(wf *internal.Workflow, varCtx internal.VarContext) error {
	for _, input := range wf.Inputs {
		if input.Required {
			if _, exists := varCtx[input.Name]; !exists {
				return fmt.Errorf("required input %q is missing from the varfile", input.Name)
			}
		}
	}
	return nil
}

func ValidateWorkflowHandlers(wf *internal.Workflow, workflowDir string) error {
	for _, step := range wf.Steps {
		logging.GlobalLogger.Info().Msgf("Validating step %q (uses=%s)", step.ID, step.Uses)

		scopedLogger := logging.ScopedLogger(step.ID, step.Uses)
		ctx := internal.ExecutionContext{
			Step:        step,
			Logger:      &scopedLogger,
			WorkflowDir: workflowDir,
		}

		handler, err := handlers.GetHandler(ctx)
		if err != nil {
			return fmt.Errorf("error getting handler for step %q: %w", step.ID, err)
		}

		if err = handler.Validate(); err != nil {
			return fmt.Errorf("error validating step %q: %w", step.ID, err)
		}
	}

	return nil
}
