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
	Agent   agent.AgentRunner
	StepCtx internal.ExecutionContext
}

func init() {
	RegisterHandlerFactory("browser", func(ctx internal.ExecutionContext) Handler {
		return &BrowserHandler{
			Agent: &agent.SubprocessAgentRunner{
				ScriptPath: "internal/agent/run.sh",
			},
			StepCtx: ctx,
		}
	})
}

func (bh *BrowserHandler) Validate() error {
	step := bh.StepCtx.Step
	fmt.Printf("(Placeholder) - validating %s\n", step.ID)
	return nil
}

func (bh *BrowserHandler) Run() error {
	if err := os.MkdirAll("output", 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	step := bh.StepCtx.Step

	outputPath := fmt.Sprintf("output/%s_output.json", step.ID)
	jsonData, runErr := bh.Agent.RunAgent(step.Prompt, outputPath, step.UploadFiles)
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
