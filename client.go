package n8n

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// version is the tag or git commit; overridden at build time via ldflags.
var version = "dev"

const (
	// default HTTP timeout
	defaultTimeout = 5 * time.Minute
	// max retries for 429 responses
	maxRetries = 3
)

// Client wraps HTTP communication with the n8n API.
// Use NewClient to create an instance.
type Client struct {
	APIKey  string
	BaseURL string
	http    *http.Client
}

// NewClient returns a Client configured with APIKey, baseURL, and optional custom HTTP client.
func NewClient(APIKey, baseURL string, opts ...func(*Client)) *Client {
	c := &Client{
		APIKey:  APIKey,
		BaseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: defaultTimeout},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// WithHTTPClient allows injecting a custom HTTP client (e.g. for testing).
func WithHTTPClient(h *http.Client) func(*Client) {
	return func(c *Client) { c.http = h }
}

// DoRequest sends an HTTP request, handles rate limits (429), and decodes JSON into v.
// req must have URL set to full path (BaseURL + endpoint).
func (c *Client) DoRequest(ctx context.Context, method, endpoint string, body io.Reader, v any, errorOnNoContent bool) error {
	url := c.BaseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}

	req.Header.Set("User-Agent", fmt.Sprintf("n8n-go/%s", version))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-N8N-API-KEY", c.APIKey)

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("http request failed: %w", err)
		}
		defer resp.Body.Close()

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("reading response body: %w", err)
		}

		// Handle non-2xx
		if resp.StatusCode >= 300 {
			apiErr := APIError{StatusCode: resp.StatusCode, RawMessage: data}
			// Try to unmarshal structured error
			tmp := struct {
				Error *APIError `json:"error"`
			}{Error: &apiErr}
			_ = json.Unmarshal(data, &tmp)

			// Check for Retry-After header on 429
			if resp.StatusCode == http.StatusTooManyRequests {
				retry := resp.Header.Get("Retry-After")
				if secs, err := strconv.Atoi(retry); err == nil && secs > 0 {
					apiErr.RetryAfter = time.Duration(secs) * time.Second
				} else {
					apiErr.RetryAfter = 1 * time.Second
				}
			}

			// If retryable, wait and retry
			if apiErr.StatusCode == http.StatusTooManyRequests && apiErr.RetryAfter > 0 && attempt < maxRetries {
				slog.Warn("rate limit hit, retrying", "retryAfter", apiErr.RetryAfter)
				select {
				case <-time.After(apiErr.RetryAfter):
					continue
				case <-ctx.Done():
					return ctx.Err()
				}
			}

			return apiErr
		}

		// No content
		if resp.StatusCode == http.StatusNoContent {
			if errorOnNoContent {
				return APIError{StatusCode: 204, Code: "no_content", Message: "no content"}
			}
			return nil
		}

		// Decode JSON
		if v != nil {
			if err := json.Unmarshal(data, v); err != nil {
				return fmt.Errorf("decoding response: %w; raw: %s", err, data)
			}
		}
		return nil
	}
	return lastErr
}
