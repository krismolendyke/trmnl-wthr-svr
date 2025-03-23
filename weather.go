package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
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
			// Filter the response to only include the specified fields
			filteredData := map[string]any{}
			fields := []string{"tempf", "feelsLike", "humidity", "dailyrainin", "dateutc"}

			for _, field := range fields {
				if value, exists := r.LastDataFields[field]; exists {
					filteredData[field] = value
				}
			}

			return filteredData, nil
		}
	}
	return nil, fmt.Errorf("no device data found for device MAC: %s", mac)
}

// Historical requests past data from the Ambient Weather API for a single device.
// Returns hourly temperature averages with timestamps, reducing the data volume.
// Each returned record contains the average tempf for that hour and the dateutc for the start of the hour.
// Assumes dateutc is in millisecond timestamp format (e.g., 1742535660000)
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

	// Debug only the last 10 records to avoid excessive logging
	recordCount := len(results.RecordFields)
	lastTenRecords := results.RecordFields
	if recordCount > 10 {
		lastTenRecords = results.RecordFields[recordCount-10:]
	}
	slog.Debug("historical (last 10 records only)",
		slog.Int("total_records", recordCount),
		slog.Any("last_records", lastTenRecords))

	// Bucket the data into hourly averages
	hourlyBuckets := make(map[string]struct {
		Sum   float64
		Count int
		First int64 // Store the first timestamp in the hour (in milliseconds)
	})

	for _, record := range results.RecordFields {
		// Skip if no temperature or date
		tempValue, hasTempf := record["tempf"]
		dateValue, hasDate := record["dateutc"]
		if !hasTempf || !hasDate {
			continue
		}

		// Parse the millisecond timestamp
		var timestampMs int64

		switch v := dateValue.(type) {
		case float64:
			timestampMs = int64(v)
		case int64:
			timestampMs = v
		case json.Number:
			if ts, err := v.Int64(); err == nil {
				timestampMs = ts
			} else {
				continue
			}
		case string:
			if ts, err := strconv.ParseInt(v, 10, 64); err == nil {
				timestampMs = ts
			} else {
				continue
			}
		default:
			continue // Skip if date is in an unexpected format
		}

		// Convert to time.Time
		dateTime := time.Unix(timestampMs/1000, 0).UTC()

		// Create bucket key in format "YYYY-MM-DD HH:00"
		hourKey := dateTime.Format("2006-01-02 15:00")

		// Get temperature as float
		var tempf float64
		switch t := tempValue.(type) {
		case float64:
			tempf = t
		case int:
			tempf = float64(t)
		case json.Number:
			if f, err := t.Float64(); err == nil {
				tempf = f
			} else {
				continue
			}
		case string:
			if f, err := strconv.ParseFloat(t, 64); err == nil {
				tempf = f
			} else {
				continue
			}
		default:
			continue
		}

		// Add to the bucket
		bucket, exists := hourlyBuckets[hourKey]
		if !exists {
			// Round down to the start of the hour for the representative timestamp
			hourStart := time.Date(
				dateTime.Year(),
				dateTime.Month(),
				dateTime.Day(),
				dateTime.Hour(),
				0, 0, 0,
				dateTime.Location(),
			)
			hourStartMs := hourStart.Unix() * 1000

			bucket = struct {
				Sum   float64
				Count int
				First int64
			}{0, 0, hourStartMs}
		}
		bucket.Sum += tempf
		bucket.Count++
		hourlyBuckets[hourKey] = bucket
	}

	// Create result records from buckets
	bucketedRecords := make([]map[string]any, 0, len(hourlyBuckets))
	for _, bucket := range hourlyBuckets {
		if bucket.Count > 0 {
			// Round to 1 decimal place for temperature
			avgTemp := math.Round((bucket.Sum/float64(bucket.Count))*10) / 10

			bucketedRecords = append(bucketedRecords, map[string]any{
				"tempf":   avgTemp,
				"dateutc": bucket.First,
			})
		}
	}

	// Sort by timestamp ascending
	sort.Slice(bucketedRecords, func(i, j int) bool {
		timeI, okI := bucketedRecords[i]["dateutc"].(int64)
		timeJ, okJ := bucketedRecords[j]["dateutc"].(int64)
		if !okI || !okJ {
			return false
		}
		return timeI < timeJ
	})

	slog.Info("bucketed historical data",
		slog.Int("original_count", recordCount),
		slog.Int("bucketed_count", len(bucketedRecords)))

	return bucketedRecords, nil
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

	// Log the size of the JSON payload
	payloadSize := len(jsonData)
	slog.Info("webhook payload details",
		slog.Int("size_bytes", payloadSize),
		slog.String("size_human", fmt.Sprintf("%.2f KB", float64(payloadSize)/1024)))

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
