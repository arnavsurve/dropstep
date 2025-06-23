package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/arnavsurve/dropstep/pkg/core"
	"github.com/arnavsurve/dropstep/pkg/log"
	"github.com/arnavsurve/dropstep/pkg/log/sinks"
	"github.com/arnavsurve/dropstep/pkg/security"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"

	// Ensure all runner implementations are initialized
	_ "github.com/arnavsurve/dropstep/pkg/steprunner/runners"
)

type RunCmd struct {
	Varfile  string `help:"The YAML varfile for input variables." default:"dsvars.yml"`
	Workflow string `help:"The workflow configuration file." default:"dropstep.yml"`
}

func getFallbackKey(providerType string) string {
	switch providerType {
	case "openai":
		return os.Getenv("OPENAI_API_KEY")
	default:
		return ""
	}
}

func (r *RunCmd) Run() error {
	wfRunID := uuid.New().String()

	consoleSink := sinks.NewConsoleSink()

	logsDir := ".dropstep/logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("could not create logs directory %q: %w", logsDir, err)
	}
	logFilePath := filepath.Join(logsDir, fmt.Sprintf("%s.json", wfRunID))
	fileSink, err := sinks.NewFileSink(logFilePath)
	if err != nil {
		return fmt.Errorf("could not create file log sink: %w", err)
	}

	logRouter := log.NewRouter()
	logRouter.AddSink(consoleSink)
	logRouter.AddSink(fileSink)

	routerWriter := logRouter
	baseZerologInstance := zerolog.New(routerWriter).With().Timestamp().Logger()
	cmdLogger := log.NewZerologAdapter(baseZerologInstance)

	// Graceful shutdown of logging sinks
	defer func() {
		cmdLogger.Info().Msg("Shutting down logger...")
		if err := logRouter.Close(); err != nil {
			fmt.Printf("Error during log shutdown: %v", err)
		}
	}()

	// Load .env
	if err := godotenv.Load(); err != nil {
		cmdLogger.Warn().Err(err).Msgf("No .env file found or error thrown while loading it. Relying on existing ENV if vars use {{ env.* }}")
	}

	// Load original workflow YAML
	wf, err := core.LoadWorkflowFromFile(r.Workflow)
	if err != nil {
		cmdLogger.Error().Err(err).Msgf("Failed to load workflow file %s", r.Workflow)
		return fmt.Errorf("could not load workflow file %q: %w", r.Workflow, err)
	}
	cmdLogger.Info().Msgf("Successfully loaded workflow: %s", wf.Name)

	// Get the workflow directory
	workflowAbsPath, err := filepath.Abs(r.Workflow)
	if err != nil {
		cmdLogger.Error().Err(err).Msgf("Could not determine absolute path for workflow file %s", r.Workflow)
		return fmt.Errorf("could not determine absolute path for workflow file %q: %w", r.Workflow, err)
	}
	workflowDir := filepath.Dir(workflowAbsPath)

	// Load varfile YAML
	var varCtx core.VarContext
	if _, statErr := os.Stat(r.Varfile); os.IsNotExist(statErr) {
		cmdLogger.Warn().Msgf("Varfile %s not found. Proceeding without global variables. Required inputs might fail validation if not in ENV.", r.Varfile)
		varCtx = make(core.VarContext)
	} else {
		varCtx, err = core.ResolveVarfile(r.Varfile)
		if err != nil {
			cmdLogger.Warn().Err(err).Msgf("Could not fully resolve varfile %q. Some variable validations might be affected.", r.Varfile)
			if varCtx == nil {
				varCtx = make(core.VarContext)
			}
		} else {
			cmdLogger.Info().Msgf("Successfully loaded and resolved varfile: %s", r.Varfile)
		}
	}

	// Validate required input variables
	if err := core.ValidateRequiredInputs(wf, varCtx); err != nil {
		cmdLogger.Error().Err(err).Msgf("Required input validation failed")
		return err
	}
	cmdLogger.Info().Msgf("Required input validation passed")

	// Initialize and attach secrets redactor
	logRouter.Redactor = security.NewRedactor(wf.Inputs, varCtx)

	// Resolve workflow providers
	resolvedProviders := make(map[string]core.ProviderConfig)
	for _, p := range wf.Providers {
		resolvedP, err := core.ResolveProviderVariables(&p, varCtx)
		if err != nil {
			return fmt.Errorf("could not resolve variables for provider %q: %w", p.Name, err)
		}
		resolvedProviders[p.Name] = *resolvedP
	}

	// Apply fallback API keys for providers with empty API keys
	for name, provider := range resolvedProviders {
		if provider.APIKey == "" {
			cmdLogger.Info().Msgf("API key for provider %q is not defined in the workflow. Falling back to environment variable.", provider.Name)
			fallbackKey := getFallbackKey(provider.Type)
			if fallbackKey != "" {
				provider.APIKey = fallbackKey
				resolvedProviders[name] = provider
			} else {
				cmdLogger.Error().Msgf("API key for provider %q is not defined in the workflow or the expected environment variable", provider.Name)
				return fmt.Errorf("API key for provider %q is not defined in the workflow or the expected environment variable", provider.Name)
			}
		}
	}

	// Create a temporary, resolved copy of the workflow for validation
	validationWf, err := core.InjectVarsIntoWorkflow(wf, varCtx)
	if err != nil {
		return fmt.Errorf("could not resolve global variables for workflow validation: %w", err)
	}

	// Validate runners using the temporary workflow
	if err := core.ValidateWorkflowRunners(validationWf, workflowDir); err != nil {
		return fmt.Errorf("workflow runner validation failed: %w", err)
	}

	cmdLogger.Info().Msg("Workflow validation passed")

	cmdLogger.Info().Msgf("Starting workflow: %q (run ID: %s)", wf.Name, wfRunID)

	// Create and use the workflow engine
	engine := core.NewWorkflowEngine(cmdLogger)
	_, err = engine.ExecuteWorkflow(wf, varCtx, nil, workflowDir, resolvedProviders)
	if err != nil {
		return err
	}

	cmdLogger.Info().Msgf("Workflow completed successfully. Logs can be found at %q", logFilePath)
	return nil
}
