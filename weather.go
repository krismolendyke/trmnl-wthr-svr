package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/lrosenman/ambient"
)

type TrmnlWebhookData struct {
	MergeVariables struct {
	} `json:"merge_variables"`
}

func Historical(key ambient.Key, mac string, limit int64) ([]ambient.Record, error) {
	now := time.Now().UTC()
	results, err := ambient.DeviceMac(key, mac, now, limit)
	if err != nil {
		slog.Error("could not get historical device data", slog.String("err", err.Error()))
		return nil, err
	}
	if results.HTTPResponseCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected response code: %d, json: %s", results.HTTPResponseCode, results.JSONResponse)
	}
	slog.Debug("historical", slog.Any("records", results))
	return results.Record, nil
}

func Latest(key ambient.Key) ([]ambient.DeviceRecord, error) {
	results, err := ambient.Device(key)
	if err != nil {
		slog.Error("could not get latest devices data", slog.String("err", err.Error()))
		return nil, err
	}
	if results.HTTPResponseCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected response code: %d, json: %s", results.HTTPResponseCode, results.JSONResponse)
	}
	slog.Debug("latest", slog.Any("records", results))
	return results.DeviceRecord, nil
}

func Update(key ambient.Key, mac string, limit int64, webhook *url.URL) error {
	historical, err := Historical(key, mac, limit)
	if err != nil {
		return err
	}
	for _, r := range historical {
		slog.Info("historical records", slog.Time("date", r.Date), slog.Float64("temp", r.Tempf))
	}

	latest, err := Latest(key)
	if err != nil {
		return err
	}
	for _, r := range latest {
		slog.Info("latest records", slog.Time("date", r.LastData.Date), slog.Float64("temp", r.LastData.Tempf))
	}

	// TODO assemble data to send to webhook

	slog.Debug("sending data to TRMNL", slog.String("webhook", webhook.String()))

	return nil
}
