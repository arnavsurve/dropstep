package handlers

import "github.com/arnavsurve/dropstep/internal"

type Handler interface {
	Validate() error
	Run() (*internal.StepResult, error)
}
