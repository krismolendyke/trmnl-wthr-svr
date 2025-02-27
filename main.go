package main

import (
	"log/slog"
	"os"

	"github.com/alecthomas/kong"
)

func main() {
	cli := CLI{Globals: Globals{}}
	ctx := kong.Parse(&cli,
		kong.Name("trmnl-wthr-svr"),
		kong.Description("Ambient Weather webhook server for TRMNL displays"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{Compact: true}),
		kong.DefaultEnvars("TRMNL_WTHR_SVR"),
	)

	logLevel := slog.LevelInfo
	if cli.Debug {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     logLevel,
	}))
	slog.SetDefault(logger)

	if err := ctx.Run(&cli.Globals); err != nil {
		slog.Error("error", slog.String("err", err.Error()))
		os.Exit(1)
	}
}
