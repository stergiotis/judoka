package main

import (
	"os"
	"slices"

	"github.com/rs/zerolog/log"
	"github.com/stergiotis/boxer/public/dev"
	"github.com/stergiotis/boxer/public/observability/logging"
	"github.com/stergiotis/boxer/public/observability/ph"
	"github.com/stergiotis/boxer/public/observability/profiling"
	"github.com/stergiotis/boxer/public/observability/vcs"
	"github.com/stergiotis/judoka/cli/plainTextNotebooks"
	"github.com/urfave/cli/v2"
)

func main() {
	exitCode := 0
	logging.SetupZeroLog()
	defer ph.PanicHandler(2, nil, nil)
	app := cli.App{
		Name:                 "judoka",
		Copyright:            "Copyright © 2025 Panos Stergiotis",
		HelpName:             "",
		Usage:                "",
		UsageText:            "",
		ArgsUsage:            "",
		Version:              vcs.BuildVersionInfo(),
		Description:          "",
		DefaultCommand:       "",
		EnableBashCompletion: false,
		Flags: slices.Concat(
			logging.LoggingFlags,
			profiling.ProfilingFlags,
			dev.DebuggerFlags,
			dev.IoOverrideFlags),
		Commands: []*cli.Command{
			plainTextNotebooks.NewCommand(),
		},
		After: func(context *cli.Context) error {
			profiling.ProfilingHandleExit(context)
			return nil
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		exitCode = 1
		log.Error().Stack().Err(err).Msg("an error occurred")
	}
	os.Exit(exitCode)
}
