package handlers

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arnavsurve/dropstep/internal"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestShellHandler_Validate tests the validation logic for shell handlers
func TestShellHandler_Validate(t *testing.T) {
	tests := []struct {
		name        string
		step        internal.Step
		shouldError bool
		errorMsg    string
	}{
		{
			name: "Valid shell command - inline",
			step: internal.Step{
				ID: "valid_step",
				Command: &internal.CommandBlock{
					Inline: "echo 'hello'",
				},
			},
			shouldError: false,
		},
		{
			name: "Valid shell command - path",
			step: internal.Step{
				ID: "valid_step",
				Command: &internal.CommandBlock{
					Path: "/path/to/script.sh",
				},
			},
			shouldError: false,
		},
		{
			name: "Invalid - both inline and path",
			step: internal.Step{
				ID: "invalid_step",
				Command: &internal.CommandBlock{
					Inline: "echo 'hello'",
					Path:   "/path/to/script.sh",
				},
			},
			shouldError: true,
			errorMsg:    "must only define either 'inline' or 'path'",
		},
		{
			name: "Invalid - no command",
			step: internal.Step{
				ID:      "invalid_step",
				Command: &internal.CommandBlock{},
			},
			shouldError: true,
			errorMsg:    "must define either 'inline' or 'path'",
		},
		{
			name: "Invalid - missing command block",
			step: internal.Step{
				ID: "invalid_step",
			},
			shouldError: true,
			errorMsg:    "must define 'run'",
		},
		{
			name: "Invalid - has prompt",
			step: internal.Step{
				ID:      "invalid_step",
				Prompt:  "Some prompt",
				Command: &internal.CommandBlock{Inline: "echo 'hello'"},
			},
			shouldError: true,
			errorMsg:    "must not define 'prompt'",
		},
		{
			name: "Invalid - has upload files",
			step: internal.Step{
				ID: "invalid_step",
				UploadFiles: []internal.FileToUpload{
					{Name: "file", Path: "/path"},
				},
				Command: &internal.CommandBlock{Inline: "echo 'hello'"},
			},
			shouldError: true,
			errorMsg:    "must not define 'upload_files'",
		},
		{
			name: "Invalid - has download dir",
			step: internal.Step{
				ID:                "invalid_step",
				TargetDownloadDir: "/downloads",
				Command:           &internal.CommandBlock{Inline: "echo 'hello'"},
			},
			shouldError: true,
			errorMsg:    "must not define 'download_dir'",
		},
		{
			name: "Invalid - has output schema",
			step: internal.Step{
				ID:               "invalid_step",
				OutputSchemaFile: "/schema.json",
				Command:          &internal.CommandBlock{Inline: "echo 'hello'"},
			},
			shouldError: true,
			errorMsg:    "must not define 'output_schema'",
		},
		{
			name: "Invalid - has HTTP call",
			step: internal.Step{
				ID:      "invalid_step",
				Call:    &internal.HTTPCall{Url: "https://example.com"},
				Command: &internal.CommandBlock{Inline: "echo 'hello'"},
			},
			shouldError: true,
			errorMsg:    "must not define 'call'",
		},
		{
			name: "Invalid - has allowed domains",
			step: internal.Step{
				ID:             "invalid_step",
				AllowedDomains: []string{"example.com"},
				Command:        &internal.CommandBlock{Inline: "echo 'hello'"},
			},
			shouldError: true,
			errorMsg:    "must not define 'allowed_domains'",
		},
		{
			name: "Invalid - has max steps",
			step: internal.Step{
				ID:       "invalid_step",
				MaxSteps: func() *int { i := 5; return &i }(),
				Command:  &internal.CommandBlock{Inline: "echo 'hello'"},
			},
			shouldError: true,
			errorMsg:    "must not define 'max_steps'",
		},
		{
			name: "Invalid - has max failures",
			step: internal.Step{
				ID:          "invalid_step",
				MaxFailures: func() *int { i := 3; return &i }(),
				Command:     &internal.CommandBlock{Inline: "echo 'hello'"},
			},
			shouldError: true,
			errorMsg:    "must not define 'max_failures'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zerolog.New(io.Discard)
			ctx := internal.ExecutionContext{
				Step:   tt.step,
				Logger: &logger,
			}

			sh := &ShellHandler{StepCtx: ctx}
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
		command        *internal.CommandBlock
		expectedOutput string
		shouldError    bool
	}{
		{
			name: "Inline echo command",
			command: &internal.CommandBlock{
				Inline: "echo 'hello'",
			},
			expectedOutput: "hello",
			shouldError:    false,
		},
		{
			name: "Script file",
			command: &internal.CommandBlock{
				Path: scriptPath,
			},
			expectedOutput: "hello from script",
			shouldError:    false,
		},
		{
			name: "JSON output",
			command: &internal.CommandBlock{
				Inline: "echo '{\"key\":\"value\",\"number\":42}'",
			},
			expectedOutput: "",  // Not checking exact output, will verify it's JSON
			shouldError:    false,
		},
		{
			name: "Command with error",
			command: &internal.CommandBlock{
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
			
			// Create a shell handler with the command
			step := internal.Step{
				ID:      "test_shell_step",
				Command: tt.command,
			}
			
			ctx := internal.ExecutionContext{
				Step:        step,
				Logger:      &logger,
				WorkflowDir: tempDir,
			}
			
			sh := &ShellHandler{StepCtx: ctx}
			
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

// TestShellHandler_Secrets is a placeholder test for secret redaction
// In practice, full test coverage would require refactoring to allow
// dependency injection for better testing
func TestShellHandler_Secrets(t *testing.T) {
	t.Skip("This test requires a redaction mechanism to be implemented")
	
	/*
	In a proper implementation with redaction, we would:
	
	1. Set up a mock command executor that outputs known secrets
	2. Configure a redaction system that knows to redact those secrets
	3. Run the command and verify the secrets are not in the logs
	4. Check that the raw output still contains the secrets (since redaction should only affect logs)
	
	This would likely involve:
	- Refactoring the ShellHandler to accept a CommandRunner interface
	- Creating a redaction system that hooks into the logger
	- Implementing a mock CommandRunner for testing
	*/
}