package log_test

import (
	"bytes"
	"testing"

	"github.com/arnavsurve/dropstep/pkg/log"
	"github.com/rs/zerolog"
)

func TestAdapter(t *testing.T) {
	out := &bytes.Buffer{}
	zl   := zerolog.New(out)
	log  := log.NewZerologAdapter(zl)

	log.Info().
		Str("unit", "test").
		Int("n", 1).
		Msg("hello")

	if !bytes.Contains(out.Bytes(), []byte(`"unit":"test"`)) {
		t.Fatalf("field missing")
	}
}
