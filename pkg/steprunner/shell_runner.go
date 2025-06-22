package steprunner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/arnavsurve/dropstep/pkg/fileutil"
	"github.com/arnavsurve/dropstep/pkg/types"
)

type ShellRunner struct {
	StepCtx types.ExecutionContext
}

func init() {
	RegisterRunnerFactory("shell", func(ctx types.ExecutionContext) (StepRunner, error) {
		return &ShellRunner{
			StepCtx: ctx,
		}, nil
	})
}

func (sr *ShellRunner) Validate() error {
	step := sr.StepCtx.Step

	if step.Prompt != "" {
		return fmt.Errorf("shell step %q must not define 'prompt'", step.ID)
	}
	if step.UploadFiles != nil {
		return fmt.Errorf("shell step %q must not define 'upload_files'", step.ID)
	}
	if step.TargetDownloadDir != "" {
		return fmt.Errorf("shell step %q must not define 'download_dir'", step.ID)
	}
	if step.OutputSchemaFile != "" {
		return fmt.Errorf("shell step %q must not define 'output_schema'", step.ID)
	}
	if step.Call != nil {
		return fmt.Errorf("shell step %q must not define 'call'", step.ID)
	}
	if step.AllowedDomains != nil {
		return fmt.Errorf("shell step %q must not define 'allowed_domains'", step.ID)
	}
	if step.MaxSteps != nil {
		return fmt.Errorf("shell step %q must not define 'max_steps'", step.ID)
	}
	if step.MaxFailures != nil {
		return fmt.Errorf("shell step %q must not define 'max_failures'", step.ID)
	}

	if step.Command == nil {
		return fmt.Errorf("shell step %q must define 'run'", step.ID)
	} else {
		if step.Command.Inline != "" && step.Command.Path != "" {
			return fmt.Errorf("shell step %q must only define either 'inline' or 'path'", step.ID)
		}
		if step.Command.Inline == "" && step.Command.Path == "" {
			return fmt.Errorf("shell step %q must define either 'inline' or 'path'", step.ID)
		}
	}

	return nil
}

func (sr *ShellRunner) Run() (*types.StepResult, error) {
	step := sr.StepCtx.Step
	logger := sr.StepCtx.Logger
	workflowDir := sr.StepCtx.WorkflowDir

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

	interpreter := "/bin/bash"
	if step.Command.Interpreter != "" {
		interpreter = step.Command.Interpreter
	}

	var cmd *exec.Cmd
	if isInline {
		cmd = sr.getInlineCommand(interpreter)
	} else {
		cmd = sr.getFileCommand(interpreter)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	logger.Info().Str("shell", interpreter).Msg("Starting shell script execution")

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("error executing script: %w", err)
	}

	waitErr := cmd.Wait()

	logBuffer(strings.NewReader(stderrBuf.String()), "STDERR", logger, "shell_line")
	logBuffer(strings.NewReader(stdoutBuf.String()), "STDOUT", logger, "shell_line")

	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			logger.Error().Int("exit_code", exitErr.ExitCode()).Msg("Script exited with non-zero code")
		}
		return nil, fmt.Errorf("shell script failed: %w", waitErr)
	}

	logger.Info().Msg("Shell script executed successfully")

	stdout := strings.TrimSpace(stdoutBuf.String())
	var structuredOutput map[string]any

	if err := json.Unmarshal([]byte(stdout), &structuredOutput); err == nil {
		logger.Debug().Msg("Shell output was valid JSON, promoting to structured output.")
		return &types.StepResult{Output: structuredOutput}, nil
	}

	logger.Debug().Msg("Shell output was not JSON, treating as raw string output.")
	return &types.StepResult{Output: stdout}, nil
}

func (sr *ShellRunner) getInlineCommand(interpreter string) *exec.Cmd {
	logger := sr.StepCtx.Logger
	inlineScript := sr.StepCtx.Step.Command.Inline
	if len(inlineScript) > 1000 {
		logger.Warn().Msgf("Long script in 'inline' - consider passing a script file as 'path' for maintainability.")
	}
	// #nosec G204
	safeScript := "set -euo pipefail\n" + inlineScript
	shellCmd := exec.Command(interpreter, "-c", safeScript)
	return shellCmd
}

func (sr *ShellRunner) getFileCommand(interpreter string) *exec.Cmd {
	scriptPath := sr.StepCtx.Step.Command.Path
	// #nosec G204
	shellCmd := exec.Command(interpreter, scriptPath)
	return shellCmd
}
