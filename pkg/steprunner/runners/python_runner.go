package runners

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/arnavsurve/dropstep/pkg/fileutil"
	"github.com/arnavsurve/dropstep/pkg/steprunner"
	"github.com/arnavsurve/dropstep/pkg/types"
)

type PythonRunner struct {
	StepCtx types.ExecutionContext
}

func init() {
	steprunner.RegisterRunnerFactory("python", func(ctx types.ExecutionContext) (steprunner.StepRunner, error) {
		return &PythonRunner{
			StepCtx: ctx,
		}, nil
	})
}

func (pr *PythonRunner) Validate() error {
	step := pr.StepCtx.Step

	if step.Prompt != "" {
		return fmt.Errorf("python step %q must not define 'prompt'", step.ID)
	}
	if step.UploadFiles != nil {
		return fmt.Errorf("python step %q must not define 'upload_files'", step.ID)
	}
	if step.TargetDownloadDir != "" {
		return fmt.Errorf("python step %q must not define 'download_dir'", step.ID)
	}
	if step.OutputSchemaFile != "" {
		return fmt.Errorf("python step %q must not define 'output_schema'", step.ID)
	}
	if step.Call != nil {
		return fmt.Errorf("python step %q must not define 'call'", step.ID)
	}
	if step.AllowedDomains != nil {
		return fmt.Errorf("python step %q must not define 'allowed_domains'", step.ID)
	}
	if step.MaxSteps != nil {
		return fmt.Errorf("python step %q must not define 'max_steps'", step.ID)
	}
	if step.MaxFailures != nil {
		return fmt.Errorf("python step %q must not define 'max_failures'", step.ID)
	}

	if step.Command == nil {
		return fmt.Errorf("python step %q must define 'run'", step.ID)
	} else {
		if step.Command.Inline != "" && step.Command.Path != "" {
			return fmt.Errorf("python step %q must only define either 'inline' or 'path'", step.ID)
		}
		if step.Command.Inline == "" && step.Command.Path == "" {
			return fmt.Errorf("python step %q must define either 'inline' or 'path'", step.ID)
		}
	}

	interpreter := "python3"
	if step.Command.Interpreter != "" {
		interpreter = step.Command.Interpreter
	}

	// #nosec G204
	cmd := exec.Command(interpreter, "--version")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("interpreter %q is not a valid command: %w. Make sure it's in your PATH", interpreter, err)
	}

	if !strings.Contains(strings.ToLower(out.String()), "python") {
		return fmt.Errorf("command %q does not appear to be a python interpreter. Output: %s", interpreter, out.String())
	}

	return nil
}

func (pr *PythonRunner) Run() (*types.StepResult, error) {
	step := pr.StepCtx.Step
	logger := pr.StepCtx.Logger
	workflowDir := pr.StepCtx.WorkflowDir

	isInline := step.Command.Inline != ""
	if !isInline {
		resolvedPath, err := fileutil.ResolvePathFromWorkflow(workflowDir, step.Command.Path)
		if err != nil {
			return nil, fmt.Errorf("error resolving script path: %w", err)
		}
		if _, err := os.Stat(resolvedPath); err != nil {
			return nil, fmt.Errorf("script file not found at %q: %w", resolvedPath, err)
		}
		step.Command.Path = resolvedPath
	}

	interpreter := "python3"
	if step.Command.Interpreter != "" {
		interpreter = step.Command.Interpreter
	}

	var cmd *exec.Cmd
	if isInline {
		cmd = pr.getInlineCommand(interpreter)
	} else {
		cmd = pr.getFileCommand(interpreter)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	logger.Info().Str("python", interpreter).Msg("Starting python script execution")

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("error executing script: %w", err)
	}

	waitErr := cmd.Wait()

	steprunner.LogBuffer(strings.NewReader(stderrBuf.String()), "STDERR", logger, "python_line")
	steprunner.LogBuffer(strings.NewReader(stdoutBuf.String()), "STDOUT", logger, "python_line")

	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			logger.Error().Int("exit_code", exitErr.ExitCode()).Msg("Script exited with non-zero code")
		}
		return nil, fmt.Errorf("python script failed: %w", waitErr)
	}

	logger.Info().Msg("Python script executed successfully")

	stdout := strings.TrimSpace(stdoutBuf.String())
	var structuredOutput map[string]any

	if err := json.Unmarshal([]byte(stdout), &structuredOutput); err == nil {
		logger.Debug().Msg("Python output was valid JSON, promoting to structured output.")
		return &types.StepResult{Output: structuredOutput}, nil
	}

	logger.Debug().Msg("Python output was not JSON, treating as raw string output.")
	return &types.StepResult{Output: stdout}, nil
}

func (ph *PythonRunner) getInlineCommand(interpreter string) *exec.Cmd {
	inlineScript := ph.StepCtx.Step.Command.Inline
	if len(inlineScript) > 1000 {
		ph.StepCtx.Logger.Warn().Msg("Long script in 'inline' - consider passing a script file as 'path' for maintainability.")
	}
	// #nosec G204
	shellCmd := exec.Command(interpreter, "-c", inlineScript)
	return shellCmd
}

func (pr *PythonRunner) getFileCommand(interpreter string) *exec.Cmd {
	scriptPath := pr.StepCtx.Step.Command.Path
	// #nosec G204
	shellCmd := exec.Command(interpreter, scriptPath)
	return shellCmd
}
