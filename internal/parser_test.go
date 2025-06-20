package internal_test

import (
	"path/filepath"
	"testing"

	"github.com/arnavsurve/dropstep/internal"
	"github.com/arnavsurve/dropstep/internal/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadWorkflowFixture(t *testing.T) {
	file := "testdata/simple_workflow.yml"
	ctx := internal.VarContext{"dir": "/tmp"}

	wf, err := internal.LoadWorkflowFromFile(file)
	require.NoError(t, err)

	injected, err := internal.InjectVarsIntoWorkflow(wf, ctx)
	require.NoError(t, err)

	require.Len(t, injected.Steps, 1)
	step := injected.Steps[0]
	assert.Equal(t, "Browse /tmp", step.Prompt)
	assert.Equal(t, "/tmp/doc.txt", step.UploadFiles[0].Path)
	assert.Equal(t, "/tmp/dl", step.TargetDownloadDir)
}

func TestLoadBrokenWorkflowFixture(t *testing.T) {
	file := "testdata/broken_workflow.yml"

	wf, err := internal.LoadWorkflowFromFile(file)
	require.NoError(t, err)

	workflowAbsPath, err := filepath.Abs(file)
	if err != nil {
		t.Errorf("could not determine absolute path for workflow file: %v", err)
	}
	workflowDir := filepath.Dir(workflowAbsPath)

	err = validation.ValidateWorkflowHandlers(wf, workflowDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must define 'prompt'")
}
