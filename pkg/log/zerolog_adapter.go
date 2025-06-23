package log

import (
	"github.com/arnavsurve/dropstep/pkg/types"
	"github.com/rs/zerolog"
)

type ZerologAdapter struct {
	logger zerolog.Logger
}

func NewZerologAdapter(logger zerolog.Logger) *ZerologAdapter {
	return &ZerologAdapter{logger: logger}
}

func (z *ZerologAdapter) Debug() types.Event {
	return &ZerologEvent{event: z.logger.Debug()}
}

func (z *ZerologAdapter) Info() types.Event {
	return &ZerologEvent{event: z.logger.Info()}
}

func (z *ZerologAdapter) Warn() types.Event {
	return &ZerologEvent{event: z.logger.Warn()}
}

func (z *ZerologAdapter) Error() types.Event {
	return &ZerologEvent{event: z.logger.Error()}
}

func (z *ZerologAdapter) Fatal() types.Event {
	return &ZerologEvent{event: z.logger.Fatal()}
}

func (z *ZerologAdapter) With() types.Context {
	return &ZerologContext{ctx: z.logger.With()}
}

// Implements the Event interface
type ZerologEvent struct {
	event *zerolog.Event
}

func (e *ZerologEvent) Msg(msg string) {
	e.event.Msg(msg)
}

func (e *ZerologEvent) Msgf(format string, v ...any) {
	e.event.Msgf(format, v...)
}

func (e *ZerologEvent) Err(err error) types.Event {
	e.event = e.event.Err(err)
	return e
}

func (e *ZerologEvent) Interface(key string, value any) types.Event {
	e.event = e.event.Interface(key, value)
	return e
}

func (e *ZerologEvent) Str(key, value string) types.Event {
	e.event = e.event.Str(key, value)
	return e
}

func (e *ZerologEvent) Int(key string, value int) types.Event {
	e.event = e.event.Int(key, value)
	return e
}

// Implements the Context interface
type ZerologContext struct {
	ctx zerolog.Context
}

func (c *ZerologContext) Str(key, value string) types.Context {
	return &ZerologContext{ctx: c.ctx.Str(key, value)}
}

func (c *ZerologContext) Int(key string, value int) types.Context {
	return &ZerologContext{ctx: c.ctx.Int(key, value)}
}

func (c *ZerologContext) Interface(key string, value any) types.Context {
	return &ZerologContext{ctx: c.ctx.Interface(key, value)}
}

func (c *ZerologContext) Timestamp() types.Context {
	return &ZerologContext{ctx: c.ctx.Timestamp()}
}

func (c *ZerologContext) Logger() types.Logger {
	return &ZerologAdapter{logger: c.ctx.Logger()}
}
