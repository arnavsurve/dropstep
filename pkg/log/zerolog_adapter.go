package log

import (
    "github.com/arnavsurve/dropstep/pkg/core"
    "github.com/rs/zerolog"
)

type ZerologAdapter struct {
    logger zerolog.Logger
}

func NewZerologAdapter(logger zerolog.Logger) *ZerologAdapter {
    return &ZerologAdapter{logger: logger}
}

func (z *ZerologAdapter) Debug() core.Event {
    return &ZerologEvent{event: z.logger.Debug()}
}

func (z *ZerologAdapter) Info() core.Event {
    return &ZerologEvent{event: z.logger.Info()}
}

func (z *ZerologAdapter) Warn() core.Event {
    return &ZerologEvent{event: z.logger.Warn()}
}

func (z *ZerologAdapter) Error() core.Event {
    return &ZerologEvent{event: z.logger.Error()}
}

func (z *ZerologAdapter) Fatal() core.Event {
    return &ZerologEvent{event: z.logger.Fatal()}
}

func (z *ZerologAdapter) With() core.Context {
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

func (e *ZerologEvent) Err(err error) core.Event {
    e.event = e.event.Err(err)
    return e
}

func (e *ZerologEvent) Interface(key string, value any) core.Event {
    e.event = e.event.Interface(key, value)
    return e
}

func (e *ZerologEvent) Str(key, value string) core.Event {
    e.event = e.event.Str(key, value)
    return e
}

func (e *ZerologEvent) Int(key string, value int) core.Event {
    e.event = e.event.Int(key, value)
    return e
}

// Implements the Context interface
type ZerologContext struct {
    ctx zerolog.Context
}

func (c *ZerologContext) Str(key, value string) core.Context {
    return &ZerologContext{ctx: c.ctx.Str(key, value)}
}

func (c *ZerologContext) Int(key string, value int) core.Context {
    return &ZerologContext{ctx: c.ctx.Int(key, value)}
}

func (c *ZerologContext) Interface(key string, value any) core.Context {
    return &ZerologContext{ctx: c.ctx.Interface(key, value)}
}

func (c *ZerologContext) Timestamp() core.Context {
    return &ZerologContext{ctx: c.ctx.Timestamp()}
}

func (c *ZerologContext) Logger() core.Logger {
    return &ZerologAdapter{logger: c.ctx.Logger()}
}
