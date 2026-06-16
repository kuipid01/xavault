package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type ReusableClient struct {
	httpClient *http.Client
}

func NewReusableClient(timeout time.Duration) *ReusableClient {
	return &ReusableClient{
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// DoRequest now accepts an optional map of custom headers
func (c *ReusableClient) DoRequest(ctx context.Context, method, url string, headers map[string]string, reqBody interface{}, resTarget interface{}) error {
	var bodyReader io.Reader

	if reqBody != nil {
		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set Defaults
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Inject custom headers (like Paystack's Authorization)
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("external API returned status %d", resp.StatusCode)
	}

	if resTarget != nil {
		if err := json.NewDecoder(resp.Body).Decode(resTarget); err != nil {
			return fmt.Errorf("failed to decode response body: %w", err)
		}
	}

	return nil
}
