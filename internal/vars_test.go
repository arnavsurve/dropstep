package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindValueInContext(t *testing.T) {
	globals := VarContext{"url": "https://example.com"}
	results := StepResultsContext{
		"step1": {
			Output: map[string]interface{}{
				"user": map[string]interface{}{"id": 123},
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
		expected interface{}
		found    bool
	}{
		{"url", "https://example.com", true},
		{"steps.step1.output.user.id", 123, true},
		{"steps.step1.output.name", "Grace", true},
		{"steps.step1.output_file", "/path/to/step1.txt", true},
		{"steps.step2.output", "raw string output", true},
		{"steps.step1.output", map[string]interface{}{"user": map[string]interface{}{"id": 123}, "name": "Grace"}, true},
		{"steps.step2.output.key", nil, false}, // Cannot access key on string
		{"nonexistent", nil, false},
		{"steps.nonexistent.output", nil, false},
		{"steps.step1.output.nonexistent", nil, false},
	}

	for _, tc := range testCases {
		t.Run(tc.key, func(t *testing.T) {
			val, found := findValueInContext(tc.key, globals, results)
			assert.Equal(t, tc.found, found)
			if tc.found {
				assert.Equal(t, tc.expected, val)
			}
		})
	}
}

func TestResolveStepVariables(t *testing.T) {
	globals := VarContext{"domain": "example.com"}
	results := StepResultsContext{
		"prev_step": {
			Output:     "user123",
			OutputFile: "/data/prev_output.txt",
		},
	}

	step := &Step{
		ID:     "current_step",
		Prompt: "Process user {{ steps.prev_step.output }} from {{ domain }}.",
		Command: &CommandBlock{
			Inline: "cat {{ steps.prev_step.output_file }}",
		},
	}

	resolved, err := ResolveStepVariables(step, globals, results)
	require.NoError(t, err)

	assert.Equal(t, "Process user user123 from example.com.", resolved.Prompt)
	assert.Equal(t, "cat /data/prev_output.txt", resolved.Command.Inline)
}

func TestResolveStringWithContext_UndefinedVar(t *testing.T) {
	input := "Hello {{ undefined_var }}"
	_, err := resolveStringWithContext(input, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "undefined variable: undefined_var")
}
