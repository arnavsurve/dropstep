package internal

import "fmt"

type Workflow struct {
	Name        string  `yaml:"name"`
	Description string  `yaml:"description"`
	Inputs      []Input `yaml:"inputs"`
	Steps       []Step  `yaml:"steps"`
}

type Input struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Required bool   `yaml:"required"`
}

type Step struct {
	ID               string         `yaml:"id"`
	Uses             string         `yaml:"uses"`                   // 'browser' | 'shell' | 'api'
	Prompt           string         `yaml:"prompt,omitempty"`       // if (uses: browser) prompt template
	Run              string         `yaml:"run,omitempty"`          // (if uses: shell) command line
	Call             *ApiCall       `yaml:"call,omitempty"`         // (if uses: api)
	UploadFiles      []FileToUpload `yaml:"upload_files,omitempty"` // (if uses: browser) files to upload
	TargetDownloadDir string 		`yaml:"download_dir,omitempty"` // (if uses: browser) target directory to place downloaded files 
	OutputSchemaFile string         `yaml:"output_schema,omitempty"`
}

func (s *Step) Validate() error {
	switch s.Uses {
	case "browser":
		if s.Prompt == "" {
			return fmt.Errorf("step %q: browser step requires 'prompt'", s.ID)
		}
		if s.Run != "" {
			return fmt.Errorf("step %q: browser step must not define 'run'", s.ID)
		}
		if s.Call != nil {
			return fmt.Errorf("step %q: browser step must not define 'call'", s.ID)
		}
	case "shell":
		if s.Run == "" {
			return fmt.Errorf("step %q: shell step requires 'run'", s.ID)
		}
		if s.Prompt != "" || s.Call != nil {
			return fmt.Errorf("step %q: shell step must not define 'prompt' or 'call'", s.ID)
		}
	case "api":
		if s.Call == nil {
			return fmt.Errorf("step %q: api step requires 'call'", s.ID)
		}
		if s.Run != "" || s.Prompt != "" {
			return fmt.Errorf("step %q: api step must not define 'run' or 'prompt'", s.ID)
		}
	default:
		return fmt.Errorf("step %q: unrecognized uses: %q", s.ID, s.Uses)
	}
	return nil
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

type ExecutionContext struct {
	Step Step
	// Include logger, DB conn here
}
