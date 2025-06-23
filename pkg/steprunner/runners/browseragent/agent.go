package browseragent

import (
	"github.com/arnavsurve/dropstep/pkg/types"
)

type AgentRunner interface {
	RunAgent(step types.Step, rawOutputPath string, schemaContent string, targetDownloadDir string, logger types.Logger, apiKey string) ([]byte, error)
}
