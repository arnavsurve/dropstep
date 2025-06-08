package handlers

import (
	"encoding/json"
	"fmt"
	"log"
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
	RegisterHandlerFactory("browser_agent", func(ctx internal.ExecutionContext) Handler {
		agentRunner, err := agent.NewSubprocessAgentRunner(ctx.Logger)
		if err != nil {
			log.Fatalf("Failed to initialize agent runner: %v", err)
		}
		return &BrowserHandler{
			Agent:   agentRunner,
			StepCtx: ctx,
		}
	})
}

func (bh *BrowserHandler) Validate() error {
	step := bh.StepCtx.Step
	logger := bh.StepCtx.Logger

	if step.TargetDownloadDir != "" {
		logger.Debug().Str("path", step.TargetDownloadDir).Msg("Resolved target path for downloads")
	}
	if step.OutputSchemaFile != "" {
		// Check if the path is non-empty after potential variable resolution (already done by InjectVars)
		// Further validation (e.g., file existence) could happen here or just before reading in Run().
		// For now, assume InjectVarsIntoWorkflow handles empty resolved paths if needed.
		logger.Debug().Str("path", step.OutputSchemaFile).Msg("Resolved output schema")
	}
	logger.Info().Msgf("(Placeholder) - validating %s", step.ID)
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
