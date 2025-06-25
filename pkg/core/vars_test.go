package core_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arnavsurve/dropstep/pkg/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveVarfile(t *testing.T) {
	// Create a temporary varfile for testing
	tempDir := t.TempDir()
	varfilePath := filepath.Join(tempDir, "test_vars.yml")

	// Set environment variable for testing
	t.Setenv("TEST_ENV_VAR", "env_value")

	// Create test varfile content - only test env and plain vars for now
	varfileContent := `
plain_var: plain_value
env_var: "{{ env.TEST_ENV_VAR }}"
empty_env_var: "{{ env.NONEXISTENT_VAR }}"
`

	require.NoError(t, os.WriteFile(varfilePath, []byte(varfileContent), 0644))

	// Test resolving the varfile
	vars, err := core.ResolveVarfile(varfilePath)
	require.NoError(t, err)

	// Verify resolved values
	assert.Equal(t, "plain_value", vars["plain_var"])
	assert.Equal(t, "env_value", vars["env_var"])
	assert.Equal(t, "", vars["empty_env_var"])

	// Test error cases
	_, err = core.ResolveVarfile("nonexistent_file.yml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reading varfile")

	// Test invalid YAML
	invalidPath := filepath.Join(tempDir, "invalid.yml")
	require.NoError(t, os.WriteFile(invalidPath, []byte("invalid: yaml: ]:"), 0644))
	_, err = core.ResolveVarfile(invalidPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing varfile YAML")
}

func TestFindValueInContext(t *testing.T) {
	globals := core.VarContext{"url": "https://example.com"}
	results := core.StepResultsContext{
		"step1": {
			Output: map[string]any{
				"user": map[string]any{"id": 123},
				"name": "Grace",
			},
			OutputFile: "/path/to/step1.txt",
		},
		"step2": {
			Output: "raw string output",
		},
	}

	testCases := []struct {
		key      string
		expected any
		found    bool
	}{
		{"url", "https://example.com", true},
		{"steps.step1.output.user.id", 123, true},
		{"steps.step1.output.name", "Grace", true},
		{"steps.step1.output_file", "/path/to/step1.txt", true},
		{"steps.step2.output", "raw string output", true},
		{"steps.step1.output", map[string]any{"user": map[string]any{"id": 123}, "name": "Grace"}, true},
		{"steps.step2.output.key", nil, false}, // Cannot access key on string
		{"nonexistent", nil, false},
		{"steps.nonexistent.output", nil, false},
		{"steps.step1.output.nonexistent", nil, false},
	}

	for _, tc := range testCases {
		t.Run(tc.key, func(t *testing.T) {
			val, found := core.FindValueInContext(tc.key, globals, results)
			assert.Equal(t, tc.found, found)
			if tc.found {
				assert.Equal(t, tc.expected, val)
			}
		})
	}
}

func TestResolveStepVariables(t *testing.T) {
	globals := core.VarContext{"domain": "example.com"}
	results := core.StepResultsContext{
		"prev_step": {
			Output:     "user123",
			OutputFile: "/data/prev_output.txt",
		},
	}

	step := &core.Step{
		ID: "current_step",
		BrowserConfig: core.BrowserConfig{
			Prompt: "Process user {{ steps.prev_step.output }} from {{ domain }}.",
		},
		Command: &core.CommandBlock{
			Inline: "cat {{ steps.prev_step.output_file }}",
		},
	}

	resolved, err := core.ResolveStepVariables(step, globals, results)
	require.NoError(t, err)

	assert.Equal(t, "Process user user123 from example.com.", resolved.BrowserConfig.Prompt)
	assert.Equal(t, "cat /data/prev_output.txt", resolved.Command.Inline)
}

func TestResolveStringWithContext_UndefinedVar(t *testing.T) {
	input := "Hello {{ undefined_var }}"
	_, err := core.ResolveStringWithContext(input, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "undefined variable: undefined_var")
}

func TestResolveStringWithContext_Json(t *testing.T) {
	globals := core.VarContext{"simple": "value"}
	results := core.StepResultsContext{
		"json_step": {
			Output: map[string]any{
				"nested": map[string]any{
					"values": []string{"one", "two"},
				},
				"id": 123,
			},
		},
	}

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple JSON variable",
			input:    "JSON: {{ steps.json_step.output.json }}",
			expected: `JSON: {"id":123,"nested":{"values":["one","two"]}}`,
		},
		{
			name:     "Nested JSON variable",
			input:    "Nested: {{ steps.json_step.output.nested.json }}",
			expected: `Nested: {"values":["one","two"]}`,
		},
		{
			name:     "Primitive value as JSON",
			input:    "ID: {{ steps.json_step.output.id.json }}",
			expected: `ID: 123`,
		},
		{
			name:     "Global variable as JSON",
			input:    "Global: {{ simple.json }}",
			expected: `Global: "value"`,
		},
		{
			name:     "Multiple JSON variables",
			input:    "Data: {{ steps.json_step.output.id.json }} {{ steps.json_step.output.nested.json }}",
			expected: `Data: 123 {"values":["one","two"]}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.ResolveStringWithContext(tc.input, globals, results)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetNestedValue(t *testing.T) {
	testData := map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": "value",
				"d": 123,
			},
		},
		"x": "top-level",
		"stringMap": map[string]string{
			"key": "value",
		},
	}

	testCases := []struct {
		name     string
		data     interface{}
		path     []string
		expected interface{}
		found    bool
	}{
		{
			name:     "Empty path returns the data",
			data:     testData,
			path:     []string{},
			expected: testData,
			found:    true,
		},
		{
			name:     "Top level value",
			data:     testData,
			path:     []string{"x"},
			expected: "top-level",
			found:    true,
		},
		{
			name:     "Nested value",
			data:     testData,
			path:     []string{"a", "b", "c"},
			expected: "value",
			found:    true,
		},
		{
			name:     "Numeric value",
			data:     testData,
			path:     []string{"a", "b", "d"},
			expected: 123,
			found:    true,
		},
		{
			name:     "String map value",
			data:     testData,
			path:     []string{"stringMap", "key"},
			expected: "value",
			found:    true,
		},
		{
			name:     "Non-existent path",
			data:     testData,
			path:     []string{"nonexistent"},
			expected: nil,
			found:    false,
		},
		{
			name:     "Partial path",
			data:     testData,
			path:     []string{"a", "nonexistent"},
			expected: nil,
			found:    false,
		},
		{
			name:     "Path on non-map type",
			data:     "string",
			path:     []string{"any"},
			expected: nil,
			found:    false,
		},
		{
			name:     "Nil data",
			data:     nil,
			path:     []string{"any"},
			expected: nil,
			found:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			val, found := core.GetNestedValue(tc.data, tc.path)
			assert.Equal(t, tc.found, found)
			if tc.found {
				assert.Equal(t, tc.expected, val)
			}
		})
	}
}

func TestResolveValue(t *testing.T) {
	globals := core.VarContext{"key": "value"}
	results := core.StepResultsContext{}

	resolver := func(s string) (string, error) {
		return core.ResolveStringWithContext(s, globals, results)
	}

	testCases := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{
			name:     "String value",
			input:    "plain {{ key }}",
			expected: "plain value",
		},
		{
			name:     "Integer value",
			input:    123,
			expected: 123,
		},
		{
			name:     "Boolean value",
			input:    true,
			expected: true,
		},
		{
			name: "Map value",
			input: map[string]interface{}{
				"str": "Hello {{ key }}",
				"num": 456,
			},
			expected: map[string]interface{}{
				"str": "Hello value",
				"num": 456,
			},
		},
		{
			name:     "Slice value",
			input:    []interface{}{"{{ key }}", 789, true},
			expected: []interface{}{"value", 789, true},
		},
		{
			name: "Nested structures",
			input: map[string]interface{}{
				"outer": map[string]interface{}{
					"inner": "nested {{ key }}",
					"list":  []interface{}{"item {{ key }}", 123},
				},
			},
			expected: map[string]interface{}{
				"outer": map[string]interface{}{
					"inner": "nested value",
					"list":  []interface{}{"item value", 123},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.ResolveValue(tc.input, resolver, globals, results)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestResolveProviderVariables(t *testing.T) {
	globals := core.VarContext{
		"api_key_var": "test-api-key",
	}

	testCases := []struct {
		name     string
		provider *core.ProviderConfig
		expected *core.ProviderConfig
		hasError bool
	}{
		{
			name: "Simple provider with template",
			provider: &core.ProviderConfig{
				Name:   "test-provider",
				APIKey: "{{ api_key_var }}",
			},
			expected: &core.ProviderConfig{
				Name:   "test-provider",
				APIKey: "test-api-key",
			},
			hasError: false,
		},
		{
			name: "Provider with static key",
			provider: &core.ProviderConfig{
				Name:   "static-provider",
				APIKey: "static-key",
			},
			expected: &core.ProviderConfig{
				Name:   "static-provider",
				APIKey: "static-key",
			},
			hasError: false,
		},
		{
			name: "Provider with undefined variable",
			provider: &core.ProviderConfig{
				Name:   "error-provider",
				APIKey: "{{ undefined_var }}",
			},
			expected: nil,
			hasError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := core.ResolveProviderVariables(tc.provider, globals)

			if tc.hasError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expected.Name, result.Name)
			assert.Equal(t, tc.expected.APIKey, result.APIKey)
		})
	}
}

func TestResolveStepVariables_HttpCall(t *testing.T) {
	globals := core.VarContext{"base_url": "https://api.example.com"}
	results := core.StepResultsContext{
		"auth_step": {
			Output: map[string]any{
				"token": "abc123",
				"user":  map[string]any{"id": 456},
			},
		},
	}

	step := &core.Step{
		ID:   "http_step",
		Uses: "http",
		Call: &core.HTTPCall{
			Method: "POST",
			Url:    "{{ base_url }}/users",
			Headers: map[string]string{
				"Authorization": "Bearer {{ steps.auth_step.output.token }}",
				"Content-Type":  "application/json",
			},
			Body: map[string]any{
				"userId": "{{ steps.auth_step.output.user.id }}",
				"action": "update",
			},
		},
	}

	resolved, err := core.ResolveStepVariables(step, globals, results)
	require.NoError(t, err)

	assert.Equal(t, "https://api.example.com/users", resolved.Call.Url)
	assert.Equal(t, "Bearer abc123", resolved.Call.Headers["Authorization"])
	assert.Equal(t, "application/json", resolved.Call.Headers["Content-Type"])

	assert.Equal(t, "456", resolved.Call.Body["userId"])
	assert.Equal(t, "update", resolved.Call.Body["action"])
}
