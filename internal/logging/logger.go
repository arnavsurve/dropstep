package logging

import (
	"time"

	"github.com/rs/zerolog"
)

var GlobalLogger zerolog.Logger

func ConfigureGlobalLogger(router *LoggerRouter, wfId, wfRunID string) {
	zerolog.TimeFieldFormat = time.RFC3339
	writer := &RouterWriter{
		Router: router,
	}

	GlobalLogger = zerolog.New(writer).
		With().
		Timestamp().
		Str("workflow_name", wfId).
		Str("workflow_run_id", wfRunID).
		Logger()
}

func ScopedLogger(stepId, stepType string) zerolog.Logger {
	return GlobalLogger.With().
		Str("step_id", stepId).
		Str("type", stepType).
		Timestamp().
		Logger()
}