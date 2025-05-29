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

		handler, err := handlers.GetHandler(step.Uses)
		if err != nil {
			log.Fatalf("%v", err)
		}

		err = handler.Validate(step)
		if err != nil {
			log.Fatalf("Error validating step %q: %v", step.ID, err)
		}

		err = handler.Run(step)
		if err != nil {
			log.Fatalf("Error running step %q: %v", step.ID, err)
		}
	}
}
