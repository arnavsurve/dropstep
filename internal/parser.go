package internal

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
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

	// Validate steps
	for i, step := range wf.Steps {
		if step.ID == "" {
			return nil, fmt.Errorf("step at index %d is missing an 'id'", i)
		}
		if err := step.Validate(); err != nil {
			return nil, fmt.Errorf("invalid step at index %d: %w", i, err)
		}
	}

	return &wf, nil
}
