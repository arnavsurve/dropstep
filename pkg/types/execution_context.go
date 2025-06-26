package types

// ExecutionContext contains the context needed for step execution
type ExecutionContext struct {
	Step        Step
	Logger      Logger // Assuming Logger is defined in types.log_types.go
	WorkflowDir string
	APIKey      string
}

// Step represents a workflow step
type Step struct {
	ID            string        `yaml:"id"`
	Uses          string        `yaml:"uses"`
	Provider      string        `yaml:"provider,omitempty"`
	Command       *CommandBlock `yaml:"run,omitempty"`
	Call          *HTTPCall     `yaml:"call,omitempty"`
	BrowserConfig BrowserConfig `yaml:"browser,omitempty"`
	MaxFailures   *int          `yaml:"max_failures,omitempty"`
	Timeout       string        `yaml:"timeout,omitempty"`
}

// CommandBlock represents a shell or python script to run
type CommandBlock struct {
	Path        string `yaml:"path"`
	Inline      string `yaml:"inline"`
	Interpreter string `yaml:"interpreter,omitempty"`
}

// HTTPCall represents an HTTP request
type HTTPCall struct {
	Method  string            `yaml:"method"`
	Url     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers"`
	Body    map[string]any    `yaml:"body"`
}

// FileToUpload represents a file to be uploaded
type FileToUpload struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

// BrowserConfig represents a browser agent's configuration
type BrowserConfig struct {
	Prompt            string         `yaml:"prompt,omitempty"`
	UploadFiles       []FileToUpload `yaml:"upload_files,omitempty"`
	TargetDownloadDir string         `yaml:"download_dir,omitempty"`
	DataDir           string         `yaml:"data_dir,omitempty"`
	OutputSchemaFile  string         `yaml:"output_schema,omitempty"`
	AllowedDomains    []string       `yaml:"allowed_domains,omitempty"`
	MaxSteps          *int           `yaml:"max_steps,omitempty"`
}

