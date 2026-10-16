package godaddy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
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
// instead of matching "404" in the message: v3 error bodies carry random
// correlation IDs that can contain those digits.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

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

type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	apiSecret  string
	// apiToken is a GoDaddy Personal Access Token (PAT). When set, the client
	// sends it as a Bearer token and uses the v3 Domains API where v3 has the
	// endpoint. Without it, the client uses the sso-key on v1 as before.
	apiToken string

	operationPollInterval time.Duration
	operationTimeout      time.Duration
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
		baseURL:               productionURL,
		apiKey:                apiKey,
		apiSecret:             apiSecret,
		operationPollInterval: 2 * time.Second,
		operationTimeout:      10 * time.Minute,
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

// NewTokenClient creates a client that authenticates with a GoDaddy Personal
// Access Token (PAT) instead of an API key and secret.
func NewTokenClient(apiToken string, opts ...ClientOption) *Client {
	client := NewClient("", "", opts...)
	client.apiToken = strings.TrimSpace(apiToken)
	return client
}

// UsesAPIToken reports whether the client authenticates with a PAT.
func (c *Client) UsesAPIToken() bool {
	return c.apiToken != ""
}

func (c *Client) authorizationHeader() string {
	if c.UsesAPIToken() {
		return "Bearer " + c.apiToken
	}
	return fmt.Sprintf("sso-key %s:%s", c.apiKey, c.apiSecret)
}

func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	return c.doRequestWithRetry(ctx, method, path, body, nil, 0)
}

func (c *Client) doRequestWithHeaders(ctx context.Context, method, path string, body interface{}, headers map[string]string) (*http.Response, error) {
	return c.doRequestWithRetry(ctx, method, path, body, headers, 0)
}

func (c *Client) doRequestWithRetry(ctx context.Context, method, path string, body interface{}, headers map[string]string, retryCount int) (*http.Response, error) {
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

	req.Header.Set("Authorization", c.authorizationHeader())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for name, value := range headers {
		req.Header.Set(name, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		
		// Handle rate limiting with intelligent retry
		if resp.StatusCode == 429 && retryCount < maxRetries {
			fmt.Printf("RATE LIMIT HIT: %s %s - Status: %d, Body: %s (Retry %d/%d)\n",
				req.Method, req.URL.Path, resp.StatusCode, string(bodyBytes), retryCount+1, maxRetries)
			
			// Parse retryAfterSec from response body
			waitTime := c.parseRetryAfter(bodyBytes)
			if waitTime > 0 {
				fmt.Printf("Waiting %d seconds before retry as specified by GoDaddy API\n", waitTime)
				select {
				case <-time.After(time.Duration(waitTime) * time.Second):
					// Continue with retry
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			} else {
				// Fallback exponential backoff if retryAfterSec not found
				backoffTime := time.Duration(1<<uint(retryCount)) * time.Second
				fmt.Printf("Using exponential backoff: waiting %v before retry\n", backoffTime)
				select {
				case <-time.After(backoffTime):
					// Continue with retry
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			
			// Recursive retry
			return c.doRequestWithRetry(ctx, method, path, body, headers, retryCount+1)
		}
		
		// DIAGNOSTIC LOG for non-429 errors or max retries reached
		if resp.StatusCode == 429 {
			fmt.Printf("RATE LIMIT EXHAUSTED: Max retries (%d) reached for %s %s\n",
				maxRetries, req.Method, req.URL.Path)
		}
		
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(bodyBytes)}
	}

	return resp, nil
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
