package steprunner

import "github.com/arnavsurve/dropstep/pkg/types"

type StepRunner interface {
	Validate() error
	Run() (*types.StepResult, error)
}
