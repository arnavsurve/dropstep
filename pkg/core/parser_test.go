package core_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/arnavsurve/dropstep/pkg/core"
	"github.com/arnavsurve/dropstep/pkg/steprunner"
	"github.com/arnavsurve/dropstep/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBrowserAgentRunner is a simplified mock of the browser agent runner for testing
type TestBrowserAgentRunner struct {
	ctx types.ExecutionContext
}

func (r *TestBrowserAgentRunner) Validate() error {
	step := r.ctx.Step
	if step.BrowserConfig.Prompt == "" {
		return fmt.Errorf("browser_agent step %q must define 'browser.prompt'", step.ID)
	}
	return nil
}

func (r *TestBrowserAgentRunner) Run() (*types.StepResult, error) {
	return &types.StepResult{}, nil
}

func init() {
	// Register our test browser_agent runner
	steprunner.RegisterRunnerFactory("browser_agent", func(ctx types.ExecutionContext) (steprunner.StepRunner, error) {
		return &TestBrowserAgentRunner{ctx: ctx}, nil
	})
}

func TestLoadWorkflowFixture(t *testing.T) {
	file := "test_fixtures/simple_workflow.yml"
	ctx := core.VarContext{"dir": "/tmp"}

	wf, err := core.LoadWorkflowFromFile(file)
	require.NoError(t, err)

	injected, err := core.InjectVarsIntoWorkflow(wf, ctx)
	require.NoError(t, err)

	require.Len(t, injected.Steps, 1)
	step := injected.Steps[0]
	assert.Equal(t, "Browse /tmp", step.BrowserConfig.Prompt)
	assert.Equal(t, "/tmp/doc.txt", step.BrowserConfig.UploadFiles[0].Path)
	assert.Equal(t, "/tmp/dl", step.BrowserConfig.TargetDownloadDir)
}

func TestLoadBrokenWorkflowFixture(t *testing.T) {
	file := "test_fixtures/broken_workflow.yml"

	wf, err := core.LoadWorkflowFromFile(file)
	require.NoError(t, err)

	workflowAbsPath, err := filepath.Abs(file)
	if err != nil {
		t.Errorf("could not determine absolute path for workflow file: %v", err)
	}
	workflowDir := filepath.Dir(workflowAbsPath)

	err = core.ValidateWorkflowRunners(wf, workflowDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must define 'browser.prompt'")
}
