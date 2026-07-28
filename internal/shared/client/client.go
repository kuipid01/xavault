package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type ReusableClient struct {
	httpClient *http.Client
}

type ExternalAPIError struct {
	StatusCode int
	Message    string
	Code       string
	Type       string
	Body       string
}

func (e *ExternalAPIError) Error() string {
	if e == nil {
		return ""
	}

	if e.Message != "" {
		return e.Message
	}

	return fmt.Sprintf("external API returned status %d", e.StatusCode)
}

func NewReusableClient(timeout time.Duration) *ReusableClient {
	return &ReusableClient{
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// DoRequest now accepts an optional map of custom headers
func (c *ReusableClient) DoRequest(
	ctx context.Context,
	method string,
	requestURL string,
	headers map[string]string,
	reqBody interface{},
	resTarget interface{},
) error {
	start := time.Now()

	var bodyReader io.Reader
	if reqBody != nil {
		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			log.Printf(
				"[HTTP_CLIENT][MARSHAL_ERROR] method=%s url=%s error=%v",
				method,
				requestURL,
				err,
			)

			return fmt.Errorf("failed to marshal request body: %w", err)
		}

		bodyReader = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL, bodyReader)
	if err != nil {
		log.Printf(
			"[HTTP_CLIENT][CREATE_REQUEST_ERROR] method=%s url=%s error=%v",
			method,
			requestURL,
			err,
		)

		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	log.Printf(
		"[HTTP_CLIENT][REQUEST_START] method=%s url=%s headers=%v body_present=%v",
		method,
		requestURL,
		safeHeaders(req.Header),
		reqBody != nil,
	)

	resp, err := c.httpClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		log.Printf(
			"[HTTP_CLIENT][REQUEST_ERROR] method=%s url=%s duration=%s error=%v",
			method,
			requestURL,
			duration,
			err,
		)

		return fmt.Errorf("request failed: %w", err)
	}

	defer resp.Body.Close()

	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		log.Printf(
			"[HTTP_CLIENT][READ_BODY_ERROR] method=%s url=%s status=%d duration=%s error=%v",
			method,
			requestURL,
			resp.StatusCode,
			duration,
			readErr,
		)

		return fmt.Errorf("failed to read response body: %w", readErr)
	}

	responseBody := string(bodyBytes)

	log.Printf(
		"[HTTP_CLIENT][RESPONSE] method=%s url=%s status=%d duration=%s body_size=%d",
		method,
		requestURL,
		resp.StatusCode,
		duration,
		len(bodyBytes),
	)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf(
			"[HTTP_CLIENT][NON_2XX] method=%s url=%s status=%d duration=%s body_size=%d",
			method,
			requestURL,
			resp.StatusCode,
			duration,
			len(bodyBytes),
		)

		return parseExternalAPIError(resp.StatusCode, responseBody)
	}

	if resTarget != nil && len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, resTarget); err != nil {
			log.Printf(
				"[HTTP_CLIENT][DECODE_ERROR] method=%s url=%s status=%d duration=%s body_size=%d error=%v",
				method,
				requestURL,
				resp.StatusCode,
				duration,
				len(bodyBytes),
				err,
			)

			return fmt.Errorf("failed to decode response body: %w", err)
		}
	}

	log.Printf(
		"[HTTP_CLIENT][REQUEST_SUCCESS] method=%s url=%s status=%d duration=%s",
		method,
		requestURL,
		resp.StatusCode,
		duration,
	)

	return nil
}

func parseExternalAPIError(statusCode int, body string) error {
	var payload struct {
		Message string `json:"message"`
		Code    string `json:"code"`
		Type    string `json:"type"`
	}

	err := json.Unmarshal([]byte(body), &payload)
	if err != nil {
		return &ExternalAPIError{
			StatusCode: statusCode,
			Message:    fmt.Sprintf("external API returned status %d", statusCode),
			Body:       body,
		}
	}

	return &ExternalAPIError{
		StatusCode: statusCode,
		Message:    payload.Message,
		Code:       payload.Code,
		Type:       payload.Type,
		Body:       body,
	}
}

func safeHeaders(headers http.Header) map[string]string {
	safe := make(map[string]string)

	for key, values := range headers {
		if len(values) == 0 {
			continue
		}

		value := values[0]

		switch strings.ToLower(key) {
		case "authorization", "x-api-key", "api-key", "secret-key":
			safe[key] = maskSecret(value)
		default:
			safe[key] = value
		}
	}

	return safe
}

func maskSecret(value string) string {
	if value == "" {
		return ""
	}

	if len(value) <= 10 {
		return "****"
	}

	return value[:6] + "****" + value[len(value)-4:]
}
