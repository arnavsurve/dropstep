package handlers

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
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

func (sh *ShellHandler) Run() error {
	step := sh.StepCtx.Step
	logger := sh.StepCtx.Logger

	var isInline bool
	if step.Run.Inline != "" {
		isInline = true
	} else {
		isInline = false
		if _, err := os.Stat(step.Run.Path); err != nil {
			return fmt.Errorf("error resolving script path: %w", err)
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

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("error creating stderr pipe: %w", err)
	}

	logger.Info().Str("shell", interpreter).Msg("Starting shell script execution")
	
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error executing script: %w", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go streamOutputStructured(stdout, &wg, "STDOUT", logger)
	go streamOutputStructured(stderr, &wg, "STDERR", logger)

	waitErr := cmd.Wait()
	wg.Wait()
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			logger.Error().Int("exit_code", exitErr.ExitCode()).Msg("Script exited with non-zero code")
		}
		return fmt.Errorf("shell script failed: %w", waitErr)
	}

	logger.Info().Msg("Shell script executed successfully")
	return nil
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