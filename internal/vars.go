package internal

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// VarContext holds resolved input variables
type VarContext map[string]string

// ResolveVarfile loads and resolves a YAML file containing input variables.
func ResolveVarfile(path string) (VarContext, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading varfile: %w", err)
	}

	var raw map[string]string
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing varfile YAML: %w", err)
	}

	// Patterns: {{ env.VAR }} and {{ shell("cmd") }}
	envRe := regexp.MustCompile(`^\s*\{\{\s*env\.([A-Za-z0-9_]+)\s*}}\s*$`)
	shellRe := regexp.MustCompile(`^\s*\{\{\s*shell\((?:"|'` + "`" + `)(.+?)(?:"|'` + "`" + `)\)\s*\}\}\s*$`)

	resolved := make(VarContext, len(raw))
	for key, val := range raw {
		switch {
		case envRe.MatchString(val):
			match := envRe.FindStringSubmatch(val)
			envKey := match[1]
			envVal := os.Getenv(envKey)
			resolved[key] = envVal

		case shellRe.MatchString(val):
			match := shellRe.FindStringSubmatch(val)
			cmdStr := match[1]
			output, err := exec.Command("sh", "-c", cmdStr).Output()
			if err != nil {
				return nil, fmt.Errorf("running shell for %s: %w", key, err)
			}
			resolved[key] = strings.TrimSpace(string(output))

		default:
			// literal
			resolved[key] = val
		}
	}
	return resolved, nil
}

// InjectVarsIntoWorkflow walks every step in the workflow and replaces
// any {{ varName }} occurrences with their value from varCtx.
func InjectVarsIntoWorkflow(wf *Workflow, varCtx VarContext) (*Workflow, error) {
	// regexp to match {{ varName }}
	re := regexp.MustCompile(`\{\{\s*([A-Za-z0-9_]+)\s*}}`)

	// Helper to do replacement on any string
	replace := func(input string) (string, error) {
		// Scan for all placeholders and error if we see one not in varCtx
		for _, m := range re.FindAllStringSubmatch(input, -1) {
			key := m[1]
			if _, exists := varCtx[key]; !exists {
				return "", fmt.Errorf("undefined variable %q in %q", key, input)
			}
		}

		out := re.ReplaceAllStringFunc(input, func(match string) string {
			key := re.FindStringSubmatch(match)[1]
			return varCtx[key]
		})
		return out, nil
	}

	// clone the workflow so we don’t mutate the original
	updated := *wf
	updated.Steps = make([]Step, len(wf.Steps))

	for i, step := range wf.Steps {
		s := step
		var err error

		// prompt
		if s.Prompt != "" {
			s.Prompt, err = replace(s.Prompt)
			if err != nil {
				return nil, fmt.Errorf("injecting vars into prompt of step %q: %w", s.ID, err)
			}

			if len(s.UploadFiles) > 0 {
				updatedUploadFiles := make([]FileToUpload, len(s.UploadFiles))
				for j, fu := range s.UploadFiles {
					updatedFile := fu
					updatedFile.Path, err = replace(fu.Path)
					if err != nil {
						return nil, fmt.Errorf("injecting vars into upload_files path of step %q: %w", s.ID, err)
					}
					updatedUploadFiles[j] = updatedFile
				}
				s.UploadFiles = updatedUploadFiles
			}
		}

		// shell run
		if s.Run != "" {
			s.Run, err = replace(s.Run)
			if err != nil {
				return nil, fmt.Errorf("injecting vars into run of step %q: %w", s.ID, err)
			}
		}

		// API call fields
		if s.Call != nil {
			s.Call.Url, _ = replace(s.Call.Url)
			for k, h := range s.Call.Headers {
				s.Call.Headers[k], _ = replace(h)
			}
			// for simplicity assume body values are strings
			for k, v := range s.Call.Body {
				if str, ok := v.(string); ok {
					newStr, _ := replace(str)
					s.Call.Body[k] = newStr
				}
			}
		}

		updated.Steps[i] = s
	}

	return &updated, nil
}
