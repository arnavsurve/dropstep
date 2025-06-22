package core

import "path/filepath"

// ResolvePathFromWorkflow resolves a path from a workflow file.
// If the provided path is already absolute, it's returned as is.
// If it's relative, it's joined with the workflowDir to create an absolute path.
func ResolvePathFromWorkflow(workflowDir, pathFromYAML string) (string, error) {
	if filepath.IsAbs(pathFromYAML) {
		return pathFromYAML, nil
	}

	absPath := filepath.Join(workflowDir, pathFromYAML)
	return absPath, nil
}