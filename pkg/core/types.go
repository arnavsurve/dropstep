package core

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

type Step struct {
	ID                string         `yaml:"id"`
	Uses              string         `yaml:"uses"`                      // 'browser_agent' | 'shell' | 'http'
	Provider          string         `yaml:"provider,omitempty"`        // (if uses: browser_agent) The name of the provider to use for this step
	Prompt            string         `yaml:"prompt,omitempty"`          // if (uses: browser_agent) prompt template
	Command           *CommandBlock  `yaml:"run,omitempty"`             // (if uses: shell) command line
	Call              *HTTPCall      `yaml:"call,omitempty"`            // (if uses: http)
	UploadFiles       []FileToUpload `yaml:"upload_files,omitempty"`    // (if uses: browser_agent) files to upload
	TargetDownloadDir string         `yaml:"download_dir,omitempty"`    // (if uses: browser_agent) target directory to place downloaded files
	OutputSchemaFile  string         `yaml:"output_schema,omitempty"`   // (if uses: browser_agent) path to JSON schema to use for LLM structured output
	AllowedDomains    []string       `yaml:"allowed_domains,omitempty"` // (if uses: browser_agent) list of allowed domains
	MaxSteps          *int           `yaml:"max_steps,omitempty"`       // (if uses: browser_agent) max number of steps an agent can take
	MaxFailures       *int           `yaml:"max_failures,omitempty"`    // (if uses: browser_agent) max number of failures an agent can incur
	Timeout           string         `yaml:"timeout,omitempty"`         // (if uses: http) timeout for the request
}

type HTTPCall struct {
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
	Interpreter string `yaml:"interpreter,omitempty"`
}

type ExecutionContext struct {
	Step        Step
	Logger      Logger
	WorkflowDir string
	APIKey      string
}

type Level int8

const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
)

type Field struct {
	Key   string
	Value any
}

func String(key, val string) Field {
	return Field{key, val}
}
func Int(key string, val int) Field {
	return Field{key, val}
}
func Error(err error) Field {
	return Field{"error", err.Error()}
}
func Any(key string, val any) Field {
	return Field{key, val}
}
