package handlers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/arnavsurve/dropstep/internal"
	"github.com/rs/zerolog"
)

type ShellHandler struct {
	StepCtx internal.ExecutionContext
}

func init() {
	RegisterHandlerFactory("shell", func(ctx internal.ExecutionContext) (Handler, error) {
		handler := &ShellHandler{
			StepCtx: ctx,
		}
		return handler, nil
	})
}

func (sh *ShellHandler) Validate() error {
	step := sh.StepCtx.Step

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

	if step.Run == nil {
		return fmt.Errorf("shell step %q must define 'run'", step.ID)
	} else {
		if step.Run.Inline != "" && step.Run.Path != "" {
			return fmt.Errorf("shell step %q must only define either 'inline' or 'path'", step.ID)
		}
		if step.Run.Inline == "" && step.Run.Path == "" {
			return fmt.Errorf("shell step %q must define either 'inline' or 'path'", step.ID)
		}
	}

	return nil
}

func (sh *ShellHandler) Run() (*internal.StepResult, error) {
	step := sh.StepCtx.Step
	logger := sh.StepCtx.Logger

	isInline := step.Run.Inline != ""
	if !isInline {
		if _, err := os.Stat(step.Run.Path); err != nil {
			return nil, fmt.Errorf("error resolving script path: %w", err)
		}
	}

	interpreter := "/bin/bash"
	if step.Run.Interpreter != "" {
		interpreter = step.Run.Interpreter
	}

	var cmd *exec.Cmd
	if isInline {
		cmd = sh.getInlineCommand(interpreter)
	} else {
		cmd = sh.getFileCommand(interpreter)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	logger.Info().Str("shell", interpreter).Msg("Starting shell script execution")
	
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("error executing script: %w", err)
	}

	waitErr := cmd.Wait()

	logBuffer(strings.NewReader(stderrBuf.String()), "STDERR", logger)
	logBuffer(strings.NewReader(stdoutBuf.String()), "STDOUT", logger)

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
		return &internal.StepResult{Output: structuredOutput}, nil
	}

	logger.Debug().Msg("Shell output was not JSON, treating as raw string output.")
	return &internal.StepResult{Output: stdout}, nil
}

func (sh *ShellHandler) getInlineCommand(interpreter string) *exec.Cmd {
	logger := sh.StepCtx.Logger
	inlineScript := sh.StepCtx.Step.Run.Inline
	if len(inlineScript) > 1000 {
		logger.Warn().Msg("Long script in 'inline' - consider passing a script file as 'path' for maintainability.")
	}
	safeScript := "set -euo pipefail\n" + inlineScript
	shellCmd := exec.Command(interpreter, "-c", safeScript)
	return shellCmd
}

func (sh *ShellHandler) getFileCommand(interpreter string) *exec.Cmd {
	scriptPath := sh.StepCtx.Step.Run.Path
	shellCmd := exec.Command(interpreter, scriptPath)
	return shellCmd
}

func logBuffer(r io.Reader, source string, logger *zerolog.Logger) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		logger.Info().
			Str("source", source).
			Str("shell_line", scanner.Text()).
			Msg("Shell output")
	}
}

// Deprecated, keeping here in case streaming is reintroduced
func streamOutputStructured(r io.Reader, wg *sync.WaitGroup, source string, logger *zerolog.Logger) {
	defer wg.Done()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		logger.Info().
			Str("source", source).
			Str("shell_line", scanner.Text()).
			Msg("Shell output")
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		if errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) {
			return
		}	
		logger.Error().Err(err).Str("source", source).Msg("Unexpected error streaming agent output")
	}
}