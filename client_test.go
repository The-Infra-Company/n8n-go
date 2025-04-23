package n8n

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient_FieldInitialization(t *testing.T) {
	tests := []struct {
		name          string
		rawBaseURL    string
		expectedBase  string
		expectedTO    time.Duration
		provideClient bool
		customTimeout time.Duration
	}{
		{
			name:         "DefaultClientTrimsSlash",
			rawBaseURL:   "https://api.n8n.io/",
			expectedBase: "https://api.n8n.io",
			expectedTO:   5 * time.Minute,
		},
		{
			name:          "CustomHTTPClientOption",
			rawBaseURL:    "https://example.com",
			expectedBase:  "https://example.com",
			provideClient: true,
			customTimeout: 123 * time.Second,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// arrange
			key := "my-apikey"
			var opts []func(*Client)
			if tt.provideClient {
				opts = append(opts, WithHTTPClient(&http.Client{Timeout: tt.customTimeout}))
			}

			// act
			c := NewClient(key, tt.rawBaseURL, opts...)

			// assert
			require.NotNil(t, c, "client should be non-nil")
			assert.Equal(t, key, c.APIKey, "API key should match")
			assert.Equal(t, tt.expectedBase, c.BaseURL, "baseURL should be trimmed of trailing slash")

			// verify timeout
			timeout := c.http.Timeout
			if tt.provideClient {
				assert.Equal(t, tt.customTimeout, timeout, "custom HTTP client should be used")
			} else {
				assert.Equal(t, tt.expectedTO, timeout, "default timeout should apply")
			}
		})
	}
}

func TestDoRequest_HeadersAndUnmarshal(t *testing.T) {
	// fake response payload
	type payload struct {
		Foo string `json:"foo"`
	}
	want := payload{Foo: "bar"}

	// spin up a test server that verifies headers and returns JSON
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// check the API key header
		assert.Equal(t, "apiKey", r.Header.Get("X-N8N-API-KEY"))
		assert.Contains(t, r.Header.Get("User-Agent"), "n8n-go/")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"foo":"bar"}`)
	}))
	defer ts.Close()

	c := NewClient("apiKey", ts.URL)
	var got payload

	// exercise doRequest
	err := c.DoRequest(context.Background(), http.MethodGet, "/", nil, &got, false)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestDoRequest_RateLimitRetries(t *testing.T) {
	attempts := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			// simulate rate-limit with Retry-After=1
			w.Header().Set("Retry-After", "1")
			http.Error(w, `{"error":{"code":"rate_limit","message":"too many requests"}}`, http.StatusTooManyRequests)
			return
		}
		// success on 3rd try
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := NewClient("key", ts.URL)
	err := c.DoRequest(context.Background(), http.MethodGet, "/", nil, nil, false)
	require.NoError(t, err)
	assert.Equal(t, 3, attempts, "should retry until success")
}
