package cmd

import (
	"fmt"

	"github.com/arnavsurve/dropstep/internal"
	"github.com/arnavsurve/dropstep/internal/handlers"
	"github.com/arnavsurve/dropstep/internal/logging"
	"github.com/arnavsurve/dropstep/internal/validation"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
)

type RunCmd struct {
	Varfile string `help:"The YAML varfile for input variables." default:"dsvars.yml"`
	Workflow string `help:"The workflow configuration file." default:"dropstep.yml"`
}

func (r *RunCmd) Run() error {
	if err := godotenv.Load(); err != nil {
		log.Warn().Err(err).Msg("No .env file found, relying on real ENV")
	}

	// Load workflow YAML
	wf, err := internal.LoadWorkflowFromFile(r.Workflow)
	if err != nil {
		return fmt.Errorf("could not load workflow file: %w", err)
	}

	// Generate workflow run UUID
	wfRunID := uuid.New().String()

	// Load varfile YAML
	varCtx, err := internal.ResolveVarfile(r.Varfile)
	if err != nil {
		log.Warn().Err(err).Msg("Could not resolve varfile, proceeding without global variables")
		varCtx = make(internal.VarContext)
	}

	// Validate each handler YAML definition
	if err := validation.ValidateWorkflowHandlers(wf); err != nil {
		return fmt.Errorf("error validating workflow steps: %w", err)
	}

	// Initialize file logger
	fileSink, err := logging.NewFileSink("out.json") 
	if err != nil {
		return fmt.Errorf("could not create file log sink: %w", err)
	}

	// Initialize logger router
	router := &logging.LoggerRouter{
		Sinks: []logging.LogSink{
			&logging.ConsoleSink{},
			fileSink,
		},
	}

	// Graceful shutdown of logging sinks
	defer func() {
		fmt.Println("Shutting down logger...")
		if err := router.Close(); err != nil {
			fmt.Printf("Error during log shutdown: %v", err)
		}
	}()

	logging.ConfigureGlobalLogger(router, wf.Name, wfRunID)
	log.Logger = logging.BaseLogger

	log.Info().Msg("Initialized workflow logger")
	log.Info().Msgf("Starting workflow: %q (run ID: %s)", wf.Name, wfRunID)

	stepResults := make(internal.StepResultsContext)

	// Run handlers
	for _, step := range wf.Steps {
		logging.BaseLogger.Info().Msgf("Running step %q (uses=%s)", step.ID, step.Uses)

		resolvedStep, err := internal.ResolveStepVariables(&step, varCtx, stepResults)
		if err != nil {
			return fmt.Errorf("could not resolve variales for step %q: %w", step.ID, err)
		}

		scopedLogger := logging.ScopedLogger(resolvedStep.ID, resolvedStep.Uses)
		ctx := internal.ExecutionContext{
			Step: *resolvedStep,
			Logger: &scopedLogger,
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