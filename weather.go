package main

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/lrosenman/ambient"
)

func Update(key ambient.Key, mac string) error {
	now := time.Now().UTC()
	results, err := ambient.DeviceMac(key, mac, now, 100)
	if err != nil {
		slog.Error("could not get device data", slog.String("err", err.Error()))
	}
	slog.Debug("results", slog.Any("records", results))
	var jsonResponse map[string]any
	if err := json.Unmarshal(results.JSONResponse, &jsonResponse); err != nil {
		slog.Error("could not unmarshal JSON response", slog.String("err", err.Error()))
	}
	slog.Debug("json response", slog.Any("json", jsonResponse))

	for _, r := range results.Record {
		slog.Info("record", slog.Time("time", r.Date), slog.Float64("temp", r.Tempf))
	}

	return nil
}
