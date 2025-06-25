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
			ctx.Logger = nullLogger
		}

		agentRunner, err := browseragent.NewSubprocessAgentRunner(logger)
		if err != nil {
			return nil, fmt.Errorf("initializing subprocess agent runner: %w", err)
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

	if step.BrowserConfig.Prompt == "" {
		return fmt.Errorf("browser_agent step %q must define 'browser.prompt'", step.ID)
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

	for i, f := range step.BrowserConfig.UploadFiles {
		if f.Name == "" {
			return fmt.Errorf("browser.upload_files[%d] in step %q is missing 'name'", i, step.ID)
		}
		if f.Path == "" {
			return fmt.Errorf("browser.upload_files[%d] in step %q is missing 'path'", i, step.ID)
		}

		resolvedPath, err := fileutil.ResolvePathFromWorkflow(workflowDir, f.Path)
		if err != nil {
			return fmt.Errorf("step %q: resolving browser.upload_files[%d] path %q: %w", step.ID, i, f.Path, err)
		}
		if _, err := os.Stat(resolvedPath); err != nil {
			return fmt.Errorf("step %q: browser.upload_files[%d] file not found at path %q: %w", step.ID, i, resolvedPath, err)
		}
	}

	if step.BrowserConfig.OutputSchemaFile != "" {
		resolvedPath, err := fileutil.ResolvePathFromWorkflow(workflowDir, step.BrowserConfig.OutputSchemaFile)
		if err != nil {
			return fmt.Errorf("step %q: resolving browser.output_schema path %q: %w", step.ID, step.BrowserConfig.OutputSchemaFile, err)
		}
		if _, err := os.Stat(resolvedPath); err != nil {
			return fmt.Errorf("step %q: browser.output_schema file not found at path %q", step.ID, resolvedPath)
		}
	}

	if step.BrowserConfig.TargetDownloadDir != "" {
		resolvedPath, err := fileutil.ResolvePathFromWorkflow(workflowDir, step.BrowserConfig.TargetDownloadDir)
		if err != nil {
			return fmt.Errorf("step %q: resolving browser.download_dir path %q: %w", step.ID, step.BrowserConfig.TargetDownloadDir, err)
		}
		if _, err := os.Stat(resolvedPath); err != nil {
			if os.IsNotExist(err) {
				logger.Warn().Str("path", resolvedPath).Msg("Download directory does not exist yet, will attempt to create at runtime")
			} else {
				return fmt.Errorf("step %q: checking browser.download_dir path %q: %w", step.ID, resolvedPath, err)
			}
		}
	}

	for i, domain := range step.BrowserConfig.AllowedDomains {
		if domain == "" {
			return fmt.Errorf("step %q: browser.allowed_domains[%d] must not be an empty string", step.ID, i)
		}
	}

	if step.BrowserConfig.MaxSteps == nil {
		// If MaxSteps is not defined, no need to validate
		// Default value is handled in the Python subprocess
	} else if *step.BrowserConfig.MaxSteps <= 0 {
		return fmt.Errorf("step %q: browser.max_steps must be greater than 0", step.ID)
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
	if step.BrowserConfig.TargetDownloadDir != "" {
		resolvedPath, err := fileutil.ResolvePathFromWorkflow(workflowDir, step.BrowserConfig.TargetDownloadDir)
		if err != nil {
			return nil, fmt.Errorf("step %q: resolving target_download_dir %q: %w", step.ID, step.BrowserConfig.TargetDownloadDir, err)
		}
		finalTargetDownloadDir = resolvedPath
		if err := os.MkdirAll(finalTargetDownloadDir, 0755); err != nil {
			return nil, fmt.Errorf("step %q: creating target download directory %q: %w", step.ID, finalTargetDownloadDir, err)
		}
		logger.Info().Str("path", finalTargetDownloadDir).Msg("Ensured target download directory exists")
	} else {
		// No download_dir specified - default to a subdir within "output/" (step_id_default_downloads/)
		defaultDownloadsDir := filepath.Join(outputDir, fmt.Sprintf("%s_default_downloads", step.ID))
		absPath, err := filepath.Abs(defaultDownloadsDir)
		if err != nil {
			return nil, fmt.Errorf("step %q: getting absolute path for default download directory %q: %w", step.ID, defaultDownloadsDir, err)
		}
		if err := os.MkdirAll(absPath, 0755); err != nil {
			return nil, fmt.Errorf("step %q: creating default download directory %q: %w", step.ID, defaultDownloadsDir, err)
		}
		finalTargetDownloadDir = absPath
		logger.Debug().Str("path", finalTargetDownloadDir).Msg("No target download directory specified, using default")
	}

	var outputSchemaJSONString string
	if step.BrowserConfig.OutputSchemaFile != "" {
		schemaFilePath, err := filepath.Abs(step.BrowserConfig.OutputSchemaFile)
		if err != nil {
			return nil, fmt.Errorf("step %q: determining absolute path for output schema file %q: %w", step.ID, step.BrowserConfig.OutputSchemaFile, err)
		}

		logger.Debug().Str("path", schemaFilePath).Msg("Loading output schema")
		schemaBytes, err := os.ReadFile(schemaFilePath)
		if err != nil {
			return nil, fmt.Errorf("step %q: reading output schema file %q: %w", step.ID, schemaFilePath, err)
		}

		if !json.Valid(schemaBytes) {
			return nil, fmt.Errorf("step %q: content of output schema file %q is not valid JSON", step.ID, schemaFilePath)
		}
		outputSchemaJSONString = string(schemaBytes)
	}

	agentStep := step
	for i, f := range agentStep.BrowserConfig.UploadFiles {
		absUploadPath, err := fileutil.ResolvePathFromWorkflow(workflowDir, f.Path)
		if err != nil {
			return nil, fmt.Errorf("step %q: resolving upload file path %q: %w", step.ID, f.Path, err)
		}
		agentStep.BrowserConfig.UploadFiles[i].Path = absUploadPath
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
