package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	DefaultOverwatchHost   = "https://overwatch.getconvoy.cloud"
	DefaultRedirectPath    = "/sso/redirect"
	DefaultTokenPath       = "/sso/token"
	DefaultAdminPortalPath = "/sso/admin-portal"
	DefaultTimeout         = 10 * time.Second
	DefaultRetryCount      = 3
)

type Config struct {
	Host            string
	RedirectPath    string
	TokenPath       string
	AdminPortalPath string
	Timeout         time.Duration
	RetryCount      int
	APIKey          string
	LicenseKey      string
	OrgID           string
}

type Client struct {
	host            string
	redirectPath    string
	tokenPath       string
	adminPortalPath string
	timeout         time.Duration
	retryCount      int
	httpClient      *http.Client
	apiKey          string
	licenseKey      string
	orgID           string
}

func NewClient(cfg Config) *Client {
	if cfg.Host == "" {
		cfg.Host = DefaultOverwatchHost
	}
	if cfg.RedirectPath == "" {
		cfg.RedirectPath = DefaultRedirectPath
	}
	if cfg.TokenPath == "" {
		cfg.TokenPath = DefaultTokenPath
	}
	if cfg.AdminPortalPath == "" {
		cfg.AdminPortalPath = DefaultAdminPortalPath
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	}
	if cfg.RetryCount == 0 {
		cfg.RetryCount = DefaultRetryCount
	}
	return &Client{
		host:            strings.TrimSuffix(cfg.Host, "/"),
		redirectPath:    cfg.RedirectPath,
		tokenPath:       cfg.TokenPath,
		adminPortalPath: cfg.AdminPortalPath,
		timeout:         cfg.Timeout,
		retryCount:      cfg.RetryCount,
		httpClient:      &http.Client{Timeout: cfg.Timeout},
		apiKey:          cfg.APIKey,
		licenseKey:      cfg.LicenseKey,
		orgID:           cfg.OrgID,
	}
}

// setAuthHeaders sets Authorization (when apiKey is set) and X-License-Key (when licenseKeyForHeader is non-empty).
func (c *Client) setAuthHeaders(req *http.Request, licenseKeyForHeader string) {
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	if licenseKeyForHeader != "" {
		req.Header.Set("X-License-Key", licenseKeyForHeader)
	}
}

func (c *Client) GetRedirectURL(ctx context.Context, licenseKey, _, redirectURI string) (*RedirectURLResponse, error) {
	if licenseKey == "" {
		return nil, fmt.Errorf("license key is required")
	}
	if redirectURI == "" {
		return nil, fmt.Errorf("redirect URI is required")
	}

	body := RedirectURLRequest{CallbackURL: redirectURI}
	if c.apiKey != "" && c.orgID != "" {
		body.OrgID = c.orgID
	}
	bodyBytes, _ := json.Marshal(body)
	url := c.host + c.redirectPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeaders(req, licenseKey)

	var lastErr error
	for attempt := 0; attempt <= c.retryCount; attempt++ {
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		rb, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(rb))
			continue
		}
		var out RedirectURLResponse
		if err := json.Unmarshal(rb, &out); err != nil {
			lastErr = err
			continue
		}
		if !out.Status {
			lastErr = fmt.Errorf("SSO redirect failed: %s", out.Message)
			continue
		}
		if out.Data.RedirectURL == "" && out.Data.AuthorizationURL != "" {
			out.Data.RedirectURL = out.Data.AuthorizationURL
		}
		return &out, nil
	}
	return nil, lastErr
}

func (c *Client) ValidateToken(ctx context.Context, token string) (*TokenValidationResponse, error) {
	if token == "" {
		return nil, fmt.Errorf("token is required")
	}

	body := TokenValidationRequest{Token: token}
	if c.apiKey != "" && c.orgID != "" {
		body.OrgID = c.orgID
	}
	bodyBytes, _ := json.Marshal(body)
	url := c.host + c.tokenPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeaders(req, c.licenseKey)

	var lastErr error
	for attempt := 0; attempt <= c.retryCount; attempt++ {
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		rb, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(rb))
			continue
		}
		var out tokenResponseFlex
		if err := json.Unmarshal(rb, &out); err != nil {
			lastErr = err
			continue
		}
		if !out.Status {
			return nil, fmt.Errorf("SSO token validation failed: %s", out.Message)
		}
		p := out.Data.Payload
		if p == nil {
			p = out.Data.Profile
		}
		if p == nil {
			return nil, fmt.Errorf("email is missing from profile")
		}
		if p.ID == "" && p.ProfileID != "" {
			p.ID = p.ProfileID
		}
		if p.OrganizationExternalID == "" && p.OrganizationID != "" {
			p.OrganizationExternalID = p.OrganizationID
		}
		if p.Email == "" {
			return nil, fmt.Errorf("email is missing from profile")
		}
		return &TokenValidationResponse{
			Status:  out.Status,
			Message: out.Message,
			Data:    TokenValidationData{Payload: *p},
		}, nil
	}
	return nil, lastErr
}

func (c *Client) GetAdminPortalURL(ctx context.Context, licenseKey, returnURL, successURL string) (*AdminPortalResponse, error) {
	if licenseKey == "" {
		return nil, fmt.Errorf("license key is required")
	}
	if returnURL == "" {
		return nil, fmt.Errorf("return_url is required")
	}

	body := AdminPortalRequest{ReturnURL: returnURL, SuccessURL: successURL}
	if c.apiKey != "" && c.orgID != "" {
		body.OrgID = c.orgID
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	url := c.host + c.adminPortalPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeaders(req, licenseKey)

	var lastErr error
	for attempt := 0; attempt <= c.retryCount; attempt++ {
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		rb, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(rb))
			continue
		}
		var out AdminPortalResponse
		if err := json.Unmarshal(rb, &out); err != nil {
			lastErr = err
			continue
		}
		if !out.Status {
			lastErr = fmt.Errorf("SSO admin portal failed: %s", out.Message)
			continue
		}
		return &out, nil
	}
	return nil, lastErr
}

type tokenResponseFlex struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		Payload *UserProfile `json:"payload"`
		Profile *UserProfile `json:"profile"`
	} `json:"data"`
}
