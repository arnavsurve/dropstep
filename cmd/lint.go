package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/arnavsurve/dropstep/internal"
	"github.com/arnavsurve/dropstep/internal/logging"
	"github.com/arnavsurve/dropstep/internal/validation"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
)

type LintCmd struct {
	Varfile  string `help:"The YAML varfile for input variables." default:"dsvars.yml"`
	Workflow string `help:"The workflow configuration file." default:"dropstep.yml"`
}

func (l *LintCmd) Run() error {
	router := &logging.LoggerRouter{
		Sinks: []logging.LogSink{
			&logging.ConsoleSink{},
		},
	}

	logging.ConfigureGlobalLogger(router, "none", "validation")
	log.Logger = logging.GlobalLogger

	log.Info().Msgf("Validating %s", l.Workflow)

	if err := godotenv.Load(); err != nil {
		log.Warn().Err(err).Msg("No .env file found, relying on real ENV")
	}

	// Load workflow YAML
	wf, err := internal.LoadWorkflowFromFile(l.Workflow)
	if err != nil {
		return fmt.Errorf("could not load workflow file: %w", err)
	}

	// Get the workflow directory
	workflowAbsPath, err := filepath.Abs(l.Workflow)
	if err != nil {
		return fmt.Errorf("could not determine absolute path for workflow file: %w", err)
	}
	workflowDir := filepath.Dir(workflowAbsPath)

	// Load varfile YAML
	varCtx, err := internal.ResolveVarfile(l.Varfile)
	if err != nil {
		// For linting, a missing varfile is not a fatal error
		log.Warn().Err(err).Msg("Could not resolve varfile, proceeding without global variables")
		varCtx = make(internal.VarContext)
	}

	// Validate required inputs
	if err := validation.ValidateRequiredInputs(wf, varCtx); err != nil {
		return err
	}

	// Resolve and merge input vars into workflow file (globals only for linting)
	wf, err = internal.InjectVarsIntoWorkflow(wf, varCtx)
	if err != nil {
		return fmt.Errorf("could not resolve global variables for workflow: %w", err)
	}

	// Validate each handler YAML definition
	if err := validation.ValidateWorkflowHandlers(wf, workflowDir); err != nil {
		return fmt.Errorf("error validating workflow steps: %w", err)
	}

	log.Info().Msg("Successfully validated configuration ✅")

	return nil
}
