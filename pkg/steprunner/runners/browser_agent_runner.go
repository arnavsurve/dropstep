package runners

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/arnavsurve/dropstep/pkg/fileutil"
	"github.com/arnavsurve/dropstep/pkg/log"
	"github.com/arnavsurve/dropstep/pkg/steprunner"
	"github.com/arnavsurve/dropstep/pkg/steprunner/runners/browseragent"
	"github.com/arnavsurve/dropstep/pkg/types"
	"github.com/rs/zerolog"
)

type BrowserAgentRunner struct {
	Agent   browseragent.AgentRunner
	StepCtx types.ExecutionContext
}

func init() {
	steprunner.RegisterRunnerFactory("browser_agent", func(ctx types.ExecutionContext) (steprunner.StepRunner, error) {
		// Create a null logger if ctx.Logger is nil to prevent crashes during validation
		logger := ctx.Logger
		if logger == nil {
			// Create a no-op logger that discards all output
			nullLogger := log.NewZerologAdapter(zerolog.New(zerolog.Nop()))
			logger = nullLogger
		}

		agentRunner, err := browseragent.NewSubprocessAgentRunner(logger)
		if err != nil {
			return nil, err
		}
		return &BrowserAgentRunner{
			Agent:   agentRunner,
			StepCtx: ctx,
		}, nil
	})
}

func (bar *BrowserAgentRunner) Validate() error {
	step := bar.StepCtx.Step
	logger := bar.StepCtx.Logger
	workflowDir := bar.StepCtx.WorkflowDir

	if step.Prompt == "" {
		return fmt.Errorf("browser_agent step %q must define 'prompt'", step.ID)
	}

	if step.Provider == "" {
		return fmt.Errorf("browser_agent step %q must specify a 'provider'", step.ID)
	}

	if step.Command != nil {
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

		// Validate file exists
		resolvedPath, err := fileutil.ResolvePathFromWorkflow(workflowDir, f.Path)
		if err != nil {
			return fmt.Errorf("step %q: failed to resolve path for upload_files[%d] %q: %w", step.ID, i, f.Path, err)
		}
		if _, err := os.Stat(resolvedPath); err != nil {
			return fmt.Errorf("step %q: upload_files[%d] file not found at path %q: %w", step.ID, i, resolvedPath, err)
		}
	}

	if step.OutputSchemaFile != "" {
		resolvedPath, err := fileutil.ResolvePathFromWorkflow(workflowDir, step.OutputSchemaFile)
		if err != nil {
			return fmt.Errorf("step %q: could not resolve output_schema path: %w", step.ID, err)
		}
		if _, err := os.Stat(resolvedPath); err != nil {
			return fmt.Errorf("step %q: output_schema file not found at path %q", step.ID, resolvedPath)
		}
	}

	if step.TargetDownloadDir != "" {
		resolvedPath, err := fileutil.ResolvePathFromWorkflow(workflowDir, step.TargetDownloadDir)
		if err != nil {
			return fmt.Errorf("step %q: could not resolve download_dir path: %w", step.ID, err)
		}
		if _, err := os.Stat(resolvedPath); err != nil {
			if os.IsNotExist(err) {
				logger.Warn().Str("path", resolvedPath).Msg("Download directory does not exist yet, will attempt to create at runtime")
			} else {
				return fmt.Errorf("step %q: error checking download_dir path %q: %w", step.ID, resolvedPath, err)
			}
		}
	}

	for i, domain := range step.AllowedDomains {
		if domain == "" {
			return fmt.Errorf("step %q: allowed_domains[%d] must not be an empty string", step.ID, i)
		}
	}

	if step.MaxSteps == nil {
		// If MaxSteps is not defined, no need to validate
		// Default value is handled in the Python subprocess
	} else if *step.MaxSteps <= 0 {
		return fmt.Errorf("step %q: max_steps must be greater than 0", step.ID)
	}

	if step.MaxFailures == nil {
		// If MaxFailures is not defined, no need to validate
		// Default value is handled in the Python subprocess
	} else if *step.MaxFailures < 0 {
		return fmt.Errorf("step %q: max_failures must not be less than 0", step.ID)
	}

	return nil
}

func (bar *BrowserAgentRunner) Run() (*types.StepResult, error) {
	step := bar.StepCtx.Step
	logger := bar.StepCtx.Logger
	workflowDir := bar.StepCtx.WorkflowDir

	outputDir := "output"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		logger.Error().
			Err(err).
			Str("dir", outputDir).
			Msg("Failed to create output directory")
		return nil, fmt.Errorf("failed to create output directory: %v", err)
	}

	var finalTargetDownloadDir string
	if step.TargetDownloadDir != "" {
		resolvedPath, err := fileutil.ResolvePathFromWorkflow(workflowDir, step.TargetDownloadDir)
		if err != nil {
			return nil, fmt.Errorf("step %q: failed to resolve target_download_dir %q: %w", step.ID, step.TargetDownloadDir, err)
		}
		finalTargetDownloadDir = resolvedPath
		if err := os.MkdirAll(finalTargetDownloadDir, 0755); err != nil {
			return nil, fmt.Errorf("step %q: failed to create target download directory %q: %w", step.ID, finalTargetDownloadDir, err)
		}
		logger.Info().Str("path", finalTargetDownloadDir).Msg("Ensured target download directory exists")
	} else {
		// No download_dir specified - default to a subdir within "output/" (step_id_default_downloads/)
		defaultDownloadsDir := filepath.Join(outputDir, fmt.Sprintf("%s_default_downloads", step.ID))
		absPath, err := filepath.Abs(defaultDownloadsDir)
		if err != nil {
			return nil, fmt.Errorf("step %q: failed to get absolute path for default download directory %q: %w", step.ID, defaultDownloadsDir, err)
		}
		if err := os.MkdirAll(absPath, 0755); err != nil {
			return nil, fmt.Errorf("step %q: failed to create default download directory %q: %w", step.ID, defaultDownloadsDir, err)
		}
		finalTargetDownloadDir = absPath
		logger.Debug().Str("path", finalTargetDownloadDir).Msg("No target download directory specified, using default")
	}

	var outputSchemaJSONString string
	if step.OutputSchemaFile != "" {
		schemaFilePath, err := filepath.Abs(step.OutputSchemaFile)
		if err != nil {
			return nil, fmt.Errorf("step %q: failed to determine absolute path for output schema file %q: %w", step.ID, step.OutputSchemaFile, err)
		}

		logger.Debug().Str("path", schemaFilePath).Msg("Loading output schema")
		schemaBytes, err := os.ReadFile(schemaFilePath)
		if err != nil {
			return nil, fmt.Errorf("step %q: failed to read output schema file %q: %w", step.ID, schemaFilePath, err)
		}

		if !json.Valid(schemaBytes) {
			return nil, fmt.Errorf("step %q: content of output schema file %q is not valid JSON", step.ID, schemaFilePath)
		}
		outputSchemaJSONString = string(schemaBytes)
	}

	agentStep := step
	for i, f := range agentStep.UploadFiles {
		absUploadPath, err := fileutil.ResolvePathFromWorkflow(workflowDir, f.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve upload file path %q: %w", f.Path, err)
		}
		agentStep.UploadFiles[i].Path = absUploadPath
	}

	agentOutputPath := fmt.Sprintf("output/%s_output.json", step.ID)
	jsonData, runErr := bar.Agent.RunAgent(
		agentStep,
		agentOutputPath,
		outputSchemaJSONString,
		finalTargetDownloadDir,
		logger,
		bar.StepCtx.APIKey,
	)

	if runErr != nil {
		logger.Error().Err(runErr).Msg("Agent execution failed")
		return nil, runErr
	}

	logger.Info().Msg("Step completed")

	var outputData map[string]any
	if err := json.Unmarshal(jsonData, &outputData); err != nil {
		logger.Error().Err(err).Msg("Error parsing JSON output from agent")
		return &types.StepResult{Output: string(jsonData), OutputFile: agentOutputPath}, nil
	}

	prettyOutput, _ := json.MarshalIndent(outputData, "", "  ")
	logger.Info().Str("output", string(prettyOutput)).Msg("Received agent output")

	result := &types.StepResult{
		Output:     outputData,
		OutputFile: agentOutputPath,
	}

	return result, nil
}
