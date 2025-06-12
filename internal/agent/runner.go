package agent

import (
	"github.com/arnavsurve/dropstep/internal"
	"github.com/rs/zerolog"
)

type AgentRunner interface {
	// RunAgent(prompt, outputPath string, filesToUpload []internal.FileToUpload, schemaContent string, targetDownloadDir string, allowedDomains []string, logger *zerolog.Logger) ([]byte, error)

	RunAgent(step internal.Step, rawOutputPath string, schemaContent string, targetDownloadDir string, logger *zerolog.Logger) ([]byte, error)
}
