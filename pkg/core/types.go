package core

import "github.com/arnavsurve/dropstep/pkg/types"

type StepResultsContext = map[string]types.StepResult

type ProviderConfig struct {
	Name   string `yaml:"name"`
	Type   string `yaml:"type"`
	APIKey string `yaml:"api_key"`
}

type Input struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Required bool   `yaml:"required,omitempty"`
	Secret   bool   `yaml:"secret,omitempty"`
	Default  string `yaml:"default,omitempty"`
}

type Workflow struct {
	Name        string           `yaml:"name"`
	Description string           `yaml:"description"`
	Inputs      []Input          `yaml:"inputs"`
	Providers   []ProviderConfig `yaml:"providers,omitempty"`
	Steps       []Step           `yaml:"steps"`
}

type Step = types.Step

type HTTPCall = types.HTTPCall

type FileToUpload = types.FileToUpload

type CommandBlock = types.CommandBlock

type ExecutionContext = types.ExecutionContext

type Level = types.Level

// Level constants
const (
	DebugLevel = types.DebugLevel
	InfoLevel  = types.InfoLevel
	WarnLevel  = types.WarnLevel
	ErrorLevel = types.ErrorLevel
	FatalLevel = types.FatalLevel
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
