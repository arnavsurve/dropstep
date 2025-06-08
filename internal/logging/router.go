package logging

import "github.com/rs/zerolog"

type LogSink interface {
	Write(level zerolog.Level, event map[string]any)
}

type LoggerRouter struct {
	Sinks []LogSink
}

func (r *LoggerRouter) Log(level zerolog.Level, event map[string]any) {
	for _, sink := range r.Sinks {
		sink.Write(level, event)
	}
}