package steprunner

import (
	"fmt"

	"github.com/arnavsurve/dropstep/pkg/core"
)

type RunnerFactory func(ctx core.ExecutionContext) (StepRunner, error)

// registry stores each type of step runner's factory function. GetRunner calls the appropriate StepRunner
// factory function to yield a new instance of that StepRunner
var registry = map[string]RunnerFactory{}

// This is called in each step runner's init() function to register its factory function with the registry.
// This allows GetRunner to return an instance of the appropriate StepRunner, using the registry to resolve
// the runner's factory.
func RegisterRunnerFactory(stepType string, factory RunnerFactory) {
	registry[stepType] = factory
}

// GetRunner returns an instance of the appropriate StepRunner based on the step's 'uses' field,
// calling the corresponding runner's factory function from the registry.
func GetRunner(ctx core.ExecutionContext) (StepRunner, error) {
	stepType := ctx.Step.Uses
	factory, ok := registry[stepType]
	if !ok {
		return nil, fmt.Errorf("no runner registered for type: %s", stepType)
	}

	return factory(ctx)
}
