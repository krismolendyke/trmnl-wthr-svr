package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alecthomas/kong"
	"github.com/lrosenman/ambient"
)

func (c *ServeCmd) Run(ctx *kong.Context) error {
	ticker := time.NewTicker(c.Interval)
	defer ticker.Stop()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	ambientKey := ambient.NewKey(c.ApplicationKey, c.APIKey)

	slog.Info("running server", slog.Duration("update interval", c.Interval))

	if err := Update(ambientKey, c.Device, c.ResultsLimit, c.WebhookUrl); err != nil {
		return err
	}
	for {
		select {
		case <-ticker.C:
			if err := Update(ambientKey, c.Device, c.ResultsLimit, c.WebhookUrl); err != nil {
				slog.Error("failed to update", slog.String("err", err.Error()))
			}
		case sig := <-sigCh:
			slog.Info("received signal, shutting down", slog.String("signal", sig.String()))
			return nil
		}
	}
}
