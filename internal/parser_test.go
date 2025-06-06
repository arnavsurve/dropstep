package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadWorkflowFixture(t *testing.T) {
	file := "testdata/simple_workflow.yml"
	ctx := VarContext{"dir": "/tmp"}

	wf, err := LoadWorkflowFromFile(file)
	require.NoError(t, err)

	injected, err := InjectVarsIntoWorkflow(wf, ctx)
	require.NoError(t, err)

	require.Len(t, injected.Steps, 1)
	step := injected.Steps[0]
	assert.Equal(t, "Browse /tmp", step.Prompt)
	assert.Equal(t, "/tmp/doc.txt", step.UploadFiles[0].Path)
	assert.Equal(t, "/tmp/dl", step.TargetDownloadDir)
}

func TestLoadBrokenWorkflowFixture(t *testing.T) {
	file := "testdata/broken_workflow.yml"

	_, err := LoadWorkflowFromFile(file)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "browser step requires 'prompt'")
}
