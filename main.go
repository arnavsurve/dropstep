package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"os"

	"github.com/arnavsurve/dropstep/internal"
	"github.com/arnavsurve/dropstep/internal/agent"
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

	if err := os.MkdirAll("output", 0755); err != nil {
		log.Fatalf("failed to create output directory: %v", err)
	}

	for _, step := range wf.Steps {
		fmt.Printf("==> Running step %q (uses=%s)\n", step.ID, step.Uses)

		switch step.Uses {
		case "browser":
			outputPath := fmt.Sprintf("output/%s_output.json", step.ID)
			jsonData, runErr := agent.RunAgent(step.Prompt, outputPath)
			if runErr != nil {
				log.Printf("step %s failed: %v\n", step.ID, runErr)
			} else {
				log.Printf("completed step %s", step.ID)
				if jsonData != nil {
					var outputData map[string]any
					if parseErr := json.Unmarshal(jsonData, &outputData); parseErr != nil {
						log.Printf("Error parsing JSON output for step %s: %v", step.ID, parseErr)
					} else {
						fmt.Printf("Parsed agent output for step %s: %+v\n", step.ID, outputData)
					}
				} else {
					fmt.Printf("No JSON output received for step %s.\n", step.ID)
				}
			}

		case "shell":
			// TODO

		case "api":
			// TODO

		default:
			log.Printf("unknown uses %q. Valid uses are 'browser', 'shell', or 'api'.", step.Uses)
		}
	}
}
