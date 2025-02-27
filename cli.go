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
	ApplicationKey string   `required:"true" help:"Ambient Weather API 'application' key"`
	APIKey         string   `required:"true" help:"Ambient Weather API key"`
	Device         string   `required:"true" help:"Ambient Weather Device MAC address"`
	ResultsLimit   int64    `required:"false" default:"10" help:"Ambient Weather maximum number of historical results to return"`
	WebhookUrl     *url.URL `required:"true" help:"TRMNL private plugin webhook URL"`
}

func (c *ServeCmd) Run(ctx *kong.Context) error {
	return Update(ambient.NewKey(c.ApplicationKey, c.APIKey), c.Device, c.ResultsLimit, c.WebhookUrl)
}
