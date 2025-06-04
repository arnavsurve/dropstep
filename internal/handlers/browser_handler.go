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
	agentRunnerInstance, err := agent.NewSubprocessAgentRunner()
	if err != nil {
		log.Fatalf("Failed to initialize agent runner: %v", err)
	}

	RegisterHandlerFactory("browser", func(ctx internal.ExecutionContext) Handler {
		return &BrowserHandler{
			Agent: agentRunnerInstance,
			StepCtx: ctx,
		}
	})
}

func (bh *BrowserHandler) Validate() error {
	step := bh.StepCtx.Step
	if step.TargetDownloadDir != "" {
		log.Printf("Step %q: Downloads, if any, will be targeted to: %s", step.ID, step.TargetDownloadDir)
	}
	if step.OutputSchemaFile != "" {
		// Check if the path is non-empty after potential variable resolution (already done by InjectVars)
		// Further validation (e.g., file existence) could happen here or just before reading in Run().
		// For now, assume InjectVarsIntoWorkflow handles empty resolved paths if needed.
		log.Printf("Step %q: Output schema will be loaded from: %s", step.ID, step.OutputSchemaFile)
	}
	fmt.Printf("(Placeholder) - validating %s\n", step.ID)
	return nil
}

func (bh *BrowserHandler) Run() error {
	step := bh.StepCtx.Step
	outputDir := "output"

	if err := os.MkdirAll("output", 0755); err != nil {
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
        log.Printf("Step %q: Ensured target download directory exists: %s", step.ID, finalTargetDownloadDir)
	} else {
		// No download_dir specified - default to a subdir within "output/" (step_id_default_downloads/)
		defaultDownloadsDir := filepath.Join(outputDir, fmt.Sprintf("%s_default_downloads", step.ID))
        if err := os.MkdirAll(defaultDownloadsDir, 0755); err != nil {
            return fmt.Errorf("step %q: failed to create default download directory %q: %w", step.ID, defaultDownloadsDir, err)
        }
        finalTargetDownloadDir = defaultDownloadsDir
        log.Printf("Step %q: No target_download_dir specified, using default: %s", step.ID, finalTargetDownloadDir)
	}

	var outputSchemaJSONString string

	if step.OutputSchemaFile != "" {
		schemaFilePath, err := filepath.Abs(step.OutputSchemaFile)
		if err != nil {
			return fmt.Errorf("step %q: failed to determine absolute path for output schema file %q: %w", step.ID, step.OutputSchemaFile, err)
		}

		log.Printf("Step %q: Loading output schema from %s", step.ID, schemaFilePath)
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
	jsonData, runErr := bh.Agent.RunAgent(step.Prompt, agentOutputPath, step.UploadFiles, outputSchemaJSONString, step.TargetDownloadDir)
	if runErr != nil {
		log.Printf("Step '%s' agent execution failed: %v\n", step.ID, runErr)
	} else {
		log.Printf("Completed step '%s'", step.ID)
		if jsonData != nil {
			var outputData map[string]any
			if parseErr := json.Unmarshal(jsonData, &outputData); parseErr != nil {
				log.Printf("Error parsing JSON output for step %s: %v", step.ID, parseErr)
			} else {
				fmt.Printf("Parsed agent output for step %s: %+v\n", step.ID, outputData)
			}
		} else {
			fmt.Printf("No JSON output received for step %s.\n", step.ID)
		}
	}

	return nil
}
