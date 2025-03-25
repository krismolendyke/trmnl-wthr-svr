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

	// Pre-allocate the map with exact capacity needed
	fields := []string{"tempf", "feelsLike", "humidity", "dailyrainin", "dateutc"}
	filteredData := make(map[string]any, len(fields))

	for _, r := range results.DeviceRecord {
		if mac == r.Macaddress {
			// Only copy the fields we need
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

// hourlyBucket holds data for calculating hourly averages
type hourlyBucket struct {
	Sum   float64
	Count int
	First int64 // Store the first timestamp in the hour (in milliseconds)
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

	// Log only a sample of records to reduce memory usage
	recordCount := len(results.RecordFields)
	if recordCount > 10 {
		sampleRecords := results.RecordFields[recordCount-10:]
		slog.Debug("historical sample",
			slog.Int("total_records", recordCount),
			slog.Any("sample_records", sampleRecords))
	}

	// Estimate map size to avoid rehashing
	// Assume 1 record per hour for the last X hours as a reasonable estimate
	estimatedHours := min(24, int(limit/12)) // Assuming ~12 records per hour
	hourlyBuckets := make(map[string]*hourlyBucket, estimatedHours)

	for _, record := range results.RecordFields {
		// Extract temperature and date only once
		tempValue, hasTempf := record["tempf"]
		dateValue, hasDate := record["dateutc"]
		if !hasTempf || !hasDate {
			continue
		}

		// Parse timestamp more efficiently
		var timestampMs int64
		switch v := dateValue.(type) {
		case float64:
			timestampMs = int64(v)
		case int64:
			timestampMs = v
		case json.Number:
			timestampMs, err = v.Int64()
			if err != nil {
				continue
			}
		case string:
			timestampMs, err = strconv.ParseInt(v, 10, 64)
			if err != nil {
				continue
			}
		default:
			continue
		}

		// Convert to time.Time only once
		dateTime := time.Unix(timestampMs/1000, 0).UTC()

		// Format time string once - avoid repeated string formatting
		hourKey := dateTime.Format("2006-01-02 15:00")

		// Get temperature efficiently
		var tempf float64
		switch t := tempValue.(type) {
		case float64:
			tempf = t
		case int:
			tempf = float64(t)
		case json.Number:
			tempf, err = t.Float64()
			if err != nil {
				continue
			}
		case string:
			tempf, err = strconv.ParseFloat(t, 64)
			if err != nil {
				continue
			}
		default:
			continue
		}

		// Add to bucket, creating if needed
		bucket, exists := hourlyBuckets[hourKey]
		if !exists {
			// Compute hour start timestamp efficiently
			hourStartMs := (timestampMs / 3600000) * 3600000 // Round down to the nearest hour
			bucket = &hourlyBucket{First: hourStartMs}
			hourlyBuckets[hourKey] = bucket
		}
		bucket.Sum += tempf
		bucket.Count++
	}

	// Create result records from buckets with pre-allocation
	bucketedRecords := make([]map[string]any, 0, len(hourlyBuckets))

	for _, bucket := range hourlyBuckets {
		if bucket.Count > 0 {
			// Round to 1 decimal place for temperature
			avgTemp := math.Round((bucket.Sum/float64(bucket.Count))*10) / 10

			// Only allocate the fields we need
			record := make(map[string]any, 2)
			record["tempf"] = avgTemp
			record["dateutc"] = bucket.First

			bucketedRecords = append(bucketedRecords, record)
		}
	}

	// Sort by timestamp ascending, reusing the slice
	sort.Slice(bucketedRecords, func(i, j int) bool {
		timeI := bucketedRecords[i]["dateutc"].(int64)
		timeJ := bucketedRecords[j]["dateutc"].(int64)
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

	// Debug with limited output to reduce memory usage
	slog.Debug("sending data to TRMNL",
		slog.String("webhook", webhook.String()),
		slog.Int("historical_count", len(data.MergeVariables.Historical)))

	// Use a buffer pool for JSON marshaling
	buffer := bytes.NewBuffer(make([]byte, 0, 8192)) // Pre-allocate a reasonable buffer size
	encoder := json.NewEncoder(buffer)
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("error marshaling webhook data: %w", err)
	}

	// Log the size of the JSON payload
	payloadSize := buffer.Len()
	slog.Info("webhook payload details",
		slog.Int("size_bytes", payloadSize),
		slog.String("size_human", fmt.Sprintf("%.2f KB", float64(payloadSize)/1024)))

	// Send the HTTP POST request using the buffer directly
	resp, err := http.Post(webhook.String(), "application/json", buffer)
	if err != nil {
		return fmt.Errorf("error sending webhook request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status efficiently
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Only read the body if there's an error
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024)) // Limit body read
		return fmt.Errorf("webhook request failed with status %d: %s", resp.StatusCode, body)
	}

	slog.Info("webhook request sent successfully", slog.Int("status", resp.StatusCode))
	return nil
}
