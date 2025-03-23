package main

import (
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/alecthomas/kong"
	"github.com/lrosenman/ambient"
)

func (c *ServerCmd) Run(ctx *kong.Context) error {
	ticker := time.NewTicker(c.Interval)
	defer ticker.Stop()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	ambientKey := ambient.NewKey(c.ApplicationKey, c.APIKey)

	slog.Info("running server", slog.Duration("update interval", c.Interval))

	if err := Update(ambientKey, c.Device, c.ResultsLimit, c.WebhookUrl); err != nil {
		if isRateLimited(err) {
			slog.Warn("rate limited on initial request, applying backoff", slog.Duration("backoff", c.Interval))
		} else {
			return err
		}
	}

	for {
		select {
		case <-ticker.C:
			err := Update(ambientKey, c.Device, c.ResultsLimit, c.WebhookUrl)
			if err != nil {
				if isRateLimited(err) {
					// Reset the ticker to implement backoff
					ticker.Reset(c.Interval)
					slog.Warn("rate limited, applying backoff", slog.Duration("backoff", c.Interval))
				} else {
					slog.Error("failed to update", slog.String("err", err.Error()))
				}
			}
		case sig := <-sigCh:
			slog.Info("received signal, shutting down", slog.String("signal", sig.String()))
			return nil
		}
	}
}

// isRateLimited checks if the error is a 429 Too Many Requests error
func isRateLimited(err error) bool {
	return err != nil && strings.Contains(err.Error(), "429")
}
