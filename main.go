package main

import (
	"os"

	"github.com/alecthomas/kong"
	"github.com/arnavsurve/dropstep/cmd/cli"
)

var CLI struct {
	Run  cli.RunCmd  `cmd:"" help:"Run a Dropstep workflow."`
	Lint cli.LintCmd `cmd:"" help:"Validate the Dropstep workflow file syntax."`
}

func main() {
	ctx := kong.Parse(&CLI,
		kong.Name("dropstep"),
		kong.Description("Dropstep: Declarative agentic automation."),
	)

	if err := ctx.Run(); err != nil {
		// Error is already logged by commands internally
		// Logging here causes the final error log to be omitted from the logging sink implementation
		os.Exit(1)
	}
}
