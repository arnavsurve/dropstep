package runners_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arnavsurve/dropstep/pkg/core"
	"github.com/arnavsurve/dropstep/pkg/log"
	"github.com/arnavsurve/dropstep/pkg/steprunner/runners"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestShellRunner_Validate tests the validation logic for shell handlers
func TestShellRunner_Validate(t *testing.T) {
	tests := []struct {
		name        string
		step        core.Step
		shouldError bool
		errorMsg    string
	}{
		{
			name: "Valid shell command - inline",
			step: core.Step{
				ID: "valid_step",
				Command: &core.CommandBlock{
					Inline: "echo 'hello'",
				},
			},
			shouldError: false,
		},
		{
			name: "Valid shell command - path",
			step: core.Step{
				ID: "valid_step",
				Command: &core.CommandBlock{
					Path: "/path/to/script.sh",
				},
			},
			shouldError: false,
		},
		{
			name: "Invalid - both inline and path",
			step: core.Step{
				ID: "invalid_step",
				Command: &core.CommandBlock{
					Inline: "echo 'hello'",
					Path:   "/path/to/script.sh",
				},
			},
			shouldError: true,
			errorMsg:    "must only define either 'inline' or 'path'",
		},
		{
			name: "Invalid - no command",
			step: core.Step{
				ID:      "invalid_step",
				Command: &core.CommandBlock{},
			},
			shouldError: true,
			errorMsg:    "must define either 'inline' or 'path'",
		},
		{
			name: "Invalid - missing command block",
			step: core.Step{
				ID: "invalid_step",
			},
			shouldError: true,
			errorMsg:    "must define 'run'",
		},
		{
			name: "Invalid - has prompt",
			step: core.Step{
				ID: "invalid_step",
				BrowserConfig: core.BrowserConfig{
					Prompt: "Some prompt",
				},
				Command: &core.CommandBlock{Inline: "echo 'hello'"},
			},
			shouldError: true,
			errorMsg:    "must not define 'browser.prompt'",
		},
		{
			name: "Invalid - has upload files",
			step: core.Step{
				ID: "invalid_step",
				BrowserConfig: core.BrowserConfig{
					UploadFiles: []core.FileToUpload{
						{Name: "file", Path: "/path"},
					},
				},
				Command: &core.CommandBlock{Inline: "echo 'hello'"},
			},
			shouldError: true,
			errorMsg:    "must not define 'browser.upload_files'",
		},
		{
			name: "Invalid - has download dir",
			step: core.Step{
				ID: "invalid_step",
				BrowserConfig: core.BrowserConfig{
					TargetDownloadDir: "/downloads",
				},
				Command: &core.CommandBlock{Inline: "echo 'hello'"},
			},
			shouldError: true,
			errorMsg:    "must not define 'browser.download_dir'",
		},
		{
			name: "Invalid - has output schema",
			step: core.Step{
				ID: "invalid_step",
				BrowserConfig: core.BrowserConfig{
					OutputSchemaFile: "/schema.json",
				},
				Command: &core.CommandBlock{Inline: "echo 'hello'"},
			},
			shouldError: true,
			errorMsg:    "must not define 'browser.output_schema'",
		},
		{
			name: "Invalid - has HTTP call",
			step: core.Step{
				ID:      "invalid_step",
				Call:    &core.HTTPCall{Url: "https://example.com"},
				Command: &core.CommandBlock{Inline: "echo 'hello'"},
			},
			shouldError: true,
			errorMsg:    "must not define 'call'",
		},
		{
			name: "Invalid - has allowed domains",
			step: core.Step{
				ID: "invalid_step",
				BrowserConfig: core.BrowserConfig{
					AllowedDomains: []string{"example.com"},
				},
				Command: &core.CommandBlock{Inline: "echo 'hello'"},
			},
			shouldError: true,
			errorMsg:    "must not define 'browser.allowed_domains'",
		},
		{
			name: "Invalid - has max steps",
			step: core.Step{
				ID: "invalid_step",
				BrowserConfig: core.BrowserConfig{
					MaxSteps: func() *int { i := 5; return &i }(),
				},
				Command: &core.CommandBlock{Inline: "echo 'hello'"},
			},
			shouldError: true,
			errorMsg:    "must not define 'browser.max_steps'",
		},
		{
			name: "Invalid - has max failures",
			step: core.Step{
				ID:          "invalid_step",
				MaxFailures: func() *int { i := 3; return &i }(),
				Command:     &core.CommandBlock{Inline: "echo 'hello'"},
			},
			shouldError: true,
			errorMsg:    "must not define 'max_failures'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseZerologInstance := zerolog.New(io.Discard)
			logger := log.NewZerologAdapter(baseZerologInstance)
			ctx := core.ExecutionContext{
				Step:   tt.step,
				Logger: logger,
			}

			sh := &runners.ShellRunner{StepCtx: ctx}
			err := sh.Validate()

			if tt.shouldError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Only run basic tests for the ShellHandler.Run method since proper mocking requires
// more sophisticated testing approaches or refactoring the production code
func TestShellHandler_RunBasic(t *testing.T) {
	// Skip this test during normal test runs since it depends on the system shell
	if testing.Short() {
		t.Skip("Skipping shell execution tests in short mode")
	}

	// Create a temporary directory to work with
	tempDir := t.TempDir()
	
	// Create a simple test script
	scriptPath := filepath.Join(tempDir, "test_script.sh")
	scriptContent := "#!/bin/bash\necho 'hello from script'"
	err := os.WriteFile(scriptPath, []byte(scriptContent), 0755)
	require.NoError(t, err)
	
	tests := []struct {
		name           string
		command        *core.CommandBlock
		expectedOutput string
		shouldError    bool
	}{
		{
			name: "Inline echo command",
			command: &core.CommandBlock{
				Inline: "echo 'hello'",
			},
			expectedOutput: "hello",
			shouldError:    false,
		},
		{
			name: "Script file",
			command: &core.CommandBlock{
				Path: scriptPath,
			},
			expectedOutput: "hello from script",
			shouldError:    false,
		},
		{
			name: "JSON output",
			command: &core.CommandBlock{
				Inline: "echo '{\"key\":\"value\",\"number\":42}'",
			},
			expectedOutput: "",  // Not checking exact output, will verify it's JSON
			shouldError:    false,
		},
		{
			name: "Command with error",
			command: &core.CommandBlock{
				Inline: "non_existent_command",
			},
			shouldError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a logger that writes to a buffer we can inspect
			var logBuffer bytes.Buffer
			logger := zerolog.New(&logBuffer).With().Timestamp().Logger()
			
			// Create a shell runner with the command
			step := core.Step{
				ID:      "test_shell_step",
				Command: tt.command,
			}
			
			ctx := core.ExecutionContext{
				Step:        step,
				Logger:      log.NewZerologAdapter(logger),
				WorkflowDir: tempDir,
			}
			
			sh := &runners.ShellRunner{StepCtx: ctx}
			
			// Execute the command
			result, err := sh.Run()
			
			// Check for expected errors
			if tt.shouldError {
				assert.Error(t, err)
				return
			}
			
			// If no error expected, validate the result
			require.NoError(t, err)
			require.NotNil(t, result)
			
			// Check for JSON output
			if strings.Contains(tt.command.Inline, "echo '{\"key\"") {
				outputMap, ok := result.Output.(map[string]interface{})
				require.True(t, ok, "Expected output to be a map for JSON")
				assert.Equal(t, "value", outputMap["key"])
				assert.Equal(t, float64(42), outputMap["number"])
			} else if tt.expectedOutput != "" {
				// Check for plain text output
				assert.Equal(t, tt.expectedOutput, result.Output)
			}
		})
	}
}
