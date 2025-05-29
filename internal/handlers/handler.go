package handlers

import "github.com/arnavsurve/dropstep/internal"

type Handler interface {
	Validate(step internal.Step) error

	Run(step internal.Step) error
}
