package agent

import (
	"github.com/arnavsurve/dropstep/internal"
	"github.com/rs/zerolog"
)

type AgentRunner interface {
	RunAgent(step internal.Step, rawOutputPath string, schemaContent string, targetDownloadDir string, logger *zerolog.Logger, apiKey string) ([]byte, error)
}
