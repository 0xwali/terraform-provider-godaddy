package godaddy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// RateLimitResponse represents the structure of rate limit error response
type RateLimitResponse struct {
	Code          string `json:"code"`
	Message       string `json:"message"`
	RetryAfterSec int    `json:"retryAfterSec"`
}

const (
	productionURL = "https://api.godaddy.com"
	testURL       = "https://api.ote-godaddy.com"
)

const (
	// maxRetryWait caps the wait a 429 response can ask for, so a bad or
	// hostile retryAfterSec cannot hang a CI job.
	maxRetryWait = 60 * time.Second
	// maxErrorBodyBytes limits how much of an error response is read and
	// echoed back in error messages.
	maxErrorBodyBytes = 64 << 10
)

// APIError is a non-2xx response from the GoDaddy API.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error: status %d, body: %s", e.StatusCode, e.Body)
}

// IsNotFound reports whether err wraps a GoDaddy 404 response. Use it
// instead of matching "404" in the message, which can also appear in an
// error body.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	apiSecret  string
}

type ClientOption func(*Client)

func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

func WithTestEnvironment() ClientOption {
	return func(c *Client) {
		c.baseURL = testURL
	}
}

func NewClient(apiKey, apiSecret string, opts ...ClientOption) *Client {
	client := &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL:   productionURL,
		apiKey:    strings.TrimSpace(apiKey),
		apiSecret: strings.TrimSpace(apiSecret),
	}

	for _, opt := range opts {
		opt(client)
	}

	// The GoDaddy API does not redirect. Go's default policy would follow a
	// redirect and resend the Authorization header to the same host even over
	// plain http, so refuse every redirect instead.
	if client.httpClient.CheckRedirect == nil {
		client.httpClient.CheckRedirect = refuseRedirect
	}

	return client
}

func refuseRedirect(req *http.Request, via []*http.Request) error {
	status := 0
	if req.Response != nil {
		status = req.Response.StatusCode
	}
	return fmt.Errorf("refusing to follow redirect (status %d) to %s: the GoDaddy API does not redirect", status, req.URL.Redacted())
}

func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	return c.doRequestWithRetry(ctx, method, path, body, 0)
}

func (c *Client) doRequestWithRetry(ctx context.Context, method, path string, body interface{}, retryCount int) (*http.Response, error) {
	const maxRetries = 3

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("sso-key %s:%s", c.apiKey, c.apiSecret))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))

		// Handle rate limiting with intelligent retry
		if resp.StatusCode == 429 && retryCount < maxRetries {
			wait := retryWait(c.parseRetryAfter(bodyBytes), retryCount)
			tflog.Warn(ctx, "GoDaddy API rate limit hit, retrying", map[string]interface{}{
				"method":  req.Method,
				"path":    req.URL.Path,
				"retry":   retryCount + 1,
				"retries": maxRetries,
				"wait":    wait.String(),
			})

			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return nil, ctx.Err()
			}

			return c.doRequestWithRetry(ctx, method, path, body, retryCount+1)
		}

		if resp.StatusCode == 429 {
			tflog.Warn(ctx, "GoDaddy API rate limit retries exhausted", map[string]interface{}{
				"method":  req.Method,
				"path":    req.URL.Path,
				"retries": maxRetries,
			})
		}

		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(bodyBytes)}
	}

	return resp, nil
}

// retryWait returns how long to wait before retry number retryCount+1: the
// server's retryAfterSec when it gave one, otherwise exponential backoff,
// never more than maxRetryWait.
func retryWait(retryAfterSec, retryCount int) time.Duration {
	if retryAfterSec > int(maxRetryWait/time.Second) {
		return maxRetryWait
	}
	if retryAfterSec > 0 {
		return time.Duration(retryAfterSec) * time.Second
	}
	wait := time.Duration(1<<uint(retryCount)) * time.Second
	if wait > maxRetryWait || wait <= 0 {
		wait = maxRetryWait
	}
	return wait
}

// parseRetryAfter extracts retryAfterSec from GoDaddy's rate limit response
func (c *Client) parseRetryAfter(bodyBytes []byte) int {
	var rateLimitResp RateLimitResponse
	if err := json.Unmarshal(bodyBytes, &rateLimitResp); err == nil && rateLimitResp.RetryAfterSec > 0 {
		return rateLimitResp.RetryAfterSec
	}
	
	// Fallback: try to extract from message string
	bodyStr := string(bodyBytes)
	if strings.Contains(bodyStr, "retryAfterSec") {
		// Look for "retryAfterSec":51 pattern
		start := strings.Index(bodyStr, "retryAfterSec\":")
		if start != -1 {
			start += len("retryAfterSec\":")
			end := start
			for end < len(bodyStr) && bodyStr[end] >= '0' && bodyStr[end] <= '9' {
				end++
			}
			if waitTimeStr := bodyStr[start:end]; len(waitTimeStr) > 0 {
				if waitTime, err := strconv.Atoi(waitTimeStr); err == nil {
					return waitTime
				}
			}
		}
	}
	
	return 0 // No valid retry time found
}

func (c *Client) Get(ctx context.Context, path string, result interface{}) error {
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

func (c *Client) Post(ctx context.Context, path string, body, result interface{}) error {
	resp, err := c.doRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

func (c *Client) Put(ctx context.Context, path string, body interface{}) error {
	resp, err := c.doRequest(ctx, http.MethodPut, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (c *Client) Patch(ctx context.Context, path string, body interface{}) error {
	resp, err := c.doRequest(ctx, http.MethodPatch, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (c *Client) Delete(ctx context.Context, path string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// pathSegment escapes a user value for one URL path segment, so a domain or
// record name cannot add segments ("../", "/"), a query ("?") or a fragment
// ("#") to the request. '*' stays literal because wildcard record names use it.
func pathSegment(value string) string {
	return strings.ReplaceAll(url.PathEscape(value), "%2A", "*")
}
