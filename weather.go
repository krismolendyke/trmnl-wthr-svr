package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/lrosenman/ambient"
)

// MergeVariables contains the Ambient Weather API data used for templating in the TRMNL plugin.
type MergeVariables struct {
	Latest     map[string]any   `json:"latest"`
	Historical []map[string]any `json:"historical"`
}

// WebhookData wraps up the Ambient Weather API response in the webhook data format expected by TRMNL.
type WebhookData struct {
	MergeVariables MergeVariables `json:"merge_variables"`
}

// Latest requests the most recent data from the Ambient Weather API for the given device MAC address.
func Latest(key ambient.Key, mac string) (map[string]any, error) {
	slog.Info("getting latest weather data", slog.String("mac", mac))
	results, err := ambient.Device(key)
	if err != nil {
		slog.Error("could not get latest devices data", slog.String("err", err.Error()))
		return nil, err
	}
	if results.HTTPResponseCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected response code: %d, json: %s", results.HTTPResponseCode, results.JSONResponse)
	}
	slog.Debug("latest", slog.Any("records", results))
	if len(results.DeviceRecord) == 0 {
		return nil, fmt.Errorf("received zero device records")
	}
	for _, r := range results.DeviceRecord {
		if mac == r.Macaddress {
			return r.LastDataFields, nil
		}
	}
	return nil, fmt.Errorf("no device data found for device MAC: %s", mac)
}

// Historical requests past data from the Ambient Weather API for a single device.
// The Ambient Weather API docs state that each record will be in 5 or 30 minute granularity and that the maximum amount
// of records to request is 288 (and defaults to that value).
// https://ambientweather.docs.apiary.io/#reference/0/device-data/query-device-data
func Historical(key ambient.Key, mac string, limit int64) ([]map[string]any, error) {
	slog.Info("getting historical weather data", slog.String("mac", mac), slog.Int64("records", limit))
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
	return results.RecordFields, nil
}

// Data assembles latest and historical data into something that can be sent to the TRMNL webhook URL.
func Data(key ambient.Key, mac string, limit int64) (*WebhookData, error) {
	latest, err := Latest(key, mac)
	if err != nil {
		return nil, err
	}

	// HACK work around ridiculous immediate 429 response for making >1 request in a second
	// "API requests are capped at 1 request/second for each user's apiKey and 3 requests/second per applicationKey."
	// -- https://ambientweather.docs.apiary.io/#introduction/rate-limiting
	// TODO remove this hack with a proper retry
	time.Sleep(time.Second)

	historical, err := Historical(key, mac, limit)
	if err != nil {
		return nil, err
	}

	return &WebhookData{
		MergeVariables: MergeVariables{
			Latest:     latest,
			Historical: historical,
		},
	}, nil
}

func Update(key ambient.Key, mac string, limit int64, webhook *url.URL) error {
	data, err := Data(key, mac, limit)
	if err != nil {
		return err
	}
	slog.Debug("sending data to TRMNL", slog.String("webhook", webhook.String()), slog.Any("data", data))

	// Convert the data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error marshaling webhook data: %w", err)
	}

	// Send the HTTP POST request
	resp, err := http.Post(webhook.String(), "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error sending webhook request: %w", err)
	}
	defer resp.Body.Close()

	// Check the response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook request failed with status %d: %s", resp.StatusCode, string(body))
	}

	slog.Info("webhook request sent successfully", slog.Int("status", resp.StatusCode))
	return nil
}
