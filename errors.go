package n8n

import (
	"errors"
	"fmt"
	"time"
)

// APIError is an error type that exposes additional information about why an API request failed.
type APIError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	StatusCode int
	RawMessage []byte
	RetryAfter time.Duration
}

// Error provides a user friendly error message.
func (e APIError) Error() string {
	return fmt.Sprintf("%s - %s", e.Code, e.Message)
}

func NotFound(err error) bool {
	var apiErr APIError
	return err != nil && errors.As(err, &apiErr) && apiErr.StatusCode == 404
}

func noContent(err error) bool {
	var apiErr APIError
	return err != nil && errors.As(err, &apiErr) && apiErr.StatusCode == 204
}
