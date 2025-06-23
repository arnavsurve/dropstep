package main

import (
	"os"

	"github.com/alecthomas/kong"
	"github.com/arnavsurve/dropstep/cmd/cli"
	"github.com/rs/zerolog/log"
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
		log.Error().Err(err).Msg("Command execution failed")
		os.Exit(1)
	}
}
