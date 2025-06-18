package security

import (
	"strings"

	"github.com/arnavsurve/dropstep/internal"
)

type Redactor struct {
	secrets []string
}

func NewRedactor(inputs []internal.Input, varCtx internal.VarContext) *Redactor {
	var secretValues []string
	for _, input := range inputs {
		if input.Secret {
			if val, ok := varCtx[input.Name]; ok && val != "" {
				secretValues = append(secretValues, val)
			}
		}
	}
	return &Redactor{
		secrets: secretValues,
	}
}

func (r *Redactor) Redact(s string) string {
	if r == nil || len(r.secrets) == 0 {
		return s
	}
	for _, secret := range r.secrets {
		s = strings.ReplaceAll(s, secret, "********")
	}
	return s
}