package main

import (
	"flag"
	"log"

	"github.com/arnavsurve/dropstep/internal"
	"github.com/arnavsurve/dropstep/internal/handlers"
	"github.com/arnavsurve/dropstep/internal/logging"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("warning: no .env file found, relying on real ENV: %v", err)
	}

	varfilePtr := flag.String("varfile", "dsvars.yml", "The varfile for workflow inputs.")
	flag.Parse()

	// Load workflow YAML
	wf, err := internal.LoadWorkflowFromFile("dropstep.yml")
	if err != nil {
		log.Fatal(err)
	}

	// Generate workflow run UUID
	wfRunID := uuid.New().String()

	// Load varfile YAML
	varCtx, err := internal.ResolveVarfile(*varfilePtr)
	if err != nil {
		log.Fatal(err)
	}

	// Merge inputs
	wf, err = internal.InjectVarsIntoWorkflow(wf, varCtx)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize file logger
	fileSink, err := logging.NewFileSink("out.json") 
	if err != nil {
		log.Fatal(err)
	}

	// Initialize logger router with sinks and global logger
	router := &logging.LoggerRouter{
		Sinks: []logging.LogSink{
			&logging.ConsoleSink{},
			fileSink,
		},
	}
	logging.ConfigureGlobalLogger(router, wf.Name, wfRunID)

	logging.BaseLogger.Info().Msg("Initialized workflow logger")

	for _, step := range wf.Steps {
		logging.BaseLogger.Info().Msgf("Running step %q (uses=%s)", step.ID, step.Uses)

		scopedLogger := logging.ScopedLogger(step.ID, step.Uses)
		ctx := internal.ExecutionContext{
			Step: step,
			Logger: &scopedLogger,
		}

		handler, err := handlers.GetHandler(ctx)
		if err != nil {
			log.Fatalf("%v", err)
		}

		if err = handler.Validate(); err != nil {
			log.Fatalf("Error validating step %q: %v", step.ID, err)
		}

		if err = handler.Run(); err != nil {
			log.Fatalf("Error running step %q: %v", step.ID, err)
		}
	}
}
