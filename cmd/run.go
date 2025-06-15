package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/arnavsurve/dropstep/internal"
	"github.com/arnavsurve/dropstep/internal/handlers"
	"github.com/arnavsurve/dropstep/internal/logging"
	"github.com/arnavsurve/dropstep/internal/validation"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
)

type RunCmd struct {
	Varfile  string `help:"The YAML varfile for input variables." default:"dsvars.yml"`
	Workflow string `help:"The workflow configuration file." default:"dropstep.yml"`
}

func (r *RunCmd) Run() error {
	// Initialize file logger
	fileSink, err := logging.NewFileSink("out.json")
	if err != nil {
		return fmt.Errorf("could not create file log sink: %w", err)
	}

	// Configure log router
	router := &logging.LoggerRouter{
		Sinks: []logging.LogSink{
			&logging.ConsoleSink{},
			fileSink,
		},
	}

	// Configure base logger with placeholder values before loading workflow values
	logging.ConfigureGlobalLogger(router, "pre-init", "pre-init")
	log.Logger = logging.GlobalLogger

	// Load .env
	if err := godotenv.Load(); err != nil {
		log.Warn().Err(err).Msg("No .env file found, relying on real ENV")
	}

	// Load original workflow YAML
	originalWf, err := internal.LoadWorkflowFromFile(r.Workflow)
	if err != nil {
		return fmt.Errorf("could not load workflow file: %w", err)
	}

	// Get the workflow directory
	workflowAbsPath, err := filepath.Abs(r.Workflow)
	if err != nil {
		return fmt.Errorf("could not determine absolute path for workflow file: %w", err)
	}
	workflowDir := filepath.Dir(workflowAbsPath)

	// Load varfile YAML
	varCtx, err := internal.ResolveVarfile(r.Varfile)
	if err != nil {
		log.Warn().Err(err).Msg("Could not resolve varfile, proceeding without global variables")
		varCtx = make(internal.VarContext)
	}

	// Create a temporary, resolved copy of the workflow for validation
	validationWf, err := internal.InjectVarsIntoWorkflow(originalWf, varCtx)
	if err != nil {
		return fmt.Errorf("could not resolve global variables for workflow validation: %w", err)
	}

	// Validate the handlers using the temporary workflow
	if err := validation.ValidateWorkflowHandlers(validationWf, workflowDir); err != nil {
		return fmt.Errorf("error validating workflow steps: %w", err)
	}

	// Generate workflow run UUID
	wfRunID := uuid.New().String()

	// Graceful shutdown of logging sinks
	defer func() {
		fmt.Println("Shutting down logger...")
		if err := router.Close(); err != nil {
			fmt.Printf("Error during log shutdown: %v", err)
		}
	}()

	// Update the global logger values
	logging.ConfigureGlobalLogger(router, originalWf.Name, wfRunID)
	log.Logger = logging.GlobalLogger

	log.Info().Msg("Initialized workflow logger")
	log.Info().Msgf("Starting workflow: %q (run ID: %s)", originalWf.Name, wfRunID)

	stepResults := make(internal.StepResultsContext)

	// Run handlers using the original workflow object
	for _, step := range originalWf.Steps {
		log.Info().Msgf("Running step %q (uses=%s)", step.ID, step.Uses)

		resolvedStep, err := internal.ResolveStepVariables(&step, varCtx, stepResults)
		if err != nil {
			return fmt.Errorf("could not resolve variables for step %q: %w", step.ID, err)
		}

		scopedLogger := logging.ScopedLogger(resolvedStep.ID, resolvedStep.Uses)
		ctx := internal.ExecutionContext{
			Step:        *resolvedStep,
			Logger:      &scopedLogger,
			WorkflowDir: workflowDir,
		}

		handler, err := handlers.GetHandler(ctx)
		if err != nil {
			return fmt.Errorf("error getting handler for step %q: %w", resolvedStep.ID, err)
		}

		result, err := handler.Run()
		if err != nil {
			return fmt.Errorf("error running step %q: %w", resolvedStep.ID, err)
		}

		if result != nil {
			log.Debug().Interface("result", result).Msgf("Storing result for step %q", resolvedStep.ID)
			stepResults[resolvedStep.ID] = *result
		}
	}

	log.Info().Msg("Workflow completed successfully.")
	return nil
}
