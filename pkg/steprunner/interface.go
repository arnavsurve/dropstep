package steprunner

import "github.com/arnavsurve/dropstep/pkg/core"

type StepRunner interface {
	Validate() error
	Run() (*core.StepResult, error)
}