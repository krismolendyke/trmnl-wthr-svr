package main

import (
	"net/url"

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
	ApplicationKey  string   `required:"true" help:"API 'application' key" env:"TRMNL_WTHR_SVR_APP_KEY"`
	APIKey          string   `required:"true" help:"API key" env:"TRMNL_WTHR_SVR_API_KEY"`
	Device          string   `required:"true" help:"Device MAC address" env:"TRMNL_WTHR_SVR_DEVICE"`
	ResultsLimit    int64    `required:"false" default:"10" help:"Maximum number of results to return"`
	TrmnlWebhookUrl *url.URL `required:"true" help:"TRMNL private plugin webhook URL" env:"TRMNL_WTHR_SVR_WEBHOOK"`
}

func (c *ServeCmd) Run(ctx *kong.Context) error {
	return Update(ambient.NewKey(c.ApplicationKey, c.APIKey), c.Device, c.ResultsLimit, c.TrmnlWebhookUrl)
}
