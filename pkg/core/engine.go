package core

import (
	"fmt"

	"github.com/arnavsurve/dropstep/pkg/steprunner"
	"github.com/arnavsurve/dropstep/pkg/types"
)

type WorkflowEngine struct {
	Logger Logger
}

func NewWorkflowEngine(logger Logger) *WorkflowEngine {
	return &WorkflowEngine{
		Logger: logger,
	}
}

func (e *WorkflowEngine) ExecuteWorkflow(
	wf *Workflow,
	varCtx VarContext,
	initialStepResults StepResultsContext,
	workflowDir string,
	resolvedProviders map[string]ProviderConfig,
	// APIKeyGetter func(providerType string) string,
) (StepResultsContext, error) {
	stepResults := initialStepResults
	if stepResults == nil {
		stepResults = make(StepResultsContext)
	}

	for _, step := range wf.Steps {
		e.Logger.Info().Msgf("Running step %q (uses=%s)", step.ID, step.Uses)

		resolvedStep, err := ResolveStepVariables(&step, varCtx, stepResults)
		if err != nil {
			return stepResults, fmt.Errorf("could not resolve variables for step %q: %w", step.ID, err)
		}

		scopedLogger := e.Logger.With().Str("step_id", resolvedStep.ID).Str("step_type", resolvedStep.Uses).Logger()

		execCtx := types.ExecutionContext{
			Step:        *resolvedStep,
			Logger:      scopedLogger,
			WorkflowDir: workflowDir,
		}

		if resolvedStep.Uses == "browser_agent" {
			providerConf, found := resolvedProviders[resolvedStep.Provider]
			if !found {
				return stepResults, fmt.Errorf("step %q references provider %q, which is not defined in providers", resolvedStep.ID, resolvedStep.Provider)
			}

			execCtx.APIKey = providerConf.APIKey
			if execCtx.APIKey == "" {
				return stepResults, fmt.Errorf("API key for provider %q is empty", resolvedStep.Provider)
			}
		}

		runner, err := steprunner.GetRunner(execCtx)
		if err != nil {
			return stepResults, fmt.Errorf("error getting runner for step %q: %w", resolvedStep.ID, err)
		}

		result, err := runner.Run()
		if err != nil {
			return stepResults, fmt.Errorf("error running step %q: %w", resolvedStep.ID, err)
		}

		if result != nil {
			e.Logger.Debug().Msgf("Storing result for step %q", resolvedStep.ID)
			stepResults[resolvedStep.ID] = *result
		}
	}

	return stepResults, nil
}
