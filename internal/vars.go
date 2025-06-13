package internal

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
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
	shellRe := regexp.MustCompile(`^\s*\{\{\s*shell\((?:"|'` + "`" + `)(.+?)(?:"|'` + "`" + `)\)\s*\}\}\s*$`)

	resolvedCtx := make(VarContext, len(rawVars))
	for key, val := range rawVars {
		switch {
		case envRe.MatchString(val):
			match := envRe.FindStringSubmatch(val)
			envKey := match[1]
			envVal, exists := os.LookupEnv(envKey)
			if !exists {
				log.Printf("warning: environment variable %q not found for varfile key %q", envKey, key)
			}
			resolvedCtx[key] = envVal
		case shellRe.MatchString(val):
			match := shellRe.FindStringSubmatch(val)
			cmdStr := match[1]
			// #nosec G204 -- User is explicitly asking for a shell command to be run.
			output, execErr := exec.Command("sh", "-c", cmdStr).Output()
			if execErr != nil {
				return nil, fmt.Errorf("running shell command for varfile key %q (%s): %w", key, cmdStr, execErr)
			}
			resolvedCtx[key] = strings.TrimSpace(string(output))
		default:
			resolvedCtx[key] = val
		}
	}
	return resolvedCtx, nil
}

// ResolveStepVariables takes a single step and resolves all its templated fields
// using the global context and the results of previously executed steps.
func ResolveStepVariables(step *Step, globals VarContext, results StepResultsContext) (*Step, error) {
	// Create a deep copy of the step to avoid modifying the original workflow definition.
	// A simple marshal/unmarshal to YAML is an effective way to deep copy these structs.
	var resolvedStep Step
	b, _ := yaml.Marshal(step)
	if err := yaml.Unmarshal(b, &resolvedStep); err != nil {
		return nil, fmt.Errorf("failed to deep copy step for resolution: %w", err)
	}

	var err error
	resolver := func(input string) (string, error) {
		return resolveStringWithContext(input, globals, results)
	}

	// Resolve all string fields in the step
	resolvedStep.Prompt, err = resolver(resolvedStep.Prompt)
	if err != nil {
		return nil, err
	}
	resolvedStep.TargetDownloadDir, err = resolver(resolvedStep.TargetDownloadDir)
	if err != nil {
		return nil, err
	}
	resolvedStep.OutputSchemaFile, err = resolver(resolvedStep.OutputSchemaFile)
	if err != nil {
		return nil, err
	}

	for i := range resolvedStep.UploadFiles {
		resolvedStep.UploadFiles[i].Path, err = resolver(resolvedStep.UploadFiles[i].Path)
		if err != nil {
			return nil, err
		}
	}

	if resolvedStep.Run != nil {
		resolvedStep.Run.Path, err = resolver(resolvedStep.Run.Path)
		if err != nil {
			return nil, err
		}
		resolvedStep.Run.Inline, err = resolver(resolvedStep.Run.Inline)
		if err != nil {
			return nil, err
		}
		resolvedStep.Run.Interpreter, err = resolver(resolvedStep.Run.Interpreter)
		if err != nil {
			return nil, err
		}
	}

	if resolvedStep.Call != nil {
		resolvedStep.Call.Url, err = resolver(resolvedStep.Call.Url)
		if err != nil {
			return nil, err
		}
		for k, v := range resolvedStep.Call.Headers {
			resolvedStep.Call.Headers[k], err = resolver(v)
			if err != nil {
				return nil, err
			}
		}
		for k, v := range resolvedStep.Call.Body {
			if strVal, ok := v.(string); ok {
				resolvedStep.Call.Body[k], err = resolver(strVal)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	for i := range resolvedStep.AllowedDomains {
		resolvedStep.AllowedDomains[i], err = resolver(resolvedStep.AllowedDomains[i])
		if err != nil {
			return nil, err
		}
	}

	if resolvedStep.MaxSteps != nil {
		maxStepsString, err := resolver(strconv.Itoa(*resolvedStep.MaxSteps))
		if err != nil {
			return nil, err
		}
		maxStepsInt, err := strconv.Atoi(maxStepsString)
		if err != nil {
			return nil, err
		}
		resolvedStep.MaxSteps = &maxStepsInt
	}

	if resolvedStep.MaxFailures != nil {
		maxFailuresString, err := resolver(strconv.Itoa(*resolvedStep.MaxFailures))
		if err != nil {
			return nil, err
		}
		maxFailuresInt, err := strconv.Atoi(maxFailuresString)
		if err != nil {
			return nil, err
		}
		resolvedStep.MaxFailures = &maxFailuresInt
	}

	return &resolvedStep, nil
}

// resolveStringWithContext is the core template resolution engine.
func resolveStringWithContext(input string, globals VarContext, results StepResultsContext) (string, error) {
	var firstErr error
	output := varRegex.ReplaceAllStringFunc(input, func(match string) string {
		if firstErr != nil {
			return match // Stop processing if an error has occurred
		}

		key := varRegex.FindStringSubmatch(match)[1]
		val, found := findValueInContext(key, globals, results)

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

// findValueInContext orchestrates the lookup for a variable.
func findValueInContext(key string, globals VarContext, results StepResultsContext) (interface{}, bool) {
	// 1. Try to resolve as a `steps` variable
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
				return getNestedValue(result.Output, parts[3:])
			case "output_file":
				if len(parts) == 3 {
					return result.OutputFile, true
				}
			}
		}
		return nil, false // Step or field not found
	}

	// 2. Fallback to global variables
	if val, ok := globals[key]; ok {
		return val, true
	}

	return nil, false
}

// getNestedValue traverses a data structure (map or string) using a path slice.
func getNestedValue(data interface{}, path []string) (interface{}, bool) {
	if len(path) == 0 {
		return data, true // Request for the whole object/string
	}
	if data == nil {
		return nil, false
	}

	current := data
	for _, key := range path {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, false // Cannot traverse further
		}
		if val, exists := m[key]; exists {
			current = val
		} else {
			return nil, false
		}
	}
	return current, true
}

// InjectVarsIntoWorkflow is kept for the linter, but it only resolves global variables.
func InjectVarsIntoWorkflow(wf *Workflow, globalVarCtx VarContext) (*Workflow, error) {
	if wf == nil {
		return nil, fmt.Errorf("cannot inject vars into nil workflow")
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
		s.Prompt = resolver(s.Prompt)
		s.TargetDownloadDir = resolver(s.TargetDownloadDir)
		if s.Run != nil {
			s.Run.Inline = resolver(s.Run.Inline)
		}
		// Limited resolution for linting is sufficient
		updatedWf.Steps[i] = s
	}

	return &updatedWf, nil
}