package godaddy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newTestClient(serverURL string) *Client {
	client := NewClient("test-key", "test-secret")
	client.baseURL = serverURL
	return client
}

func TestClient_RefusesRedirects(t *testing.T) {
	var leaked atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			leaked.Store(true)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/steal", http.StatusFound)
	}))
	defer api.Close()

	_, err := newTestClient(api.URL).GetDomain(context.Background(), "example.com")
	if err == nil || !strings.Contains(err.Error(), "refusing to follow redirect") {
		t.Fatalf("GetDomain() error = %v, want a refused redirect", err)
	}
	if leaked.Load() {
		t.Error("the redirect target received the Authorization header")
	}
}

func TestClient_KeepsCustomCheckRedirect(t *testing.T) {
	custom := func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	client := NewClient("k", "s", WithHTTPClient(&http.Client{CheckRedirect: custom}))
	if client.httpClient.CheckRedirect == nil {
		t.Fatal("CheckRedirect was cleared")
	}
	plain := NewClient("k", "s", WithHTTPClient(&http.Client{}))
	if plain.httpClient.CheckRedirect == nil {
		t.Fatal("a custom client without CheckRedirect still follows redirects")
	}
}

func TestRetryWaitIsCapped(t *testing.T) {
	tests := []struct {
		retryAfter, retryCount int
		want                   time.Duration
	}{
		{retryAfter: 5, want: 5 * time.Second},
		{retryAfter: 60, want: 60 * time.Second},
		{retryAfter: 61, want: maxRetryWait},
		{retryAfter: 999999999, want: maxRetryWait},
		{retryAfter: 0, retryCount: 0, want: time.Second},
		{retryAfter: 0, retryCount: 2, want: 4 * time.Second},
		{retryAfter: 0, retryCount: 40, want: maxRetryWait},
	}
	for _, tt := range tests {
		if got := retryWait(tt.retryAfter, tt.retryCount); got != tt.want {
			t.Errorf("retryWait(%d, %d) = %v, want %v", tt.retryAfter, tt.retryCount, got, tt.want)
		}
	}
}

func TestClient_ErrorBodyIsLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(strings.Repeat("x", 10*maxErrorBodyBytes)))
	}))
	defer server.Close()

	_, err := newTestClient(server.URL).GetDomain(context.Background(), "example.com")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v, want *APIError", err)
	}
	if len(apiErr.Body) != maxErrorBodyBytes {
		t.Errorf("error body length = %d, want %d", len(apiErr.Body), maxErrorBodyBytes)
	}
}

func TestIsNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "missing") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"message":"record 404 is busy"}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.GetDomain(context.Background(), "missing.com")
	if !IsNotFound(err) {
		t.Errorf("IsNotFound(%v) = false, want true", err)
	}
	_, err = client.GetDomain(context.Background(), "busy.com")
	if IsNotFound(err) {
		t.Errorf("IsNotFound(%v) = true for a 409 that mentions 404", err)
	}
}

func TestClient_EscapesPathValues(t *testing.T) {
	var requestURIs []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURIs = append(requestURIs, r.RequestURI)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	ctx := context.Background()
	names := []string{"../../victim.com/records/A/www", "a?b=c", "a#frag", "*", "*.dev"}
	for _, name := range names {
		if _, err := client.GetDNSRecordsByTypeAndName(ctx, "example.com", "A", name); err != nil {
			t.Fatalf("GetDNSRecordsByTypeAndName(%q) error = %v", name, err)
		}
	}

	want := []string{
		"/v1/domains/example.com/records/A/..%2F..%2Fvictim.com%2Frecords%2FA%2Fwww",
		"/v1/domains/example.com/records/A/a%3Fb=c",
		"/v1/domains/example.com/records/A/a%23frag",
		"/v1/domains/example.com/records/A/*",
		"/v1/domains/example.com/records/A/*.dev",
	}
	for i := range want {
		if requestURIs[i] != want[i] {
			t.Errorf("request %d = %q, want %q", i, requestURIs[i], want[i])
		}
	}
}

func TestClient_AvailabilityQueryIsEncoded(t *testing.T) {
	var query string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	if _, err := newTestClient(server.URL).CheckDomainAvailability(context.Background(), "a.com&forTransfer=true"); err != nil {
		t.Fatalf("CheckDomainAvailability() error = %v", err)
	}
	if query != "domain=a.com%26forTransfer%3Dtrue" {
		t.Errorf("query = %q, want the domain encoded as one value", query)
	}
}

func TestClient_RefusesEmptyRecordSet(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	ctx := context.Background()
	if _, err := client.GetDNSRecordsByTypeAndName(ctx, "example.com", "TXT", ""); err == nil {
		t.Error("GetDNSRecordsByTypeAndName with an empty name: want an error")
	}
	if err := client.ReplaceDNSRecordsByTypeAndName(ctx, "example.com", "TXT", " ", nil); err == nil {
		t.Error("ReplaceDNSRecordsByTypeAndName with a blank name: want an error")
	}
	if err := client.DeleteDNSRecord(ctx, "example.com", "", "www"); err == nil {
		t.Error("DeleteDNSRecord with an empty type: want an error")
	}
	if calls.Load() != 0 {
		t.Errorf("API calls = %d, want 0", calls.Load())
	}
}

func TestNewClient_TrimsCredentials(t *testing.T) {
	var auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := NewClient(" key\n", "\tsecret ")
	client.baseURL = server.URL
	if _, err := client.GetDomain(context.Background(), "example.com"); err != nil {
		t.Fatalf("GetDomain() error = %v", err)
	}
	if auth != "sso-key key:secret" {
		t.Errorf("Authorization = %q, want trimmed credentials", auth)
	}
}
