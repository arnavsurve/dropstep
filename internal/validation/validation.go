package validation

import (
	"fmt"

	"github.com/arnavsurve/dropstep/internal"
	"github.com/arnavsurve/dropstep/internal/handlers"
	"github.com/arnavsurve/dropstep/internal/logging"
)

func ValidateWorkflowHandlers(wf *internal.Workflow) error {
	for _, step := range wf.Steps {
		logging.BaseLogger.Info().Msgf("Validating step %q (uses=%s)", step.ID, step.Uses)

		scopedLogger := logging.ScopedLogger(step.ID, step.Uses)
		ctx := internal.ExecutionContext{
			Step: step,
			Logger: &scopedLogger,
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