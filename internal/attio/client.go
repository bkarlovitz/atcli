package attio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.attio.com/v2"

type Client struct {
	token      string
	httpClient *http.Client
	baseURL    string
}

type ClientOption func(*Client)

func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		if strings.TrimSpace(baseURL) != "" {
			c.baseURL = strings.TrimRight(baseURL, "/")
		}
	}
}

func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

func NewClient(token string, opts ...ClientOption) *Client {
	c := &Client{
		token:      token,
		httpClient: http.DefaultClient,
		baseURL:    defaultBaseURL,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type APIError struct {
	StatusCode    int
	Status        string
	Body          string
	RetryAfter    time.Duration
	HasRetryAfter bool
}

func (e *APIError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("Attio returned %s", e.Status)
	}
	return fmt.Sprintf("Attio returned %s: %s", e.Status, e.Body)
}

func IsPermissionError(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusForbidden || apiErr.StatusCode == http.StatusUnauthorized)
}

func (c *Client) getJSON(ctx context.Context, path string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return c.apiError(resp)
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}

func (c *Client) postJSON(ctx context.Context, path string, payload, target any) error {
	return c.writeJSON(ctx, http.MethodPost, path, payload, target)
}

func (c *Client) putJSON(ctx context.Context, path string, payload, target any) error {
	return c.writeJSON(ctx, http.MethodPut, path, payload, target)
}

func (c *Client) writeJSON(ctx context.Context, method, path string, payload, target any) error {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, &body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return c.apiError(resp)
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}

func (c *Client) apiError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	bodyText := strings.TrimSpace(string(body))
	if c.token != "" {
		bodyText = strings.ReplaceAll(bodyText, c.token, "[redacted]")
	}
	retryAfter, hasRetryAfter := parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
	return &APIError{
		StatusCode:    resp.StatusCode,
		Status:        resp.Status,
		Body:          bodyText,
		RetryAfter:    retryAfter,
		HasRetryAfter: hasRetryAfter,
	}
}

func parseRetryAfter(raw string, now time.Time) (time.Duration, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	if seconds, err := time.ParseDuration(raw + "s"); err == nil {
		if seconds < 0 {
			return 0, true
		}
		return seconds, true
	}
	retryAt, err := http.ParseTime(raw)
	if err != nil {
		return 0, false
	}
	delay := retryAt.Sub(now)
	if delay < 0 {
		return 0, true
	}
	return delay, true
}
