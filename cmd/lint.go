package cmd

import (
	"fmt"

	"github.com/arnavsurve/dropstep/internal"
	"github.com/arnavsurve/dropstep/internal/logging"
	"github.com/arnavsurve/dropstep/internal/validation"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
)

type LintCmd struct {
	Varfile string `help:"The YAML varfile for input variables." default:"dsvars.yml"`
	Workflow string `help:"The workflow configuration file." default:"dropstep.yml"`
}

func (l *LintCmd) Run() error {
	router := &logging.LoggerRouter{
		Sinks: []logging.LogSink{
			&logging.ConsoleSink{},
		},
	}

	logging.ConfigureGlobalLogger(router, "none", "validation")
	log.Logger = logging.BaseLogger

	log.Info().Msgf("Validating %s", l.Workflow)

	if err := godotenv.Load(); err != nil {
		log.Warn().Err(err).Msg("No .env file found, relying on real ENV")
	}

	// Load workflow YAML
	wf, err := internal.LoadWorkflowFromFile(l.Workflow)
	if err != nil {
		return fmt.Errorf("could not load workflow file: %w", err)
	}

	// Load varfile YAML
	varCtx, err := internal.ResolveVarfile(l.Varfile)
	if err != nil {
		return fmt.Errorf("could not resolve varfile: %w", err)
	}

	// Resolve and merge input vars into workflow file
	wf, err = internal.InjectVarsIntoWorkflow(wf, varCtx)
	if err != nil {
		return fmt.Errorf("could not resolve variables for workflow: %w", err)
	}

	// Validate each handler YAML definition
	if err := validation.ValidateWorkflowHandlers(wf); err != nil {
		return fmt.Errorf("error validating workflow steps: %w", err)
	}

	log.Info().Msg("Successfully validated configuration ✅")

	return nil
}