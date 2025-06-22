package log

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/arnavsurve/dropstep/pkg/security"
	"github.com/arnavsurve/dropstep/pkg/types"
	"github.com/rs/zerolog"
)

// LogEvent represents a log event that will be written to sinks
type LogEvent struct {
	Level     types.Level
	Message   string
	Fields    map[string]any
	RawError  error
	Timestamp time.Time
}

// Sink defines the interface for log output destinations
type Sink interface {
	Write(event *LogEvent) error
	io.Closer
}

// Router routes log events to multiple sinks
type Router struct {
	sinks    []Sink
	redactor *security.Redactor
}

func NewRouter(sinks ...Sink) *Router {
	return &Router{sinks: sinks}
}

func (r *Router) Write(p []byte) (n int, err error) {
	var zerologOutput map[string]any
	if err := json.Unmarshal(p, &zerologOutput); err != nil {
		fmt.Fprintf(os.Stderr, "Router: Error unmarshaling log line: %v, data: %s\n", err, string(p))
		return len(p), nil
	}

	evt := &LogEvent{
		Fields: make(map[string]any),
	}

	if lvlStr, ok := zerologOutput[zerolog.LevelFieldName].(string); ok {
		zlLevel, err := zerolog.ParseLevel(lvlStr)
		if err == nil {
			evt.Level = ConvertZerologLevel(zlLevel)
		}
	}
	if msg, ok := zerologOutput[zerolog.MessageFieldName].(string); ok {
		evt.Message = msg
	}
	if tsStr, ok := zerologOutput[zerolog.TimestampFieldName].(string); ok {
		evt.Timestamp, _ = time.Parse(time.RFC3339Nano, tsStr)
	} else {
		evt.Timestamp = time.Now()
	}
	if errField, ok := zerologOutput[zerolog.ErrorFieldName].(string); ok {
		evt.Fields[zerolog.ErrorFieldName] = errField
	}

	reservedFields := map[string]struct{}{
		zerolog.LevelFieldName:     {},
		zerolog.MessageFieldName:   {},
		zerolog.TimestampFieldName: {},
		zerolog.ErrorFieldName:     {},
	}
	for k, v := range zerologOutput {
		if _, isReserved := reservedFields[k]; !isReserved {
			evt.Fields[k] = v
		}
	}

	if r.redactor != nil {
		evt.Message = r.redactor.Redact(evt.Message)
		for k, v := range evt.Fields {
			if strVal, ok := v.(string); ok {
				evt.Fields[k] = r.redactor.Redact(strVal)
			}
		}
		for _, v := range evt.Fields {
			if m, ok := v.(map[string]any); ok {
				for kk, vv := range m {
					if strVal, ok := vv.(string); ok {
						m[kk] = r.redactor.Redact(strVal)
					}
				}
			}
			if s, ok := v.([]any); ok {
				for i, vv := range s {
					if strVal, ok := vv.(string); ok {
						s[i] = r.redactor.Redact(strVal)
					}
				}
			}
		}
	}

	for _, sink := range r.sinks {
		// TODO: check evt.Level against sink's minLevel if sinks have individual levels
		if err := sink.Write(evt); err != nil {
			fmt.Fprintf(os.Stderr, "Router: Error writing to sink: %v\n", err)
		}
	}

	return len(p), nil
}

func ConvertZerologLevel(zl zerolog.Level) types.Level {
	switch zl {
	case zerolog.DebugLevel:
		return types.DebugLevel
	case zerolog.InfoLevel:
		return types.InfoLevel
	case zerolog.WarnLevel:
		return types.WarnLevel
	case zerolog.ErrorLevel:
		return types.ErrorLevel
	case zerolog.FatalLevel:
		return types.FatalLevel
	default:
		return types.InfoLevel
	}
}

func (r *Router) AddSink(sink Sink) {
	r.sinks = append(r.sinks, sink)
}

func (r *Router) Close() error {
	var firstErr error
	for _, sink := range r.sinks {
		if err := sink.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
