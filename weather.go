package main

import (
	"log/slog"
	"time"

	"github.com/lrosenman/ambient"
)

func Update(key ambient.Key, mac string, resultsLimit int64) error {
	now := time.Now().UTC()
	results, err := ambient.DeviceMac(key, mac, now, resultsLimit)
	if err != nil {
		slog.Error("could not get device data", slog.String("err", err.Error()))
	}
	slog.Debug("results", slog.Any("records", results))
	slog.Debug("json response", slog.Any("json", string(results.JSONResponse)))

	for _, r := range results.Record {
		slog.Info("record", slog.Time("time", r.Date), slog.Float64("temp", r.Tempf))
	}

	return nil
}
