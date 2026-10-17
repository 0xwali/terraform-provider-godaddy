package godaddy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type recordedRequest struct {
	Method         string
	Path           string
	Query          string
	Auth           string
	IdempotencyKey string
	Body           string
}

// fakeGoDaddy records every request and answers from a route table keyed by
// "METHOD /path".
type fakeGoDaddy struct {
	t        *testing.T
	mu       sync.Mutex
	requests []recordedRequest
	routes   map[string]func(w http.ResponseWriter, r *http.Request)
}

func newFakeGoDaddy(t *testing.T) (*fakeGoDaddy, *httptest.Server) {
	f := &fakeGoDaddy{t: t, routes: map[string]func(http.ResponseWriter, *http.Request){}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.requests = append(f.requests, recordedRequest{
			Method:         r.Method,
			Path:           r.URL.Path,
			Query:          r.URL.RawQuery,
			Auth:           r.Header.Get("Authorization"),
			IdempotencyKey: r.Header.Get("Idempotency-Key"),
			Body:           string(body),
		})
		f.mu.Unlock()

		handler, ok := f.routes[r.Method+" "+r.URL.Path]
		if !ok {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	return f, server
}

func (f *fakeGoDaddy) handle(route string, status int, body string) {
	f.routes[route] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func (f *fakeGoDaddy) calls(method, pathPrefix string) []recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []recordedRequest
	for _, req := range f.requests {
		if req.Method == method && strings.HasPrefix(req.Path, pathPrefix) {
			out = append(out, req)
		}
	}
	return out
}

func newTestTokenClient(serverURL string) *Client {
	client := NewTokenClient("  test-pat\n")
	client.baseURL = serverURL
	client.operationPollInterval = time.Millisecond
	client.operationTimeout = time.Second
	return client
}

func TestNewTokenClient(t *testing.T) {
	client := NewTokenClient(" my-token ", WithTestEnvironment())

	if !client.UsesAPIToken() {
		t.Fatal("UsesAPIToken() = false, want true")
	}
	if got := client.authorizationHeader(); got != "Bearer my-token" {
		t.Errorf("authorizationHeader() = %q, want %q", got, "Bearer my-token")
	}
	if client.baseURL != testURL {
		t.Errorf("baseURL = %q, want %q", client.baseURL, testURL)
	}
}

func TestKeyClientKeepsSSOKeyAndV1(t *testing.T) {
	f, server := newFakeGoDaddy(t)
	f.handle("GET /v1/domains/example.com", http.StatusOK, `{"domain":"example.com","status":"ACTIVE","nameServers":["ns1.a.com","ns2.a.com"]}`)
	f.handle("PATCH /v1/domains/example.com", http.StatusNoContent, ``)

	client := NewClient("key", "secret")
	client.baseURL = server.URL

	if client.UsesAPIToken() {
		t.Fatal("UsesAPIToken() = true for a key client")
	}
	if _, err := client.GetDomain(context.Background(), "example.com"); err != nil {
		t.Fatalf("GetDomain() error = %v", err)
	}
	if err := client.UpdateDomain(context.Background(), "example.com", DomainUpdate{NameServers: []string{"ns1.b.com", "ns2.b.com"}}); err != nil {
		t.Fatalf("UpdateDomain() error = %v", err)
	}

	for _, req := range f.requests {
		if req.Auth != "sso-key key:secret" {
			t.Errorf("%s %s Authorization = %q, want sso-key", req.Method, req.Path, req.Auth)
		}
		if strings.HasPrefix(req.Path, "/v3/") {
			t.Errorf("key client called v3: %s %s", req.Method, req.Path)
		}
	}
}

func TestTokenClient_GetDomainUsesV3(t *testing.T) {
	f, server := newFakeGoDaddy(t)
	f.handle("GET /v3/domains/domain-names/example.com", http.StatusOK, `{
		"domain": "example.com",
		"status": "ACTIVE",
		"expiresAt": "2027-01-02T03:04:05Z",
		"createdAt": "2020-01-02T03:04:05Z",
		"autoRenew": true,
		"privacy": true,
		"transferLock": true,
		"nameServers": ["ns1.v3.com", "ns2.v3.com"]
	}`)
	f.handle("GET /v1/domains/example.com", http.StatusOK, `{
		"domain": "example.com",
		"status": "STALE",
		"expirationProtected": true,
		"transferProtected": true,
		"locked": false,
		"renewAuto": false,
		"nameServers": ["ns1.v1.com", "ns2.v1.com"],
		"contactAdmin": {"nameFirst": "Ada", "email": "ada@example.com"}
	}`)

	client := newTestTokenClient(server.URL)
	domain, err := client.GetDomain(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("GetDomain() error = %v", err)
	}

	if domain.Status != "ACTIVE" || !domain.Locked || !domain.RenewAuto || !domain.Privacy {
		t.Errorf("v3 fields not used: status=%q locked=%v renewAuto=%v privacy=%v", domain.Status, domain.Locked, domain.RenewAuto, domain.Privacy)
	}
	if strings.Join(domain.Nameservers, ",") != "ns1.v3.com,ns2.v3.com" {
		t.Errorf("Nameservers = %v, want the v3 list", domain.Nameservers)
	}
	if domain.Expires == nil || domain.Expires.Year() != 2027 {
		t.Errorf("Expires = %v, want 2027 from v3", domain.Expires)
	}
	// Fields v3 does not return come from v1.
	if !domain.ExpirationProtected || !domain.TransferProtected || domain.ContactAdmin.NameFirst != "Ada" {
		t.Errorf("v1-only fields missing: expirationProtected=%v transferProtected=%v admin=%q",
			domain.ExpirationProtected, domain.TransferProtected, domain.ContactAdmin.NameFirst)
	}

	for _, req := range f.requests {
		if req.Auth != "Bearer test-pat" {
			t.Errorf("%s %s Authorization = %q, want %q", req.Method, req.Path, req.Auth, "Bearer test-pat")
		}
	}
}

func TestTokenClient_UpdateNameserversWaitsForOperation(t *testing.T) {
	f, server := newFakeGoDaddy(t)
	f.handle("PUT /v3/domains/domain-names/example.com/nameservers", http.StatusAccepted,
		`{"operationId":"op-1","type":"UPDATE_NAMESERVERS","status":"CONFIRMED"}`)

	polls := 0
	f.routes["GET /v3/domains/operations/op-1"] = func(w http.ResponseWriter, r *http.Request) {
		polls++
		status := "EXECUTING"
		if polls >= 2 {
			status = "COMPLETED"
		}
		_ = json.NewEncoder(w).Encode(DomainOperation{OperationID: "op-1", Status: status})
	}

	client := newTestTokenClient(server.URL)
	err := client.UpdateDomain(context.Background(), "example.com", DomainUpdate{NameServers: []string{"ns1.new.com", "ns2.new.com"}})
	if err != nil {
		t.Fatalf("UpdateDomain() error = %v", err)
	}

	puts := f.calls(http.MethodPut, "/v3/domains/domain-names/example.com/nameservers")
	if len(puts) != 1 {
		t.Fatalf("nameserver PUT calls = %d, want 1", len(puts))
	}
	if puts[0].Body != `["ns1.new.com","ns2.new.com"]` {
		t.Errorf("PUT body = %s, want a bare JSON array", puts[0].Body)
	}
	if len(puts[0].IdempotencyKey) < 16 || len(puts[0].IdempotencyKey) > 64 {
		t.Errorf("Idempotency-Key = %q, want 16-64 chars", puts[0].IdempotencyKey)
	}
	if polls != 2 {
		t.Errorf("operation polls = %d, want 2", polls)
	}
	if len(f.calls(http.MethodPatch, "/v1/")) != 0 {
		t.Error("nameserver update fell back to v1 PATCH")
	}
}

func TestTokenClient_UpdateDomainRoutesEachField(t *testing.T) {
	f, server := newFakeGoDaddy(t)
	done := `{"operationId":"op","status":"COMPLETED"}`
	f.handle("POST /v3/domains/domain-names/example.com/auto-renew", http.StatusAccepted, done)
	f.handle("POST /v3/domains/domain-names/example.com/transfer-lock", http.StatusAccepted, done)
	f.handle("PATCH /v1/domains/example.com", http.StatusNoContent, ``)

	renew, locked, whois := false, true, true
	client := newTestTokenClient(server.URL)
	err := client.UpdateDomain(context.Background(), "example.com", DomainUpdate{
		RenewAuto:   &renew,
		Locked:      &locked,
		ExposeWhois: &whois,
	})
	if err != nil {
		t.Fatalf("UpdateDomain() error = %v", err)
	}

	if got := f.calls(http.MethodPost, "/v3/domains/domain-names/example.com/auto-renew"); len(got) != 1 || got[0].Body != "false" {
		t.Errorf("auto-renew calls = %+v, want one with body false", got)
	}
	if got := f.calls(http.MethodPost, "/v3/domains/domain-names/example.com/transfer-lock"); len(got) != 1 || got[0].Body != "true" {
		t.Errorf("transfer-lock calls = %+v, want one with body true", got)
	}
	patches := f.calls(http.MethodPatch, "/v1/domains/example.com")
	if len(patches) != 1 || patches[0].Body != `{"exposeWhois":true}` {
		t.Errorf("v1 PATCH calls = %+v, want one with only exposeWhois", patches)
	}
	if len(f.calls(http.MethodPut, "/v3/domains/domain-names/example.com/nameservers")) != 0 {
		t.Error("nameservers were updated without a change")
	}
}

func TestTokenClient_FailedOperation(t *testing.T) {
	f, server := newFakeGoDaddy(t)
	f.handle("PUT /v3/domains/domain-names/example.com/nameservers", http.StatusAccepted,
		`{"operationId":"op-2","status":"FAILED","error":{"name":"INVALID_NAMESERVER","message":"bad host"}}`)

	client := newTestTokenClient(server.URL)
	err := client.UpdateDomain(context.Background(), "example.com", DomainUpdate{NameServers: []string{"a", "b"}})
	if err == nil || !strings.Contains(err.Error(), "INVALID_NAMESERVER") {
		t.Fatalf("UpdateDomain() error = %v, want INVALID_NAMESERVER", err)
	}
}

func TestTokenClient_OperationTimeout(t *testing.T) {
	f, server := newFakeGoDaddy(t)
	f.handle("PUT /v3/domains/domain-names/example.com/nameservers", http.StatusAccepted, `{"operationId":"op-3","status":"EXECUTING"}`)
	f.handle("GET /v3/domains/operations/op-3", http.StatusOK, `{"operationId":"op-3","status":"EXECUTING"}`)

	client := newTestTokenClient(server.URL)
	client.operationTimeout = 20 * time.Millisecond
	err := client.UpdateDomain(context.Background(), "example.com", DomainUpdate{NameServers: []string{"a", "b"}})
	if err == nil || !strings.Contains(err.Error(), "still EXECUTING") {
		t.Fatalf("UpdateDomain() error = %v, want a timeout", err)
	}
}

func TestTokenClient_GetDNSRecordsV3(t *testing.T) {
	f, server := newFakeGoDaddy(t)
	f.handle("GET /v3/domains/zones/example.com/dns-records", http.StatusOK, `{"items":[
		{"recordId":"r1","type":"A","name":"www","data":"192.0.2.1","ttl":600},
		{"recordId":"r2","type":"A","name":"wwwx","data":"192.0.2.2","ttl":600}
	],"links":[]}`)

	client := newTestTokenClient(server.URL)
	records, err := client.GetDNSRecordsByTypeAndName(context.Background(), "example.com", "A", "www")
	if err != nil {
		t.Fatalf("GetDNSRecordsByTypeAndName() error = %v", err)
	}
	if len(records) != 1 || records[0].Data != "192.0.2.1" {
		t.Errorf("records = %+v, want only www", records)
	}

	got := f.calls(http.MethodGet, "/v3/domains/zones/example.com/dns-records")
	if len(got) != 1 || !strings.Contains(got[0].Query, "type=A") || !strings.Contains(got[0].Query, "name=www") {
		t.Errorf("list calls = %+v, want type and name filters", got)
	}
}

func TestTokenClient_GetDNSRecordsV3Paginates(t *testing.T) {
	f, server := newFakeGoDaddy(t)
	f.routes["GET /v3/domains/zones/example.com/dns-records"] = func(w http.ResponseWriter, r *http.Request) {
		count := v3DNSPageSize
		if r.URL.Query().Get("page") == "2" {
			count = 3
		}
		list := v3DNSRecordList{}
		for i := 0; i < count; i++ {
			list.Items = append(list.Items, v3DNSRecord{RecordID: "r", Type: "TXT", Name: "@", Data: "x", TTL: 600})
		}
		_ = json.NewEncoder(w).Encode(list)
	}

	f.handle("GET /v1/domains/example.com/records", http.StatusOK, `[]`)

	client := newTestTokenClient(server.URL)
	records, err := client.GetDNSRecords(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("GetDNSRecords() error = %v", err)
	}
	if len(records) != v3DNSPageSize+3 {
		t.Errorf("records = %d, want %d", len(records), v3DNSPageSize+3)
	}
}

func TestTokenClient_ZoneListMergesV3AndV1(t *testing.T) {
	f, server := newFakeGoDaddy(t)
	f.handle("GET /v3/domains/zones/example.com/dns-records", http.StatusOK, `{"items":[
		{"recordId":"a1","type":"A","name":"@","data":"192.0.2.1","ttl":600},
		{"recordId":"c1","type":"CAA","name":"@","data":"letsencrypt.org","ttl":600,"flag":0,"tag":"issue"},
		{"recordId":"n1","type":"NS","name":"@","data":"ns1.domaincontrol.com.","ttl":3600}
	],"links":[]}`)
	f.handle("GET /v1/domains/example.com/records", http.StatusOK, `[
		{"type":"A","name":"@","data":"192.0.2.1","ttl":600},
		{"type":"CAA","name":"@","data":"0 issue \"letsencrypt.org\"","ttl":600},
		{"type":"NS","name":"@","data":"ns1.domaincontrol.com","ttl":3600}
	]`)

	client := newTestTokenClient(server.URL)
	records, err := client.GetDNSRecords(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("GetDNSRecords() error = %v", err)
	}

	byType := map[string][]string{}
	for _, record := range records {
		byType[record.Type] = append(byType[record.Type], record.Data)
	}
	if len(byType["A"]) != 1 {
		t.Errorf("A records = %v, want one (from v3, not duplicated)", byType["A"])
	}
	if len(byType["CAA"]) != 1 || byType["CAA"][0] != `0 issue "letsencrypt.org"` {
		t.Errorf("CAA records = %v, want the full v1 data", byType["CAA"])
	}
	if len(byType["NS"]) != 1 || byType["NS"][0] != "ns1.domaincontrol.com" {
		t.Errorf("NS records = %v, want the v1 record", byType["NS"])
	}
}

func TestTokenClient_LongTTLWritesUseV1(t *testing.T) {
	f, server := newFakeGoDaddy(t)
	f.handle("PATCH /v1/domains/example.com/records", http.StatusOK, ``)
	f.handle("PUT /v1/domains/example.com/records/TXT/@", http.StatusOK, ``)

	client := newTestTokenClient(server.URL)
	ctx := context.Background()
	long := DNSRecord{Type: "TXT", Name: "@", Data: "v=spf1 -all", TTL: 604800}
	if err := client.AddDNSRecord(ctx, "example.com", long); err != nil {
		t.Fatalf("AddDNSRecord() error = %v", err)
	}
	if err := client.ReplaceDNSRecordsByTypeAndName(ctx, "example.com", "TXT", "@", []DNSRecord{long}); err != nil {
		t.Fatalf("ReplaceDNSRecordsByTypeAndName() error = %v", err)
	}
	for _, req := range f.requests {
		if strings.HasPrefix(req.Path, "/v3/") {
			t.Errorf("TTL above %d went to v3: %s %s", v3MaxTTL, req.Method, req.Path)
		}
	}
}

func TestTokenClient_NSRecordsUseV1(t *testing.T) {
	f, server := newFakeGoDaddy(t)
	f.handle("GET /v1/domains/example.com/records/NS/@", http.StatusOK, `[]`)

	client := newTestTokenClient(server.URL)
	if _, err := client.GetDNSRecordsByTypeAndName(context.Background(), "example.com", "NS", "@"); err != nil {
		t.Fatalf("GetDNSRecordsByTypeAndName() error = %v", err)
	}
	if len(f.calls(http.MethodGet, "/v3/")) != 0 {
		t.Error("NS records went to v3")
	}
}

func TestTokenClient_TrailingDotIsNormalized(t *testing.T) {
	f, server := newFakeGoDaddy(t)
	f.handle("GET /v3/domains/zones/example.com/dns-records", http.StatusOK, `{"items":[
		{"recordId":"m1","type":"MX","name":"@","data":"mail.example.com.","ttl":600,"priority":10}
	],"links":[]}`)

	priority := 10
	client := newTestTokenClient(server.URL)
	records, err := client.GetDNSRecordsByTypeAndName(context.Background(), "example.com", "MX", "@")
	if err != nil {
		t.Fatalf("GetDNSRecordsByTypeAndName() error = %v", err)
	}
	if len(records) != 1 || records[0].Data != "mail.example.com" {
		t.Fatalf("records = %+v, want data without the trailing dot", records)
	}

	// The same record in config form must count as unchanged.
	err = client.ReplaceDNSRecordsByTypeAndName(context.Background(), "example.com", "MX", "@", []DNSRecord{
		{Type: "MX", Name: "@", Data: "mail.example.com", TTL: 600, Priority: &priority},
	})
	if err != nil {
		t.Fatalf("ReplaceDNSRecordsByTypeAndName() error = %v", err)
	}
	if n := len(f.calls(http.MethodPut, "/v3/")) + len(f.calls(http.MethodPost, "/v3/")) + len(f.calls(http.MethodDelete, "/v3/")); n != 0 {
		t.Errorf("writes = %d, want 0 for an unchanged record", n)
	}
}

func TestTokenClient_GetDomainKeepsV1WhenV3OmitsFields(t *testing.T) {
	f, server := newFakeGoDaddy(t)
	f.handle("GET /v3/domains/domain-names/example.com", http.StatusOK,
		`{"domain":"example.com","status":"ACTIVE","createdAt":"2020-01-02T03:04:05Z","autoRenew":true,"privacy":false}`)
	f.handle("GET /v1/domains/example.com", http.StatusOK,
		`{"domain":"example.com","locked":true,"nameServers":["ns1.v1.com","ns2.v1.com"]}`)

	client := newTestTokenClient(server.URL)
	domain, err := client.GetDomain(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("GetDomain() error = %v", err)
	}
	if !domain.Locked {
		t.Error("Locked = false, want the v1 value when v3 omits transferLock")
	}
	if strings.Join(domain.Nameservers, ",") != "ns1.v1.com,ns2.v1.com" {
		t.Errorf("Nameservers = %v, want the v1 list when v3 omits nameServers", domain.Nameservers)
	}
}

func TestTokenClient_UnknownOperationStatusStops(t *testing.T) {
	f, server := newFakeGoDaddy(t)
	f.handle("PUT /v3/domains/domain-names/example.com/nameservers", http.StatusAccepted,
		`{"operationId":"op-4","status":"CANCELLED"}`)

	client := newTestTokenClient(server.URL)
	err := client.UpdateDomain(context.Background(), "example.com", DomainUpdate{NameServers: []string{"a", "b"}})
	if err == nil || !strings.Contains(err.Error(), "CANCELLED") {
		t.Fatalf("UpdateDomain() error = %v, want CANCELLED", err)
	}
	if len(f.calls(http.MethodGet, "/v3/domains/operations/")) != 0 {
		t.Error("polled an operation with an unknown status")
	}
}

func TestIsNotFound_StatusNotBodyText(t *testing.T) {
	f, server := newFakeGoDaddy(t)
	f.handle("GET /v1/domains/missing.com", http.StatusNotFound, `{"code":"NOT_FOUND"}`)
	f.handle("GET /v1/domains/conflict.com", http.StatusConflict, `{"correlationId":"a404b"}`)

	client := NewClient("key", "secret")
	client.baseURL = server.URL
	ctx := context.Background()

	_, err := client.GetDomain(ctx, "missing.com")
	if !IsNotFound(err) {
		t.Errorf("IsNotFound(%v) = false, want true", err)
	}
	_, err = client.GetDomain(ctx, "conflict.com")
	if IsNotFound(err) {
		t.Errorf("IsNotFound(%v) = true for a 409 whose body contains 404", err)
	}
}

func TestTokenClient_ReplaceDNSRecordSetV3(t *testing.T) {
	f, server := newFakeGoDaddy(t)
	f.handle("GET /v3/domains/zones/example.com/dns-records", http.StatusOK, `{"items":[
		{"recordId":"keep","type":"A","name":"www","data":"192.0.2.1","ttl":600},
		{"recordId":"change","type":"A","name":"www","data":"192.0.2.2","ttl":600},
		{"recordId":"drop","type":"A","name":"www","data":"192.0.2.3","ttl":600}
	],"links":[]}`)
	f.handle("PUT /v3/domains/zones/example.com/dns-records/change", http.StatusOK, `{}`)
	f.handle("DELETE /v3/domains/zones/example.com/dns-records/drop", http.StatusNoContent, ``)

	client := newTestTokenClient(server.URL)
	err := client.ReplaceDNSRecordsByTypeAndName(context.Background(), "example.com", "A", "www", []DNSRecord{
		{Type: "A", Name: "www", Data: "192.0.2.1", TTL: 600},
		{Type: "A", Name: "www", Data: "192.0.2.9", TTL: 600},
	})
	if err != nil {
		t.Fatalf("ReplaceDNSRecordsByTypeAndName() error = %v", err)
	}

	puts := f.calls(http.MethodPut, "/v3/")
	if len(puts) != 1 || !strings.Contains(puts[0].Body, "192.0.2.9") || strings.Contains(puts[0].Body, "recordId") {
		t.Errorf("PUT calls = %+v, want one rewrite to 192.0.2.9 without recordId", puts)
	}
	if got := f.calls(http.MethodDelete, "/v3/"); len(got) != 1 {
		t.Errorf("DELETE calls = %d, want 1", len(got))
	}
	if got := f.calls(http.MethodPost, "/v3/"); len(got) != 0 {
		t.Errorf("POST calls = %d, want 0", len(got))
	}
	if got := f.calls(http.MethodPut, "/v1/"); len(got) != 0 {
		t.Error("replace fell back to v1")
	}
}

func TestTokenClient_AddAndDeleteDNSRecordV3(t *testing.T) {
	f, server := newFakeGoDaddy(t)
	f.handle("POST /v3/domains/zones/example.com/dns-records", http.StatusCreated, `{"recordId":"new"}`)
	f.handle("GET /v3/domains/zones/example.com/dns-records", http.StatusOK, `{"items":[
		{"recordId":"m1","type":"MX","name":"@","data":"mail.example.com","ttl":600,"priority":10}
	],"links":[]}`)
	f.handle("DELETE /v3/domains/zones/example.com/dns-records/m1", http.StatusNoContent, ``)

	priority := 10
	client := newTestTokenClient(server.URL)
	ctx := context.Background()
	if err := client.AddDNSRecord(ctx, "example.com", DNSRecord{Type: "mx", Name: "@", Data: "mail.example.com", TTL: 600, Priority: &priority}); err != nil {
		t.Fatalf("AddDNSRecord() error = %v", err)
	}
	if err := client.DeleteDNSRecord(ctx, "example.com", "MX", "@"); err != nil {
		t.Fatalf("DeleteDNSRecord() error = %v", err)
	}

	posts := f.calls(http.MethodPost, "/v3/domains/zones/example.com/dns-records")
	if len(posts) != 1 || !strings.Contains(posts[0].Body, `"type":"MX"`) || !strings.Contains(posts[0].Body, `"priority":10`) {
		t.Errorf("POST calls = %+v, want one MX record with priority", posts)
	}
	if got := f.calls(http.MethodDelete, "/v3/domains/zones/example.com/dns-records/m1"); len(got) != 1 {
		t.Errorf("DELETE calls = %d, want 1", len(got))
	}
}

func TestTokenClient_UnsupportedV3TypesUseV1(t *testing.T) {
	f, server := newFakeGoDaddy(t)
	f.handle("GET /v1/domains/example.com/records/CAA/@", http.StatusOK, `[]`)
	f.handle("PATCH /v1/domains/example.com/records", http.StatusOK, ``)

	client := newTestTokenClient(server.URL)
	ctx := context.Background()
	if _, err := client.GetDNSRecordsByTypeAndName(ctx, "example.com", "CAA", "@"); err != nil {
		t.Fatalf("GetDNSRecordsByTypeAndName() error = %v", err)
	}
	if err := client.AddDNSRecord(ctx, "example.com", DNSRecord{Type: "SSHFP", Name: "@", Data: "1 1 abc", TTL: 600}); err != nil {
		t.Fatalf("AddDNSRecord() error = %v", err)
	}

	if len(f.calls(http.MethodGet, "/v3/")) != 0 || len(f.calls(http.MethodPost, "/v3/")) != 0 {
		t.Error("v3 was called for a record type it does not support")
	}
	for _, req := range f.requests {
		if req.Auth != "Bearer test-pat" {
			t.Errorf("%s %s Authorization = %q, want the PAT", req.Method, req.Path, req.Auth)
		}
	}
}

func TestNextPagePath(t *testing.T) {
	tests := []struct {
		name string
		href string
		want string
	}{
		{"absolute link keeps only path and query", "https://api.godaddy.com/v3/domains/domain-names?pageSize=200&pageToken=abc", "/v3/domains/domain-names?pageSize=200&pageToken=abc"},
		{"relative link", "/v3/domains/domain-names?pageToken=abc", "/v3/domains/domain-names?pageToken=abc"},
		{"opaque href cannot reach another host", "x:@evil.example/steal", ""},
		{"userinfo is refused", "https://user@api.godaddy.com/v3/domains/domain-names", ""},
		{"other endpoint is refused", "https://api.godaddy.com/v1/shoppers/1", ""},
		{"dot segments are refused", "/v3/domains/domain-names/../../v1/x", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			links := []v3Link{{Rel: "self", Href: "/v3/domains/domain-names"}, {Rel: "next", Href: tt.href}}
			if got := nextPagePath(links); got != tt.want {
				t.Errorf("nextPagePath(%q) = %q, want %q", tt.href, got, tt.want)
			}
		})
	}
	if got := nextPagePath([]v3Link{{Rel: "self", Href: "/v3/domains/domain-names"}}); got != "" {
		t.Errorf("nextPagePath() without next = %q, want empty", got)
	}
}

func TestTokenClient_ListDomainsStaysOnHost(t *testing.T) {
	f, server := newFakeGoDaddy(t)
	f.routes["GET /v3/domains/domain-names"] = func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("pageToken") == "" {
			_, _ = w.Write([]byte(`{"items":[{"domain":"a.com","status":"ACTIVE","createdAt":"2020-01-02T03:04:05Z"}],
				"links":[{"rel":"next","href":"https://other.example/v3/domains/domain-names?pageToken=2"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"items":[{"domain":"b.com","status":"ACTIVE","createdAt":"2020-01-02T03:04:05Z"}],"links":[]}`))
	}

	client := newTestTokenClient(server.URL)
	domains, err := client.ListDomains(context.Background())
	if err != nil {
		t.Fatalf("ListDomains() error = %v", err)
	}
	if len(domains) != 2 || domains[1].Domain != "b.com" {
		t.Errorf("domains = %+v, want a.com and b.com from the test server", domains)
	}
}

func TestTokenClient_RejectsPathTricks(t *testing.T) {
	_, server := newFakeGoDaddy(t)
	client := newTestTokenClient(server.URL)
	ctx := context.Background()

	if _, err := client.GetDomain(ctx, ".."); err == nil {
		t.Error("GetDomain(\"..\") succeeded")
	}
	if _, err := client.GetDNSRecordsByTypeAndName(ctx, "example.com", "A", ""); err == nil {
		t.Error("GetDNSRecordsByTypeAndName with an empty name succeeded")
	}
	if err := client.ReplaceDNSRecordsByTypeAndName(ctx, "example.com", "TXT", "", nil); err == nil {
		t.Error("ReplaceDNSRecordsByTypeAndName with an empty name succeeded; it would rewrite every TXT record")
	}
	if err := client.DeleteDNSRecord(ctx, "example.com", "A", ""); err == nil {
		t.Error("DeleteDNSRecord with an empty name succeeded; it would delete every A record")
	}
}

func TestTokenClient_EscapesPathValues(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{"items":[],"links":[]}`))
	}))
	t.Cleanup(server.Close)

	client := newTestTokenClient(server.URL)
	_, _ = client.GetDNSRecordsByType(context.Background(), "example.com/../../v1/x?y", "A")
	if gotPath != "/v3/domains/zones/example.com%2F..%2F..%2Fv1%2Fx%3Fy/dns-records" {
		t.Errorf("path = %q, want the domain escaped as one segment", gotPath)
	}
}
