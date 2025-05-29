package handlers

import "fmt"

type HandlerFactory func() Handler

var registry = map[string]HandlerFactory{}

func RegisterHandlerFactory(stepType string, factory HandlerFactory) {
	registry[stepType] = factory
}

func GetHandler(stepType string) (Handler, error) {
	factory, ok := registry[stepType]
	if !ok {
		return nil, fmt.Errorf("no handler registered for type: %s", stepType)
	}

	return factory(), nil
}
