package agent

import "github.com/arnavsurve/dropstep/internal"

type AgentRunner interface {
	RunAgent(prompt, outputPath string, filesToUpload []internal.FileToUpload, schemaContent string) ([]byte, error)
}
