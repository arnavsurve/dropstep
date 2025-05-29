package handlers

import (
	"fmt"
	"github.com/arnavsurve/dropstep/internal"
)

type HandlerFactory func(ctx internal.ExecutionContext) Handler

var registry = map[string]HandlerFactory{}

func RegisterHandlerFactory(stepType string, factory HandlerFactory) {
	registry[stepType] = factory
}

func GetHandler(ctx internal.ExecutionContext) (Handler, error) {
	stepType := ctx.Step.Uses
	factory, ok := registry[stepType]
	if !ok {
		return nil, fmt.Errorf("no handler registered for type: %s", stepType)
	}

	return factory(ctx), nil
}
