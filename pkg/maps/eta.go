package maps

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const distanceMatrixEndpoint = "https://maps.googleapis.com/maps/api/distancematrix/json"

// Client wraps calls to the Google Distance Matrix API.
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Google Maps client.
// Returns nil if apiKey is empty so callers can skip ETA lookups.
func NewClient(apiKey string) *Client {
	if apiKey == "" {
		return nil
	}
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// distanceMatrixResponse is a minimal representation of the API response.
type distanceMatrixResponse struct {
	Rows []struct {
		Elements []struct {
			Status   string `json:"status"`
			Duration struct {
				Value int `json:"value"` // seconds
			} `json:"duration"`
		} `json:"elements"`
	} `json:"rows"`
	Status string `json:"status"`
}

// GetETA returns the travel time in seconds from (originLat, originLng) to
// destinationAddress using the Google Distance Matrix API.
// An error is returned on network failure or when no route is found.
func (c *Client) GetETA(ctx context.Context, originLat, originLng float64, destinationAddress string) (int, error) {
	origin := fmt.Sprintf("%f,%f", originLat, originLng)
	u, err := url.Parse(distanceMatrixEndpoint)
	if err != nil {
		return 0, fmt.Errorf("maps: parse endpoint URL: %w", err)
	}
	q := u.Query()
	q.Set("origins", origin)
	q.Set("destinations", destinationAddress)
	q.Set("key", c.apiKey)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, fmt.Errorf("maps: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("maps: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("maps: unexpected status %d", resp.StatusCode)
	}

	var body distanceMatrixResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, fmt.Errorf("maps: decode response: %w", err)
	}

	if body.Status != "OK" {
		return 0, fmt.Errorf("maps: API status %s", body.Status)
	}

	if len(body.Rows) == 0 || len(body.Rows[0].Elements) == 0 {
		return 0, fmt.Errorf("maps: no results")
	}

	elem := body.Rows[0].Elements[0]
	if elem.Status != "OK" {
		return 0, fmt.Errorf("maps: element status %s", elem.Status)
	}

	return elem.Duration.Value, nil
}
