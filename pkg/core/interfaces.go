package core

// Event defines a single log event.
type Event interface {
	Msg(msg string)
	Msgf(format string, v ...any)
	Err(err error) Event
	Interface(key string, value any) Event
	Str(key, value string) Event
	Int(key string, value int) Event
}

// Context defines a logging context.
type Context interface {
	Str(key, value string) Context
	Int(key string, value int) Context
	Interface(key string, value any) Context
	Timestamp() Context
	Logger() Logger
}

// Logger defines the logging interface.
type Logger interface {
	Debug() Event
	Info() Event
	Warn() Event
	Error() Event
	Fatal() Event
	With() Context
}

// StepRunner defines the interface for a step runner.
type StepRunner interface {
	Validate() error
	Run() (*StepResult, error)
}
