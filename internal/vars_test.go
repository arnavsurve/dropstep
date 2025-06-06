package internal

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a temporary varfile
func createTempVarFile(t *testing.T, content string) string {
	t.Helper()
	tmpFile, err := os.CreateTemp(t.TempDir(), "test-vars-*.yml")
	require.NoError(t, err)
	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)
	err = tmpFile.Close()
	require.NoError(t, err)
	return tmpFile.Name()
}

func TestResolveLiteralVars(t *testing.T) {
	yml := `foo: "bar"
number: "123"`
	tmp := createTempVarFile(t, yml)
	ctx, err := ResolveVarfile(tmp)
	require.NoError(t, err)
	assert.Equal(t, "bar", ctx["foo"])
	assert.Equal(t, "123", ctx["number"])
}

func TestResolveStringVariables(t *testing.T) {
	ctx := VarContext{"name": "Arnav", "lang": "Go"}
	out, err := resolveStringVariables("Hello {{name}}, you're coding in {{lang}}", ctx, "greeting")
	require.NoError(t, err)
	assert.Equal(t, "Hello Arnav, you're coding in Go", out)
}

func TestNewStepVariableResolver(t *testing.T) {
	t.Run("resolves upload file paths and prompt context", func(t *testing.T) {
		globalCtx := VarContext{
			"dir": "/tmp/files",
		}

		uploadFiles := []FileToUpload{
			{Name: "doc1", Path: "{{dir}}/report.txt"},
			{Name: "img", Path: "{{dir}}/img.png"},
			{Name: "loose", Path: ""},
		}

		resolver, err := newStepVariableResolver(globalCtx, "test-step", uploadFiles)
		require.NoError(t, err)

		resolved := resolver.GetResolvedUploadFiles()

		assert.Equal(t, "/tmp/files/report.txt", resolved[0].Path)
		assert.Equal(t, "/tmp/files/img.png", resolved[1].Path)
		assert.Equal(t, "", resolved[2].Path)

		promptCtx := resolver.promptContext
		assert.Equal(t, "/tmp/files", promptCtx["dir"])
		assert.Equal(t, "/tmp/files/report.txt", promptCtx["doc1"])
		assert.Equal(t, "/tmp/files/img.png", promptCtx["img"])
		assert.Equal(t, "", promptCtx["loose"])
	})

	t.Run("warns if upload file name overrides global var", func(t *testing.T) {
		globalCtx := VarContext{
			"foo": "should-be-overridden",
		}

		uploadFiles := []FileToUpload{
			{Name: "foo", Path: "/x/y.txt"},
		}

		resolver, err := newStepVariableResolver(globalCtx, "conflict-step", uploadFiles)
		require.NoError(t, err)

		promptCtx := resolver.promptContext
		// The upload file "foo" should override globalCtx["foo"] in prompt context
		assert.Equal(t, "/x/y.txt", promptCtx["foo"])
	})

	t.Run("returns error on unresolved variable in path", func(t *testing.T) {
		globalCtx := VarContext{}

		uploadFiles := []FileToUpload{
			{Name: "bad", Path: "{{missing}}/file.txt"},
		}

		_, err := newStepVariableResolver(globalCtx, "error-step", uploadFiles)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "undefined variable(s) [missing]")
	})
}

func TestInjectVarsIntoWorkflow(t *testing.T) {
	globalCtx := VarContext{
		"dir": "/tmp",
		"file": "test.txt",
	}

	wf := &Workflow{
		Steps: []Step{
			{
				ID:    "step1",
				Uses:  "browser",
				Prompt: "Open {{file}}",
				Run:    "cat {{dir}}/{{file}}",
				TargetDownloadDir: "{{dir}}/downloads",
				UploadFiles: []FileToUpload{
					{Name: "fileVar", Path: "{{dir}}/to_upload.txt"},
				},
			},
		},
	}

	updated, err := InjectVarsIntoWorkflow(wf, globalCtx)
	require.NoError(t, err)

	step := updated.Steps[0]
	assert.Equal(t, "Open test.txt", step.Prompt)
	assert.Equal(t, "cat /tmp/test.txt", step.Run)
	assert.Equal(t, "/tmp/downloads", step.TargetDownloadDir)
	assert.Equal(t, "/tmp/to_upload.txt", step.UploadFiles[0].Path)
	assert.Equal(t, "fileVar", step.UploadFiles[0].Name)
}

func TestStepValidate(t *testing.T) {
	tests := []struct {
		name      string
		step      Step
		expectErr bool
	}{
		{
			name: "valid browser step",
			step: Step{ID: "b1", Uses: "browser", Prompt: "Do something"},
		},
		{
			name: "browser step with forbidden run",
			step: Step{ID: "b2", Uses: "browser", Prompt: "Do something", Run: "echo"},
			expectErr: true,
		},
		{
			name: "valid shell step",
			step: Step{ID: "s1", Uses: "shell", Run: "ls -la"},
		},
		{
			name: "shell step missing run",
			step: Step{ID: "s2", Uses: "shell"},
			expectErr: true,
		},
		{
			name: "valid api step",
			step: Step{
				ID: "a1", Uses: "api", 
				Call: &ApiCall{Method: "POST", Url: "http://example.com"},
			},
		},
		{
			name: "api step missing call",
			step: Step{ID: "a2", Uses: "api"},
			expectErr: true,
		},
		{
			name: "unknown uses",
			step: Step{ID: "x1", Uses: "funky"},
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.step.Validate()
			if tc.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}