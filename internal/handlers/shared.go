package handlers

import (
	"bufio"
	"io"

	"github.com/rs/zerolog"
)

// logBuffer is a shared helper to stream reader content to a structured logger
func logBuffer(r io.Reader, source string, logger *zerolog.Logger, logKey string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		logger.Info().
			Str("source", source).
			Str(logKey, scanner.Text()).
			Msg("Script output")
	}
}
