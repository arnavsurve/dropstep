package internal

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

func LoadWorkflowFromFile(path string) (*Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading workflow file: %w", err)
	}

	var wf Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("error parsing workflow YAML: %w", err)
	}

	// basic validation
	for i, step := range wf.Steps {
		if step.ID == "" {
			return nil, fmt.Errorf("step at index %d is missing an 'id'", i)
		}
		if step.Uses == "" {
			return nil, fmt.Errorf("step '%s' is missing 'uses'", step.ID)
		}
		if step.Uses == "browser" && step.Prompt == "" {
			return nil, fmt.Errorf("step '%s' uses 'browser' but has no 'prompt'", step.ID)
		}
		if step.Uses == "shell" && step.Run == "" {
			return nil, fmt.Errorf("step '%s' uses 'shell' but has no 'run'", step.ID)
		}
		if step.Uses == "api" && step.Call == nil {
			return nil, fmt.Errorf("step '%s' uses 'api' but has no 'call' block", step.ID)
		}
	}

	return &wf, nil
}
