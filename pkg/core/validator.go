package core

import (
	"fmt"

	"github.com/arnavsurve/dropstep/pkg/steprunner"
	"github.com/arnavsurve/dropstep/pkg/types"
)

// Validate checks fields at the workflow level, validating workflow name, input types/uniqueness, and step uniqueness.
func ValidateWorkflowStructure(wf *Workflow) error {
	if wf.Name == "" {
		return fmt.Errorf("workflow is missing 'name'")
	}

	validInputTypes := map[string]bool{
		"string":  true,
		"file":    true,
		"number":  true,
		"boolean": true,
	}

	inputNames := make(map[string]bool)
	for i, input := range wf.Inputs {
		if input.Name == "" {
			return fmt.Errorf("input %d is missing 'name'", i)
		}
		if inputNames[input.Name] {
			return fmt.Errorf("duplicate input name: %q", input.Name)
		}
		inputNames[input.Name] = true

		if !validInputTypes[input.Type] {
			return fmt.Errorf("input %q has invalid type %q", input.Name, input.Type)
		}
	}

	providerNames := make(map[string]bool)
	for i, provider := range wf.Providers {
		if provider.Name == "" {
			return fmt.Errorf("provider %d is missing 'name'", i)
		}
		if providerNames[provider.Name] {
			return fmt.Errorf("duplicate provider name: %q", provider.Name)
		}
		providerNames[provider.Name] = true

		if provider.Type == "" {
			return fmt.Errorf("provider %q is missing 'type'", provider.Name)
		}
	}

	stepIDs := make(map[string]bool)
	for i, step := range wf.Steps {
		if step.ID == "" {
			return fmt.Errorf("step %d is missing 'id'", i)
		}
		if stepIDs[step.ID] {
			return fmt.Errorf("duplicate step id: %q", step.ID)
		}
		stepIDs[step.ID] = true

		if step.Uses == "" {
			return fmt.Errorf("step %q is missing 'uses'", step.ID)
		}
	}

	return nil
}

func ValidateRequiredInputs(wf *Workflow, varCtx VarContext) error {
	for _, input := range wf.Inputs {
		if input.Required {
			if _, exists := varCtx[input.Name]; !exists && input.Default == "" {
				return fmt.Errorf("required input %q is missing from the varfile and no default value is provided", input.Name)
			}
		}
	}
	return nil
}

func ValidateWorkflowRunners(wf *Workflow, workflowDir string) error {
	for _, step := range wf.Steps {
		ctx := types.ExecutionContext{
			Step:        step,
			WorkflowDir: workflowDir,
		}

		runner, err := steprunner.GetRunner(ctx)
		if err != nil {
			return fmt.Errorf("getting runner for step %q: %w", step.ID, err)
		}

		if err = runner.Validate(); err != nil {
			return fmt.Errorf("validating step %q: %w", step.ID, err)
		}
	}

	return nil
}
