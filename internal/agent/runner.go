package agent

type AgentRunner interface {
	RunAgent(prompt string, outputPath string) ([]byte, error)
}
