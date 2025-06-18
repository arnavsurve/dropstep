package logging

import (
	"io"

	"github.com/arnavsurve/dropstep/internal/security"
	"github.com/rs/zerolog"
)

type LogSink interface {
	Write(level zerolog.Level, event map[string]any, redactor *security.Redactor)
	io.Closer
}

type LoggerRouter struct {
	Sinks    []LogSink
	Redactor *security.Redactor
}

func (r *LoggerRouter) Log(level zerolog.Level, event map[string]any) {
	for _, sink := range r.Sinks {
		sink.Write(level, event, r.Redactor)
	}
}

// Close enables graceful shutdown of loggers by iterating through all sinks and calling their Close() method.
func (r *LoggerRouter) Close() error {
	var firstErr error
	for _, sink := range r.Sinks {
		if err := sink.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
