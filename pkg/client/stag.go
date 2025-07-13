package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/tabular/relay/pkg/types"
)

// StagClient handles communication with STAG service
type StagClient struct {
	baseURL    string
	httpClient *http.Client
	apiKey     string
}

// NewStagClient creates a new STAG client
func NewStagClient(baseURL, apiKey string, timeout time.Duration) *StagClient {
	return &StagClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// IngestEvents sends a batch of events to STAG
func (c *StagClient) IngestEvents(ctx context.Context, events []types.SpatialEvent) error {
	if len(events) == 0 {
		return nil
	}
	
	// Create batch payload
	batch := map[string]interface{}{
		"events":    events,
		"timestamp": time.Now().UnixMilli(),
		"count":     len(events),
	}
	
	return c.postJSON(ctx, "/ingest", batch)
}

// HealthCheck verifies STAG service availability
func (c *StagClient) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}
	
	c.addHeaders(req)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("STAG health check returned status %d", resp.StatusCode)
	}
	
	return nil
}

// postJSON sends a JSON POST request to STAG
func (c *StagClient) postJSON(ctx context.Context, endpoint string, payload interface{}) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}
	
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		c.baseURL+endpoint,
		bytes.NewReader(jsonData),
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	c.addHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("STAG returned status %d", resp.StatusCode)
	}
	
	return nil
}

// addHeaders adds common headers to requests
func (c *StagClient) addHeaders(req *http.Request) {
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	req.Header.Set("User-Agent", "tabular-relay/1.0")
}