package security

import (
	"testing"

	"github.com/arnavsurve/dropstep/internal"
	"github.com/stretchr/testify/assert"
)

func TestRedactor_Redact(t *testing.T) {
	tests := []struct {
		name    string
		inputs  []internal.Input
		varCtx  internal.VarContext
		input   string
		want    string
		secrets []string
	}{
		{
			name: "exact match",
			inputs: []internal.Input{
				{Name: "password", Secret: true},
			},
			varCtx: internal.VarContext{
				"password": "supersecret",
			},
			input:   "The password is supersecret",
			want:    "The password is ********",
			secrets: []string{"supersecret"},
		},
		{
			name: "multiple occurrences",
			inputs: []internal.Input{
				{Name: "api_key", Secret: true},
			},
			varCtx: internal.VarContext{
				"api_key": "abcdef",
			},
			input:   "API key: abcdef is being used. Backup key: abcdef should be stored.",
			want:    "API key: ******** is being used. Backup key: ******** should be stored.",
			secrets: []string{"abcdef"},
		},
		{
			name: "substring of another word",
			inputs: []internal.Input{
				{Name: "key", Secret: true},
			},
			varCtx: internal.VarContext{
				"key": "key",
			},
			input:   "The keyboard has keys for typing. The key is important.",
			want:    "The ********board has ********s for typing. The ******** is important.",
			secrets: []string{"key"},
		},
		{
			name: "multiple secrets",
			inputs: []internal.Input{
				{Name: "password", Secret: true},
				{Name: "api_key", Secret: true},
			},
			varCtx: internal.VarContext{
				"password": "pass123",
				"api_key":  "key456",
			},
			input:   "Password: pass123, API Key: key456",
			want:    "Password: ********, API Key: ********",
			secrets: []string{"pass123", "key456"},
		},
		{
			name: "empty secret is skipped",
			inputs: []internal.Input{
				{Name: "empty_secret", Secret: true},
				{Name: "valid_secret", Secret: true},
			},
			varCtx: internal.VarContext{
				"empty_secret": "",
				"valid_secret": "valid",
			},
			input:   "Empty: , Valid: valid",
			want:    "Empty: , Valid: ********",
			secrets: []string{"valid"},
		},
		{
			name:    "nil redactor returns original string",
			inputs:  nil,
			varCtx:  nil,
			input:   "Original string",
			want:    "Original string",
			secrets: nil,
		},
		{
			name:    "redactor with no secrets returns original string",
			inputs:  []internal.Input{},
			varCtx:  internal.VarContext{},
			input:   "Original string",
			want:    "Original string",
			secrets: []string{},
		},
		{
			name: "secret not found in input",
			inputs: []internal.Input{
				{Name: "unused", Secret: true},
			},
			varCtx: internal.VarContext{
				"unused": "notused",
			},
			input:   "This string doesn't contain the secret",
			want:    "This string doesn't contain the secret",
			secrets: []string{"notused"},
		},
		{
			name: "overlapping secrets",
			inputs: []internal.Input{
				{Name: "short", Secret: true},
				{Name: "long", Secret: true},
			},
			varCtx: internal.VarContext{
				"short": "secret",
				"long":  "supersecret",
			},
			input:   "This contains supersecret and secret values",
			want:    "This contains ******** and ******** values",
			secrets: []string{"secret", "supersecret"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create redactor directly with secrets for precise testing
			r := &Redactor{
				secrets: tt.secrets,
			}
			
			// Test the direct redaction
			got := r.Redact(tt.input)
			assert.Equal(t, tt.want, got)
			
			// Also test the factory function
			factoryRedactor := NewRedactor(tt.inputs, tt.varCtx)
			if tt.secrets == nil {
				assert.Nil(t, factoryRedactor.secrets)
			} else {
				assert.ElementsMatch(t, tt.secrets, factoryRedactor.secrets)
			}
		})
	}
}

func TestNewRedactor(t *testing.T) {
	tests := []struct {
		name          string
		inputs        []internal.Input
		varCtx        internal.VarContext
		wantSecrets   []string
	}{
		{
			name: "collect secret values",
			inputs: []internal.Input{
				{Name: "password", Secret: true},
				{Name: "username", Secret: false},
				{Name: "api_key", Secret: true},
			},
			varCtx: internal.VarContext{
				"password": "pass123",
				"username": "user1",
				"api_key":  "key456",
			},
			wantSecrets: []string{"pass123", "key456"},
		},
		{
			name: "empty secrets are excluded",
			inputs: []internal.Input{
				{Name: "password", Secret: true},
				{Name: "empty_secret", Secret: true},
			},
			varCtx: internal.VarContext{
				"password":     "pass123",
				"empty_secret": "",
			},
			wantSecrets: []string{"pass123"},
		},
		{
			name: "missing values in context are excluded",
			inputs: []internal.Input{
				{Name: "password", Secret: true},
				{Name: "missing_secret", Secret: true},
			},
			varCtx: internal.VarContext{
				"password": "pass123",
			},
			wantSecrets: []string{"pass123"},
		},
		{
			name:        "empty inputs result in empty secrets",
			inputs:      []internal.Input{},
			varCtx:      internal.VarContext{},
			wantSecrets: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRedactor(tt.inputs, tt.varCtx)
			assert.ElementsMatch(t, tt.wantSecrets, r.secrets)
		})
	}
}