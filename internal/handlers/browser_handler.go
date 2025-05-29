package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/arnavsurve/dropstep/internal"
	"github.com/arnavsurve/dropstep/internal/agent"
)

type BrowserHandler struct {
	Agent agent.AgentRunner
}

func init() {
	RegisterHandlerFactory("browser", func() Handler {
		return &BrowserHandler{
			Agent: &agent.SubprocessAgentRunner{
				ScriptPath: "internal/agent/run.sh",
			},
		}
	})
}

func (bh *BrowserHandler) Validate(step internal.Step) error {
	fmt.Printf("(Placeholder) - validating %s\n", step.ID)
	return nil
}

func (bh *BrowserHandler) Run(step internal.Step) error {
	if err := os.MkdirAll("output", 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	outputPath := fmt.Sprintf("output/%s_output.json", step.ID)

	jsonData, runErr := bh.Agent.RunAgent(step.Prompt, outputPath)
	if runErr != nil {
		log.Printf("Step '%s' failed: %v\n", step.ID, runErr)
	} else {
		log.Printf("Completed step '%s'", step.ID)
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

	return nil
}
