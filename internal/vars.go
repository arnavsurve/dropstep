package internal

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// VarContext holds resolved input variables
type VarContext map[string]string

// varRegex is a package-level compiled regular expression for matching {{ varName }} placeholders.
var varRegex = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_]+)\s*}}`)

// ResolveVarfile loads a YAML file specified by path, parses it as a map of
// variable names to string values, and resolves any special templating within those values
// (e.g., {{ env.VAR }}, {{ shell("cmd") }}).
// It returns a VarContext containing the fully resolved global variables.
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
				// Consider if this should be an error or an empty string.
				// For now, matches behavior of os.Getenv which returns "" for non-existent.
				log.Printf("warning: environment variable %q not found for varfile key %q", envKey, key)
			}
			resolvedCtx[key] = envVal
		case shellRe.MatchString(val):
			match := shellRe.FindStringSubmatch(val)
			cmdStr := match[1]
			// #nosec G204 -- User is explicitly asking for a shell command to be run.
			// This is a feature, and the security implication is owned by the user crafting the varfile.
			output, execErr := exec.Command("sh", "-c", cmdStr).Output()
			if execErr != nil {
				return nil, fmt.Errorf("running shell command for varfile key %q (%s): %w", key, cmdStr, execErr)
			}
			resolvedCtx[key] = strings.TrimSpace(string(output))
		default:
			resolvedCtx[key] = val // Literal value
		}
	}
	return resolvedCtx, nil
}

// stepVariableResolver encapsulates the logic for resolving variables within a single step.
// It manages different contexts (global, prompt-specific) for variable lookup.
type stepVariableResolver struct {
	globalContext       VarContext
	promptContext       VarContext
	resolvedUploadFiles []FileToUpload
}

// newStepVariableResolver creates and initializes a resolver for a given step.
// It resolves paths in `originalUploadFiles` using `globalCtx` and then builds
// the `promptContext`.
func newStepVariableResolver(globalCtx VarContext, stepID string, originalUploadFiles []FileToUpload) (*stepVariableResolver, error) {
	// Resolve paths in UploadFiles using the globalContext
	resolvedUploads := make([]FileToUpload, len(originalUploadFiles))
	for i, fu := range originalUploadFiles {
		if fu.Path == "" {
			resolvedUploads[i] = FileToUpload{Name: fu.Name, Path: ""}
			continue
		}
		resolvedPath, err := resolveStringVariables(fu.Path, globalCtx, "upload file path")
		if err != nil {
			return nil, fmt.Errorf("resolving path for upload file '%s': %w", fu.Name, err)
		}
		resolvedUploads[i] = FileToUpload{Name: fu.Name, Path: resolvedPath}
	}

	// Build the promptContext: globalContext + upload_file.Name -> resolvedPath
	// Estimate capacity to potentially reduce reallocations
	promptCtx := make(VarContext, len(globalCtx)+len(resolvedUploads))
	for k, v := range globalCtx {
		promptCtx[k] = v
	}
	for _, fu := range resolvedUploads { // fu.Path is now the resolved path
		if fu.Name == "" { // Skip if name is empty, though schema should enforce it.
			continue
		}
		if _, exists := promptCtx[fu.Name]; exists {
			log.Printf("Warning: Upload file name '%s' in step '%s' overrides an existing global variable in prompt context.", fu.Name, stepID)
		}
		promptCtx[fu.Name] = fu.Path
	}

	return &stepVariableResolver{
		globalContext:       globalCtx,
		promptContext:       promptCtx,
		resolvedUploadFiles: resolvedUploads,
	}, nil
}

// Resolve uses the appropriate context (global or prompt) to substitute variables in the input string.
func (r *stepVariableResolver) Resolve(input string, contextType string, fieldDescription string) (string, error) {
	if input == "" {
		return "", nil
	}

	var ctxToUse VarContext
	switch contextType {
	case "prompt":
		ctxToUse = r.promptContext
	case "global":
		ctxToUse = r.globalContext
	default:
		// This should ideally not happen if called correctly from InjectVarsIntoWorkflow
		return "", fmt.Errorf("internal error: unknown context type %q for resolving %s", contextType, fieldDescription)
	}
	return resolveStringVariables(input, ctxToUse, fieldDescription)
}

// GetResolvedUploadFiles returns the list of upload files with their paths fully resolved.
func (r *stepVariableResolver) GetResolvedUploadFiles() []FileToUpload {
	return r.resolvedUploadFiles
}


// resolveStringVariables is the core function for replacing {{varName}} placeholders in a string
// using the provided context. It returns an error if any variable is undefined.
// fieldDescription is used for more informative error messages.
func resolveStringVariables(input string, context VarContext, fieldDescription string) (string, error) {
	// Validate that all variables in the input string exist in the context
	var missingVars []string
	for _, m := range varRegex.FindAllStringSubmatch(input, -1) {
		key := m[1]
		if _, exists := context[key]; !exists {
			// Collect all missing variables for a comprehensive error message
			found := false
			for _, mv := range missingVars {
				if mv == key {
					found = true
					break
				}
			}
			if !found {
				missingVars = append(missingVars, key)
			}
		}
	}
	if len(missingVars) > 0 {
		return "", fmt.Errorf("undefined variable(s) [%s] in %s: %q", strings.Join(missingVars, ", "), fieldDescription, input)
	}

	// If all variables are defined, perform the replacement
	output := varRegex.ReplaceAllStringFunc(input, func(match string) string {
		key := varRegex.FindStringSubmatch(match)[1]
		return context[key]
	})
	return output, nil
}

// InjectVarsIntoWorkflow processes a workflow, injecting variables into various fields of its steps.
// It uses a globalVarCtx (typically from dsvars.yml) and creates a stepVariableResolver
// for each step to handle context-specific variable resolution (e.g., for prompts).
func InjectVarsIntoWorkflow(wf *Workflow, globalVarCtx VarContext) (*Workflow, error) {
	if wf == nil {
		return nil, fmt.Errorf("cannot inject vars into nil workflow")
	}
	if globalVarCtx == nil {
		// Allow empty globalVarCtx, but not nil, to avoid nil pointer dereferences.
		// If it's truly nil, initialize it.
		globalVarCtx = make(VarContext)
	}

	// Create a new workflow structure to hold the updated steps, avoiding mutation of the input `wf`.
	updatedWf := *wf                              // Shallow copy of the workflow structure
	updatedWf.Steps = make([]Step, len(wf.Steps)) // Allocate a new slice for steps

	for i, step := range wf.Steps {
		s := step // Work on a copy of the current step

		// 1. Create a variable resolver for this specific step.
		// This resolver internally handles resolving upload file paths and building the prompt context.
		resolver, err := newStepVariableResolver(globalVarCtx, s.ID, s.UploadFiles)
		if err != nil {
			return nil, fmt.Errorf("step %q (%s): failed to initialize variable resolver: %w", s.ID, s.Uses, err)
		}

		// 2. Resolve templated fields using the resolver.
		s.Prompt, err = resolver.Resolve(s.Prompt, "prompt", "step prompt")
		if err != nil {
			return nil, fmt.Errorf("step %q (%s): %w", s.ID, s.Uses, err)
		}

		// Resolve TargetDownloadDir
		if s.TargetDownloadDir != "" {
			s.TargetDownloadDir, err = resolver.Resolve(s.TargetDownloadDir, "global", "target download directory")
			if err != nil {
				return nil, fmt.Errorf("step %q (%s): resolving target_download_dir: %w", s.ID, s.Uses, err)
			}
		}

		// Only set resolvedUploadFiles if there are any. This is because
		// many step types rely on s.UploadFiles being nil for validation
		resolvedUploadFiles := resolver.GetResolvedUploadFiles()
		if len(resolvedUploadFiles) > 0 {
			s.UploadFiles = resolver.resolvedUploadFiles
		}

		// Resolve path for OutputSchemaFile using global context
		if s.OutputSchemaFile != "" {
			resolvedSchemaPath, pathErr := resolver.Resolve(s.OutputSchemaFile, "global", "output schema file path")
			if pathErr != nil {
				return nil, fmt.Errorf("step %q (%s): %w", s.ID, s.Uses, pathErr)
			}
			s.OutputSchemaFile = resolvedSchemaPath // Update the step's copy with the resolved path
		}

		if s.Run != nil {
			if s.Run.Path != "" {
				s.Run.Path, err = resolver.Resolve(s.Run.Path, "global", "shell script path")
				if err != nil {
					return nil, fmt.Errorf("step %q (%s): %w", s.ID, s.Uses, err)
				}
			}

			if s.Run.Inline != "" {
				s.Run.Inline, err = resolver.Resolve(s.Run.Inline, "global", "inline shell script")
				if err != nil {
					return nil, fmt.Errorf("step %q (%q): %w", s.ID, s.Uses, err)
				}
			}

			if s.Run.Interpreter != "" {
				s.Run.Interpreter, err = resolver.Resolve(s.Run.Interpreter, "global", "shell to use for shell script")
				if err != nil {
					return nil, fmt.Errorf("step %q (%q): %w", s.ID, s.Uses, err)
				}
			}
		}

		if s.Call != nil {
			// Work on a copy of ApiCall to avoid modifying the original step's Call pointer directly
			// if the original step was part of a shared slice or map elsewhere.
			call := *s.Call

			call.Url, err = resolver.Resolve(call.Url, "global", "API call URL")
			if err != nil {
				return nil, fmt.Errorf("step %q (%s): %w", s.ID, s.Uses, err)
			}

			if len(call.Headers) > 0 {
				updatedHeaders := make(map[string]string, len(call.Headers))
				for key, val := range call.Headers {
					updatedHeaders[key], err = resolver.Resolve(val, "global", fmt.Sprintf("API call header '%s'", key))
					if err != nil {
						return nil, fmt.Errorf("step %q (%s): %w", s.ID, s.Uses, err)
					}
				}
				call.Headers = updatedHeaders
			}

			if len(call.Body) > 0 {
				updatedBody := make(map[string]any, len(call.Body))
				for key, val := range call.Body {
					if strVal, ok := val.(string); ok {
						updatedBody[key], err = resolver.Resolve(strVal, "global", fmt.Sprintf("API call body field '%s'", key))
						if err != nil {
							return nil, fmt.Errorf("step %q (%s): %w", s.ID, s.Uses, err)
						}
					} else {
						updatedBody[key] = val // Non-string values are kept as is
					}
				}
				call.Body = updatedBody
			}
			s.Call = &call // Assign the modified copy back
		}

		updatedWf.Steps[i] = s // Store the fully processed copy of the step
	}

	return &updatedWf, nil
}
