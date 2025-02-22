package main

import (
	"log/slog"

	"github.com/alecthomas/kong"
	"github.com/lrosenman/ambient"
)

type Globals struct {
	Debug bool `short:"D" help:"Enable debug mode"`
}

type CLI struct {
	Globals

	Serve ServeCmd `cmd:"" help:"Run the webhook server"`
}

type ServeCmd struct {
	ApplicationKey string `required:"true" help:"API 'application' key" env:"TRMNL_WTHR_SVR_APP_KEY"`
	APIKey         string `required:"true" help:"API key" env:"TRMNL_WTHR_SVR_API_KEY"`
}

func (c *ServeCmd) Run(ctx *kong.Context) error {
	key := ambient.NewKey(c.ApplicationKey, c.APIKey)
	devices, err := ambient.Device(key)
	if err != nil {
		slog.Error("could not list devices", slog.String("err", err.Error()))
	}
	slog.Debug("Run", slog.Any("devices", devices))

	slog.Debug("devices", slog.Int("count", len(devices.DeviceRecord)))
	for _, d := range devices.DeviceRecord {
		slog.Debug(
			"device",
			slog.String("MAC", d.Macaddress),
			slog.String("name", d.Info.Name),
		)
	}

	return nil
}
