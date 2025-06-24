package core

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func LoadWorkflowFromFile(path string) (*Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading workflow file %q: %w", path, err)
	}

	var wf Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("parsing workflow YAML: %w", err)
	}

	if err := ValidateWorkflowStructure(&wf); err != nil {
		return nil, fmt.Errorf("invalid workflow: %w", err)
	}

	return &wf, nil
}
