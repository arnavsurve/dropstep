package types

// ExecutionContext contains the context needed for step execution
type ExecutionContext struct {
	Step        Step
	Logger      Logger
	WorkflowDir string
	APIKey      string
}

// Step represents a workflow step
type Step struct {
	ID                string
	Uses              string
	Provider          string
	Prompt            string
	Command           *CommandBlock
	Call              *HTTPCall
	UploadFiles       []FileToUpload
	TargetDownloadDir string
	OutputSchemaFile  string
	AllowedDomains    []string
	MaxSteps          *int
	MaxFailures       *int
	Timeout           string
}

// CommandBlock represents a shell command to run
type CommandBlock struct {
	Path        string
	Inline      string
	Interpreter string
}

// HTTPCall represents an HTTP request
type HTTPCall struct {
	Method  string
	Url     string
	Headers map[string]string
	Body    map[string]any
}

// FileToUpload represents a file to be uploaded
type FileToUpload struct {
	Name string
	Path string
}
