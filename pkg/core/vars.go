package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// VarContext holds resolved input variables from dsvars.yml.
type VarContext map[string]string

// varRegex is a package-level compiled regular expression for matching {{ varName }} placeholders.
var varRegex = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9\._-]+)\s*\}\}`)

// ResolveVarfile loads a YAML varfile (e.g. dsvars.yml), parses it, and resolves special values.
func ResolveVarfile(path string) (VarContext, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading varfile %q: %w", path, err)
	}

	var rawVars map[string]string
	if err := yaml.Unmarshal(data, &rawVars); err != nil {
		return nil, fmt.Errorf("parsing varfile YAML from %q: %w", path, err)
	}

	envRe := regexp.MustCompile(`^\s*\{\{\s*env\.([A-Za-z0-9_]+)\s*}}\s*$`)

	resolvedCtx := make(VarContext, len(rawVars))
	for key, val := range rawVars {
		if envRe.MatchString(val) {
			match := envRe.FindStringSubmatch(val)
			envKey := match[1]
			envVal, exists := os.LookupEnv(envKey)
			if !exists {
				log.Printf("warning: environment variable %q not found for varfile key %q", envKey, key)
			}
			resolvedCtx[key] = envVal
		} else {
			resolvedCtx[key] = val
		}
	}
	return resolvedCtx, nil
}

// Helper function to recursively resolve variables in various data structures
func ResolveValue(value any, resolver func(string) (string, error), globals VarContext, results StepResultsContext) (any, error) {
	switch v := value.(type) {
	case string:
		return resolver(v)
	case map[string]any:
		resolvedMap := make(map[string]any)
		for key, val := range v {
			resolvedVal, err := ResolveValue(val, resolver, globals, results)
			if err != nil {
				return nil, fmt.Errorf("resolving map key %q: %w", key, err)
			}
			resolvedMap[key] = resolvedVal
		}
		return resolvedMap, nil
	case []any:
		resolvedSlice := make([]any, len(v))
		for i, item := range v {
			resolvedItem, err := ResolveValue(item, resolver, globals, results)
			if err != nil {
				return nil, fmt.Errorf("resolving slice item at index %d: %w", i, err)
			}
			resolvedSlice[i] = resolvedItem
		}
		return resolvedSlice, nil
	default:
		// For other types (int, bool, etc.), return as is
		return v, nil
	}
}

// ResolveStepVariables takes a single step and resolves all its templated
// fields using the global context and the results of previously executed steps.
func ResolveStepVariables(step *Step, globals VarContext, results StepResultsContext) (*Step, error) {
	// Create a deep copy of the step to avoid modifying the original workflow definition.
	var resolvedStep Step
	b, _ := yaml.Marshal(step)
	if err := yaml.Unmarshal(b, &resolvedStep); err != nil {
		return nil, fmt.Errorf("deep copying step for resolution: %w", err)
	}

	resolutionCtx := make(VarContext)
	for k, v := range globals {
		resolutionCtx[k] = v
	}

	// For each file, resolve its path first, then add its `name` as a variable
	// that resolves to the basename of the path
	for i, file := range resolvedStep.BrowserConfig.UploadFiles {
		resolvedPath, err := ResolveStringWithContext(file.Path, resolutionCtx, results)
		if err != nil {
			return nil, fmt.Errorf("resolving path for file variable %q: %w", file.Name, err)
		}
		resolvedStep.BrowserConfig.UploadFiles[i].Path = resolvedPath
		resolutionCtx[file.Name] = filepath.Base(resolvedPath)
	}

	var err error
	coreResolver := func(input string) (string, error) {
		return ResolveStringWithContext(input, resolutionCtx, results)
	}

	// Resolve all string fields in the step
	resolvedStep.BrowserConfig.Prompt, err = coreResolver(resolvedStep.BrowserConfig.Prompt)
	if err != nil {
		return nil, fmt.Errorf("resolving browser.prompt for step %q: %w", step.ID, err)
	}
	resolvedStep.BrowserConfig.TargetDownloadDir, err = coreResolver(resolvedStep.BrowserConfig.TargetDownloadDir)
	if err != nil {
		return nil, fmt.Errorf("resolving browser.download_dir for step %q: %w", step.ID, err)
	}
	resolvedStep.BrowserConfig.DataDir, err = coreResolver(resolvedStep.BrowserConfig.DataDir)
	if err != nil {
		return nil, fmt.Errorf("resolving browser.data_dir for step %q: %w", step.ID, err)
	}
	resolvedStep.BrowserConfig.OutputSchemaFile, err = coreResolver(resolvedStep.BrowserConfig.OutputSchemaFile)
	if err != nil {
		return nil, fmt.Errorf("resolving browser.output_schema for step %q: %w", step.ID, err)
	}

	if resolvedStep.Command != nil {
		resolvedStep.Command.Path, err = coreResolver(resolvedStep.Command.Path)
		if err != nil {
			return nil, fmt.Errorf("resolving command.path for step %q: %w", step.ID, err)
		}
		resolvedStep.Command.Inline, err = coreResolver(resolvedStep.Command.Inline)
		if err != nil {
			return nil, fmt.Errorf("resolving command.inline for step %q: %w", step.ID, err)
		}
		resolvedStep.Command.Interpreter, err = coreResolver(resolvedStep.Command.Interpreter)
		if err != nil {
			return nil, fmt.Errorf("resolving command.interpreter for step %q: %w", step.ID, err)
		}
	}

	if resolvedStep.Call != nil {
		resolvedStep.Call.Url, err = coreResolver(resolvedStep.Call.Url)
		if err != nil {
			return nil, fmt.Errorf("resolving call.url for step %q: %w", step.ID, err)
		}

		if resolvedStep.Call.Headers != nil {
			resolvedHeaders := make(map[string]string)
			for k, v := range resolvedStep.Call.Headers {
				resolvedV, errHeader := coreResolver(v)
				if errHeader != nil {
					return nil, fmt.Errorf("resolving call.headers[%s] for step %q: %w", k, step.ID, errHeader)
				}
				resolvedHeaders[k] = resolvedV
			}
			resolvedStep.Call.Headers = resolvedHeaders
		}

		if resolvedStep.Call.Body != nil {
			resolvedBody, errBody := ResolveValue(resolvedStep.Call.Body, coreResolver, resolutionCtx, results)
			if errBody != nil {
				return nil, fmt.Errorf("resolving call.body for step %q: %w", step.ID, errBody)
			}
			if castedBody, ok := resolvedBody.(map[string]any); ok {
				resolvedStep.Call.Body = castedBody
			} else if resolvedBody != nil { // if resolvedBody is nil, it means original body was nil.
				return nil, fmt.Errorf("resolved call.body for step %q is not a map, got %T", step.ID, resolvedBody)
			}
		}
	}

	for i := range resolvedStep.BrowserConfig.AllowedDomains {
		resolvedStep.BrowserConfig.AllowedDomains[i], err = coreResolver(resolvedStep.BrowserConfig.AllowedDomains[i])
		if err != nil {
			return nil, fmt.Errorf("resolving browser.allowed_domains[%d] for step %q: %w", i, step.ID, err)
		}
	}

	// MaxSteps and MaxFailures (if they support templating for some reason, usually they are static)
	if resolvedStep.BrowserConfig.MaxSteps != nil {
		maxStepsStr, err := coreResolver(strconv.Itoa(*resolvedStep.BrowserConfig.MaxSteps))
		if err != nil {
			return nil, fmt.Errorf("resolving browser.max_steps for step %q: %w", step.ID, err)
		}
		maxStepsInt, convErr := strconv.Atoi(maxStepsStr)
		if convErr != nil {
			return nil, fmt.Errorf("resolved browser.max_steps for step %q (%s) is not an int: %w", step.ID, maxStepsStr, convErr)
		}
		resolvedStep.BrowserConfig.MaxSteps = &maxStepsInt
	}

	// Resolve step.Timeout string
	if resolvedStep.Timeout != "" {
		resolvedStep.Timeout, err = coreResolver(resolvedStep.Timeout)
		if err != nil {
			return nil, fmt.Errorf("resolving timeout for step %q: %w", step.ID, err)
		}
	}

	if resolvedStep.MaxFailures != nil {
		maxFailuresStr, err := coreResolver(strconv.Itoa(*resolvedStep.MaxFailures))
		if err != nil {
			return nil, fmt.Errorf("resolving max_failures for step %q: %w", step.ID, err)
		}
		maxFailuresInt, convErr := strconv.Atoi(maxFailuresStr)
		if convErr != nil {
			return nil, fmt.Errorf("resolved max_failures for step %q (%s) is not an int: %w", step.ID, maxFailuresStr, convErr)
		}
		resolvedStep.MaxFailures = &maxFailuresInt
	}

	return &resolvedStep, nil
}

// ResolveStringWithContext is the core template resolution engine.
func ResolveStringWithContext(input string, globals VarContext, results StepResultsContext) (string, error) {
	var firstErr error
	output := varRegex.ReplaceAllStringFunc(input, func(match string) string {
		if firstErr != nil {
			return match // Stop processing if an error has occurred
		}

		key := varRegex.FindStringSubmatch(match)[1]
		val, found := FindValueInContext(key, globals, results)

		if !found {
			firstErr = fmt.Errorf("undefined variable: %s", key)
			return match
		}
		return fmt.Sprintf("%v", val)
	})

	if firstErr != nil {
		return "", firstErr
	}
	return output, nil
}

// FindValueInContext orchestrates the lookup for a variable.
func FindValueInContext(key string, globals VarContext, results StepResultsContext) (any, bool) {
	wantsJSON := strings.HasSuffix(key, ".json")
	if wantsJSON {
		key = strings.TrimSuffix(key, ".json")
	}

	var value any
	var found bool

	// Try to resolve as a `steps` variable
	if strings.HasPrefix(key, "steps.") {
		parts := strings.Split(key, ".")
		if len(parts) < 3 { // Must be at least `steps.id.field`
			return nil, false
		}
		stepID := parts[1]
		field := parts[2]

		if result, ok := results[stepID]; ok {
			switch field {
			case "output":
				value, found = GetNestedValue(result.Output, parts[3:])
			case "output_file":
				if len(parts) == 3 {
					value, found = result.OutputFile, true
				}
			}
		}
	} else {
		if val, ok := globals[key]; ok {
			value, found = val, true
		}
	}

	if !found {
		return nil, false
	}

	if wantsJSON {
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			return fmt.Sprintf("{\"error\": \"failed to marshal to json: %v\"}", err), true
		}
		return string(jsonBytes), true
	}

	return value, true
}

// GetNestedValue traverses a data structure (map or string) using a path slice.
func GetNestedValue(data any, path []string) (any, bool) {
	if len(path) == 0 {
		return data, true
	}
	if data == nil {
		return nil, false
	}

	current := data
	for _, keyInPath := range path {
		switch typedCurrent := current.(type) {
		case map[string]any:
			if val, exists := typedCurrent[keyInPath]; exists {
				current = val
			} else {
				return nil, false
			}
		case map[string]string:
			if val, exists := typedCurrent[keyInPath]; exists {
				current = val
			} else {
				return nil, false
			}
		default:
			return nil, false
		}
	}
	return current, true
}

// InjectVarsIntoWorkflow is kept for the linter, but it only resolves global variables.
func InjectVarsIntoWorkflow(wf *Workflow, globalVarCtx VarContext) (*Workflow, error) {
	if wf == nil {
		return nil, fmt.Errorf("injecting vars into nil workflow")
	}

	// Create a deep copy
	var updatedWf Workflow
	buf := new(bytes.Buffer)
	if err := yaml.NewEncoder(buf).Encode(wf); err != nil {
		return nil, err
	}
	if err := yaml.NewDecoder(buf).Decode(&updatedWf); err != nil {
		return nil, err
	}

	resolver := func(input string) string {
		return varRegex.ReplaceAllStringFunc(input, func(match string) string {
			key := varRegex.FindStringSubmatch(match)[1]

			if val, ok := globalVarCtx[key]; ok {
				return val
			}

			return match
		})
	}

	for i, step := range updatedWf.Steps {
		s := step // Work on a copy
		s.BrowserConfig.Prompt = resolver(s.BrowserConfig.Prompt)
		s.BrowserConfig.TargetDownloadDir = resolver(s.BrowserConfig.TargetDownloadDir)
		s.BrowserConfig.DataDir = resolver(s.BrowserConfig.DataDir)

		for j := range s.BrowserConfig.UploadFiles {
			s.BrowserConfig.UploadFiles[j].Path = resolver(s.BrowserConfig.UploadFiles[j].Path)
		}

		if s.Command != nil {
			s.Command.Inline = resolver(s.Command.Inline)
			s.Command.Path = resolver(s.Command.Path)
			s.Command.Interpreter = resolver(s.Command.Interpreter)
		}

		updatedWf.Steps[i] = s
	}

	return &updatedWf, nil
}

func ResolveProviderVariables(p *ProviderConfig, globals VarContext) (*ProviderConfig, error) {
	// Create a deep copy to avoid modifying the original
	var resolvedProvider ProviderConfig
	b, _ := yaml.Marshal(p)
	if err := yaml.Unmarshal(b, &resolvedProvider); err != nil {
		return nil, fmt.Errorf("deep copying provider for resolution: %w", err)
	}

	resolvedKey, err := ResolveStringWithContext(resolvedProvider.APIKey, globals, nil)
	if err != nil {
		return nil, fmt.Errorf("resolving 'api_key' for provider %q: %w", p.Name, err)
	}
	resolvedProvider.APIKey = resolvedKey

	return &resolvedProvider, nil
}
