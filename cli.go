package main

import (
	"net/url"
	"time"
)

type Globals struct {
	Debug bool `short:"D" help:"Enable debug mode"`
}

type CLI struct {
	Globals

	Server ServerCmd `cmd:"" help:"Run the webhook server"`
}

type ServerCmd struct {
	ApplicationKey string        `required:"true" help:"Ambient Weather API 'application' key"`
	APIKey         string        `required:"true" help:"Ambient Weather API key"`
	Device         string        `required:"true" help:"Ambient Weather Device MAC address"`
	ResultsLimit   int64         `required:"false" default:"288" help:"Ambient Weather maximum number of historical results to return"`
	WebhookUrl     *url.URL      `required:"true" help:"TRMNL private plugin webhook URL"`
	Interval       time.Duration `required:"false" default:"15m" help:"Time interval between data updates"`
}
