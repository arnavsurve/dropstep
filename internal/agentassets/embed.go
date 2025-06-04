package agentassets

import (
	"embed"
	"io/fs"
	"path/filepath"
)

//go:embed all:agent_scripts
var agentScriptsFS embed.FS

// Define constants for the paths within the embedded filesystem
const (
	agentDirInEmbed     = "agent_scripts" // Directory name used in go:embed
	RunScriptFile       = "run.sh"
	MainPyFile          = "main.py"
	CliPyFile           = "cli.py"
	ModelsPyFile        = "models.py"
	ActionsPyFile       = "actions.py"
	SettingsPyFile      = "settings.py"
	InitPyFile          = "__init__.py"
	RequirementsFile    = "requirements.txt"
)

// GetAgentScriptContent returns the content of a specific script file from the embedded FS.
func GetAgentScriptContent(filename string) ([]byte, error) {
	return agentScriptsFS.ReadFile(filepath.Join(agentDirInEmbed, filename))
}

// GetAgentScriptsFS returns the embedded filesystem for the agent scripts.
// This allows iterating or accessing multiple files if needed (e.g., for venv setup).
func GetAgentScriptsFS() fs.FS {
	subFS, err := fs.Sub(agentScriptsFS, agentDirInEmbed)
	if err != nil {
		panic("Failed to get sub-filesystem for agent scripts: " + err.Error())
	}
	return subFS
}
