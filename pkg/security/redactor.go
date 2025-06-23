package security

import (
	"sort"
	"strings"

	"github.com/arnavsurve/dropstep/pkg/core"
)

type Redactor struct {
	Secrets []string
}

func NewRedactor(inputs []core.Input, varCtx core.VarContext) *Redactor {
	var secretValues []string
	for _, input := range inputs {
		if input.Secret {
			if val, ok := varCtx[input.Name]; ok && val != "" {
				secretValues = append(secretValues, val)
			}
		}
	}
	return &Redactor{
		Secrets: secretValues,
	}
}

func (r *Redactor) Redact(s string) string {
	if r == nil || len(r.Secrets) == 0 {
		return s
	}

	// Sort secrets by length in descending order to handle overlapping secrets properly
	// This ensures longer secrets are replaced before their substrings
	secrets := make([]string, len(r.Secrets))
	copy(secrets, r.Secrets)
	sort.Slice(secrets, func(i, j int) bool {
		return len(secrets[i]) > len(secrets[j])
	})

	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		s = strings.ReplaceAll(s, secret, "********")
	}
	return s
}
