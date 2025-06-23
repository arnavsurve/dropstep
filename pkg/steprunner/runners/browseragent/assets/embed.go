package assets

import (
	"embed"
	"io/fs"
	"path/filepath"
)

//go:embed all:agent_scripts
var agentScriptsFS embed.FS

const (
	agentDirInEmbed  = "agent_scripts"
	RunScriptFile    = "run.sh"
	MainPyFile       = "main.py"
	CliPyFile        = "cli.py"
	ModelsPyFile     = "models.py"
	ActionsPyFile    = "actions.py"
	SettingsPyFile   = "settings.py"
	InitPyFile       = "__init__.py"
	RequirementsFile = "requirements.txt"
)

func GetAgentScriptContent(filename string) ([]byte, error) {
	return agentScriptsFS.ReadFile(filepath.Join(agentDirInEmbed, filename))
}

func GetAgentScriptsFS() fs.FS {
	subFS, err := fs.Sub(agentScriptsFS, agentDirInEmbed)
	if err != nil {
		panic("Failed to get sub-filesystem for agent scripts: " + err.Error())
	}
	return subFS
}
