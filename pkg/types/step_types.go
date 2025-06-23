package types

// StepResult is the standardized output structure returned by every handler's Run method.
type StepResult struct {
	Output     any    `json:"output"`
	OutputFile string `json:"output_file,omitempty"`
}
