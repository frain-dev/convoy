package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/pkg/log"
)

const (
	// DefaultOverwatchHost is the hardcoded Overwatch URL
	DefaultOverwatchHost = "https://overwatch.getconvoy.cloud"
	// DefaultValidatePath is the default validation endpoint
	DefaultValidatePath = "/licenses/validate"
	// DefaultTimeout is the default request timeout
	DefaultTimeout = 10 * time.Second
	// DefaultRetryCount is the default number of retries
	DefaultRetryCount = 3
)

// Client handles communication with the license service
type Client struct {
	host         string
	validatePath string
	timeout      time.Duration
	retryCount   int
	httpClient   *http.Client
	logger       log.StdLogger
}

// Config holds configuration for the license service client
type Config struct {
	Host         string
	ValidatePath string
	Timeout      time.Duration
	RetryCount   int
	Logger       log.StdLogger
}

// NewClient creates a new license service client
func NewClient(cfg Config) *Client {
	if cfg.Host == "" {
		cfg.Host = DefaultOverwatchHost
	}
	if cfg.ValidatePath == "" {
		cfg.ValidatePath = DefaultValidatePath
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	}
	if cfg.RetryCount == 0 {
		cfg.RetryCount = DefaultRetryCount
	}

	return &Client{
		host:         cfg.Host,
		validatePath: cfg.ValidatePath,
		timeout:      cfg.Timeout,
		retryCount:   cfg.RetryCount,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		logger: cfg.Logger,
	}
}

// ValidateLicense validates a license key with the license service
func (c *Client) ValidateLicense(ctx context.Context, licenseKey string) (*LicenseValidationData, error) {
	if licenseKey == "" {
		return nil, fmt.Errorf("license key is required")
	}

	reqBody := ValidateLicenseRequest{
		LicenseKey: licenseKey,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s%s", c.host, c.validatePath)

	var lastErr error
	for attempt := 0; attempt <= c.retryCount; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			if c.logger != nil {
				c.logger.WithError(err).Warnf("License validation attempt %d failed, retrying...", attempt+1)
			}
			continue
		}

		// Read and close body immediately to avoid connection pool exhaustion during retries
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read response: %w", err)
			continue
		}

		// Check HTTP status code
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			if c.logger != nil {
				c.logger.Warnf("License validation attempt %d returned status %d, retrying...", attempt+1, resp.StatusCode)
			}
			continue
		}

		var validationResp LicenseValidationResponse
		if err := json.Unmarshal(body, &validationResp); err != nil {
			lastErr = fmt.Errorf("failed to unmarshal response: %w", err)
			continue
		}

		if !validationResp.Status {
			err := c.parseError(validationResp.Message)
			return nil, err
		}

		if validationResp.Data == nil {
			return nil, fmt.Errorf("invalid response: data is nil")
		}

		// Check license status (consolidated with parseError logic)
		if err := c.checkLicenseStatus(validationResp.Data.Status); err != nil {
			return nil, err
		}

		return validationResp.Data, nil
	}

	return nil, fmt.Errorf("license validation failed after %d attempts: %w", c.retryCount+1, lastErr)
}

// parseError parses error message and returns appropriate error type
func (c *Client) parseError(message string) error {
	switch message {
	case "License not found":
		return ErrLicenseNotFound
	case "License is suspended":
		return ErrLicenseSuspended
	case "License has expired":
		return ErrLicenseExpired
	case "License has been revoked":
		return ErrLicenseRevoked
	default:
		return &LicenseError{
			Message: message,
			Status:  "validation_failed",
		}
	}
}

// checkLicenseStatus validates the license status
func (c *Client) checkLicenseStatus(status string) error {
	switch status {
	case "active":
		return nil
	case "suspended":
		return ErrLicenseSuspended
	case "expired":
		return ErrLicenseExpired
	case "revoked":
		return ErrLicenseRevoked
	default:
		return fmt.Errorf("unknown license status: %s", status)
	}
}
