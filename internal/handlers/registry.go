package handlers

import (
	"fmt"

	"github.com/arnavsurve/dropstep/internal"
)

type HandlerFactory func(ctx internal.ExecutionContext) (Handler, error)

// registry stores each type of handler's factory function. GetHandler calls the appropriate Handler
// factory function to yield a new instance of that Handler
var registry = map[string]HandlerFactory{}

// This is called in each handler's init() function to register its factory function with the registry.
// This allows GetHandler to return an instance of the appropriate Handler, using the registry to resolve
// the handler's factory.
func RegisterHandlerFactory(stepType string, factory HandlerFactory) {
	registry[stepType] = factory
}

// GetHandler returns an instance of the appropriate Handler based on the step's 'uses' field,
// calling the corresponding handler's factory function from the registry.
func GetHandler(ctx internal.ExecutionContext) (Handler, error) {
	stepType := ctx.Step.Uses
	factory, ok := registry[stepType]
	if !ok {
		return nil, fmt.Errorf("no handler registered for type: %s", stepType)
	}

	return factory(ctx)
}
