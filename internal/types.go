package internal

import (
	"fmt"

	"github.com/rs/zerolog"
)

// StepResult is the standardized output structure returned by every handler's Run method.
type StepResult struct {
	Output     any    `json:"output"`
	OutputFile string `json:"output_file,omitempty"`
}

type StepResultsContext = map[string]StepResult

type ProviderConfig struct {
	Name   string `yaml:"name"`
	Type   string `yaml:"type"`
	APIKey string `yaml:"api_key"`
}

type Input struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Required bool   `yaml:"required"`
	Secret   bool   `yaml:"secret"`
}

type Workflow struct {
	Name        string           `yaml:"name"`
	Description string           `yaml:"description"`
	Inputs      []Input          `yaml:"inputs"`
	Providers   []ProviderConfig `yaml:"providers,omitempty"`
	Steps       []Step           `yaml:"steps"`
}

// Validate checks fields at the workflow level, validating workflow name, input types/uniqueness, and step uniqueness.
func (wf *Workflow) Validate() error {
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

type Step struct {
	ID                string         `yaml:"id"`
	Uses              string         `yaml:"uses"`                      // 'browser_agent' | 'shell' | 'api'
	Provider          string         `yaml:"provider,omitempty"`        // (if uses: browser_agent) The name of the provider to use for this step
	Prompt            string         `yaml:"prompt,omitempty"`          // if (uses: browser_agent) prompt template
	Command           *CommandBlock  `yaml:"run,omitempty"`             // (if uses: shell) command line
	Call              *ApiCall       `yaml:"call,omitempty"`            // (if uses: api)
	UploadFiles       []FileToUpload `yaml:"upload_files,omitempty"`    // (if uses: browser_agent) files to upload
	TargetDownloadDir string         `yaml:"download_dir,omitempty"`    // (if uses: browser_agent) target directory to place downloaded files
	OutputSchemaFile  string         `yaml:"output_schema,omitempty"`   // (if uses: browser_agent) path to JSON schema to use for LLM structured output
	AllowedDomains    []string       `yaml:"allowed_domains,omitempty"` // (if uses: browser_agent) list of allowed domains
	MaxSteps          *int           `yaml:"max_steps,omitempty"`       // (if uses: browser_agent) max number of steps an agent can take
	MaxFailures       *int           `yaml:"max_failures,omitempty"`    // (if uses: browser_agent) max number of failures an agent can incur
}

type ApiCall struct {
	Method  string            `yaml:"method"`
	Url     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers"`
	Body    map[string]any    `yaml:"body"`
}

type FileToUpload struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

type CommandBlock struct {
	Path        string `yaml:"path"`
	Inline      string `yaml:"inline"`
	Interpreter string `yaml:"interpreter"`
}

type ExecutionContext struct {
	Step        Step
	Logger      *zerolog.Logger
	WorkflowDir string
	APIKey      string
}
