package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/arnavsurve/dropstep/pkg/core"
	"github.com/arnavsurve/dropstep/pkg/log"
	"github.com/arnavsurve/dropstep/pkg/log/sinks"
	"github.com/arnavsurve/dropstep/pkg/steprunner"
	"github.com/arnavsurve/dropstep/pkg/types"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"

	// Ensure all runner implementations are initialized
	_ "github.com/arnavsurve/dropstep/pkg/steprunner/runners"
)

type LintCmd struct {
	Varfile  string `help:"The YAML varfile for input variables." default:"dsvars.yml"`
	Workflow string `help:"The workflow configuration file." default:"dropstep.yml"`
}

func (l *LintCmd) Run() error {
	consoleSink := sinks.NewConsoleSink()

	logRouter := log.NewRouter()
	logRouter.AddSink(consoleSink)

	routerWriter := logRouter
	baseZerologInstance := zerolog.New(routerWriter).With().Timestamp().Logger()
	cmdLogger := log.NewZerologAdapter(baseZerologInstance)

	cmdLogger.Info().Msgf("Validating %s using %s", l.Workflow, l.Varfile)

	// Load .env
	if err := godotenv.Load(); err != nil {
		cmdLogger.Warn().Err(err).Msgf("No .env file found or error thrown while loading it. Relying on existing ENV if vars use {{ env.* }}")
	}

	// Load original workflow YAML
	wf, err := core.LoadWorkflowFromFile(l.Workflow)
	if err != nil {
		cmdLogger.Error().Err(err).Msgf("Failed to load workflow file %s", l.Workflow)
		return fmt.Errorf("could not load workflow file %q: %w", l.Workflow, err)
	}
	cmdLogger.Info().Msgf("Successfully loaded workflow: %s", wf.Name)

	// Get the workflow directory
	workflowAbsPath, err := filepath.Abs(l.Workflow)
	if err != nil {
		cmdLogger.Error().Err(err).Msgf("Could not determine absolute path for workflow file %s", l.Workflow)
		return fmt.Errorf("could not determine absolute path for workflow file %q: %w", l.Workflow, err)
	}
	workflowDir := filepath.Dir(workflowAbsPath)

	// Load varfile YAML
	var varCtx core.VarContext
	if _, statErr := os.Stat(l.Varfile); os.IsNotExist(statErr) {
		cmdLogger.Warn().Msgf("Varfile %s not found. Proceeding without global variables. Required inputs might fail validation if not in ENV.", l.Varfile)
		varCtx = make(core.VarContext)
	} else {
		varCtx, err = core.ResolveVarfile(l.Varfile)
		if err != nil {
			cmdLogger.Warn().Err(err).Msgf("Could not fully resolve varfile %q. Some variable validations might be affected.", l.Varfile)
			if varCtx == nil {
				varCtx = make(core.VarContext)
			}
		} else {
			cmdLogger.Info().Msgf("Successfully loaded and resolved varfile: %s", l.Varfile)
		}
	}

	// Validate required input variables
	if err := core.ValidateRequiredInputs(wf, varCtx); err != nil {
		cmdLogger.Error().Err(err).Msgf("Required input validation failed")
		return err
	}
	cmdLogger.Info().Msgf("Required input validation passed")

	// Create a temporary, resolved copy of the workflow for validation
	validationWf, err := core.InjectVarsIntoWorkflow(wf, varCtx)
	if err != nil {
		cmdLogger.Error().Err(err).Msg("Could not resolve global variables for workflow validation")
		return fmt.Errorf("could not resolve global variables for workflow: %w", err)
	}

	cmdLogger.Info().Msgf("Starting validation of individual steps...")
	for _, stepConfig := range validationWf.Steps {
		stepLogger := cmdLogger.With().
			Str("step_id", stepConfig.ID).
			Str("step_uses", stepConfig.Uses).
			Logger()

		stepLogger.Info().Msg("Validating step configuration...")

		execCtx := types.ExecutionContext{
			Step:        stepConfig,
			Logger:      stepLogger,
			WorkflowDir: workflowDir,
		}

		runner, err := steprunner.GetRunner(execCtx)
		if err != nil {
			stepLogger.Error().Err(err).Msg("Error getting runner for step")
			return fmt.Errorf("error getting runner for step %q: %w", stepConfig.ID, err)
		}

		if err := runner.Validate(); err != nil {
			stepLogger.Error().Err(err).Msg("Step configuration validation failed")
			return fmt.Errorf("validation failed for step %q (uses: %s): %w", stepConfig.ID, stepConfig.Uses, err)
		}

		stepLogger.Info().Msgf("Step configuration validation passed")
	}

	cmdLogger.Info().Msg("Successfully validated workflow configuration ✅")
	return nil
}
