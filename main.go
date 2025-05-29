package main

import (
	"flag"
	"fmt"
	"github.com/arnavsurve/dropstep/internal"
	"github.com/arnavsurve/dropstep/internal/handlers"
	"github.com/joho/godotenv"
	"log"
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

	for _, step := range wf.Steps {
		fmt.Printf("==> Running step %q (uses=%s)\n", step.ID, step.Uses)

		ctx := internal.ExecutionContext{
			Step: step,
			// logger, db conn goes here
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
