package steprunner

import (
	"bufio"
	"io"

	"github.com/arnavsurve/dropstep/pkg/types"
)

// logBuffer is a shared helper to stream reader content to a structured logger
func logBuffer(r io.Reader, source string, logger types.Logger, logKey string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		logger.Info().
			Str("source", source).
			Str(logKey, scanner.Text()).
			Msg("Script output")
	}
}
