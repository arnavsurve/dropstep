package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/arnavsurve/dropstep/internal"
	"github.com/arnavsurve/dropstep/internal/agent"
)

type BrowserHandler struct {
	Agent   agent.AgentRunner
	StepCtx internal.ExecutionContext
}

func init() {
	RegisterHandlerFactory("browser_agent", func(ctx internal.ExecutionContext) (Handler, error) {
		agentRunner, err := agent.NewSubprocessAgentRunner(ctx.Logger)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize agent runner: %w", err)
		}
		handler := &BrowserHandler{
			Agent:   agentRunner,
			StepCtx: ctx,
		}
		return handler, nil
	})
}

func (bh *BrowserHandler) Validate() error {
	step := bh.StepCtx.Step
	logger := bh.StepCtx.Logger

	if step.Prompt == "" {
		return fmt.Errorf("browser_agent step %q must define 'prompt'", step.ID)
	}

	if step.Run != "" {
		return fmt.Errorf("browser_agent step %q must not define 'run'", step.ID)
	}
	if step.Call != nil {
		return fmt.Errorf("browser_agent step %q must not define 'call'", step.ID)
	}

	for i, f := range step.UploadFiles {
		if f.Name == "" {
			return fmt.Errorf("upload_files[%d] in step %q is missing 'name'", i, step.ID)
		}
		if f.Path == "" {
			return fmt.Errorf("upload_files[%d] in step %q is missing 'path'", i, step.ID)
		}
		if _, err := os.Stat(f.Path); err != nil {
			return fmt.Errorf("upload_files[%d] in step %q: file not found at path %q", i, step.ID, f.Path)
		}
	}

	if step.OutputSchemaFile != "" {
		if _, err := os.Stat(step.OutputSchemaFile); err != nil {
			return fmt.Errorf("step %q: output_schema file not found at path %q", step.ID, step.OutputSchemaFile)
		}
	}

	if step.TargetDownloadDir != "" {
		if _, err := os.Stat(step.TargetDownloadDir); err != nil {
			if os.IsNotExist(err) {
				logger.Warn().Str("path", step.TargetDownloadDir).Msg("Download directory does not exist yet, will attempt to create at runtime")
			} else {
				return fmt.Errorf("step %q: error checking download_dir path %q: %w", step.ID, step.TargetDownloadDir, err)
			}
		}
	}

	return nil
}

func (bh *BrowserHandler) Run() error {
	step := bh.StepCtx.Step
	logger := bh.StepCtx.Logger

	outputDir := "output"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		logger.Error().
			Err(err).
			Str("dir", outputDir).
			Msg("Failed to create output directory")
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	var finalTargetDownloadDir string
	if step.TargetDownloadDir != "" {
		absPath, err := filepath.Abs(step.TargetDownloadDir)
		if err != nil {
			return fmt.Errorf("step %q: failed to get absolute path for target_download_dir %q: %w", step.ID, step.TargetDownloadDir, err)
		}
		finalTargetDownloadDir = absPath
		if err := os.MkdirAll(finalTargetDownloadDir, 0755); err != nil {
			return fmt.Errorf("step %q: failed to create target download directory %q: %w", step.ID, finalTargetDownloadDir, err)
		}
		logger.Info().Str("path", finalTargetDownloadDir).Msg("Ensured target download directory exists")
	} else {
		// No download_dir specified - default to a subdir within "output/" (step_id_default_downloads/)
		defaultDownloadsDir := filepath.Join(outputDir, fmt.Sprintf("%s_default_downloads", step.ID))
		if err := os.MkdirAll(defaultDownloadsDir, 0755); err != nil {
			return fmt.Errorf("step %q: failed to create default download directory %q: %w", step.ID, defaultDownloadsDir, err)
		}
		finalTargetDownloadDir = defaultDownloadsDir
		logger.Debug().Str("path", finalTargetDownloadDir).Msg("No target download directory specified, using default")
	}

	var outputSchemaJSONString string

	if step.OutputSchemaFile != "" {
		schemaFilePath, err := filepath.Abs(step.OutputSchemaFile)
		if err != nil {
			return fmt.Errorf("step %q: failed to determine absolute path for output schema file %q: %w", step.ID, step.OutputSchemaFile, err)
		}

		logger.Debug().Str("path", schemaFilePath).Msg("Loading output schema")
		schemaBytes, err := os.ReadFile(schemaFilePath)
		if err != nil {
			return fmt.Errorf("step %q: failed to read output schema file %q: %w", step.ID, schemaFilePath, err)
		}

		if !json.Valid(schemaBytes) {
			return fmt.Errorf("step %q: content of output schema file %q is not valid JSON", step.ID, schemaFilePath)
		}
		outputSchemaJSONString = string(schemaBytes)
	}

	agentOutputPath := fmt.Sprintf("output/%s_output.json", step.ID)
	jsonData, runErr := bh.Agent.RunAgent(step.Prompt, agentOutputPath, step.UploadFiles, outputSchemaJSONString, finalTargetDownloadDir, logger)

	if runErr != nil {
		logger.Error().Err(runErr).Msg("Agent execution failed")
	} else {
		logger.Info().Msg("Step completed")
		if jsonData != nil {
			var outputData map[string]any
			if parseErr := json.Unmarshal(jsonData, &outputData); parseErr != nil {
				logger.Error().Err(parseErr).Msg("Error parsing JSON output")
			} else {
				prettyOutput, err := json.MarshalIndent(outputData, "", "  ")
				if err != nil {
					logger.Error().Err(err).Msg("Error pretty-printing agent output")
					logger.Info().Msgf("Parsed agent output (raw): %+v\n", outputData)
				} else {
					logger.Info().
					RawJSON("output", prettyOutput).
					Msg("Received agent output")
				}
			}
		} else {
			logger.Info().Msg("No JSON output received")
		}
	}

	return nil
}
