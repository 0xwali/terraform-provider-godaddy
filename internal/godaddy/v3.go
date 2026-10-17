package godaddy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// The v3 Domains API only accepts a Personal Access Token (PAT). The client
// uses it when it has a token, and falls back to v1 (which also accepts a PAT)
// for operations that v3 does not have yet.
// Spec: https://developer.godaddy.com/openapi/domains-v3.json
const v3Prefix = "/v3/domains"

const (
	operationStatusConfirmed = "CONFIRMED"
	operationStatusExecuting = "EXECUTING"
	operationStatusCompleted = "COMPLETED"

	v3DNSPageSize = 100
	v3MaxPages    = 1000
	// v3MaxTTL is the v3 DNSRecord ttl maximum. v1 allows more, so records
	// with a longer TTL stay on v1.
	v3MaxTTL = 86400
)

// v3DNSRecordTypes are the record types the provider sends to the v3 DNS
// endpoints. v3 also lists NS, SOA and CAA, but GoDaddy-managed NS and SOA
// records are read-only on v3 (409), and v3 models CAA with separate flag and
// tag fields, so those stay on v1 with the rest of the types v3 does not
// support.
var v3DNSRecordTypes = map[string]bool{
	RecordTypeA:     true,
	RecordTypeAAAA:  true,
	RecordTypeCNAME: true,
	RecordTypeMX:    true,
	RecordTypeSRV:   true,
	RecordTypeTXT:   true,
}

// hostnameRecordTypes hold a hostname in data. v3 may return it with a
// trailing dot; v1 and the provider config use the form without it.
var hostnameRecordTypes = map[string]bool{
	RecordTypeCNAME: true,
	RecordTypeMX:    true,
	RecordTypeNS:    true,
	RecordTypeSRV:   true,
}

type v3Domain struct {
	Domain       string     `json:"domain"`
	Status       string     `json:"status"`
	ExpiresAt    *time.Time `json:"expiresAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	RenewBy      *time.Time `json:"renewBy,omitempty"`
	UpdatedAt    *time.Time `json:"updatedAt,omitempty"`
	AutoRenew    bool       `json:"autoRenew"`
	Privacy      bool       `json:"privacy"`
	TransferLock *bool      `json:"transferLock,omitempty"`
	NameServers  []string   `json:"nameServers,omitempty"`
}

type v3Link struct {
	Rel  string `json:"rel"`
	Href string `json:"href"`
}

type v3DomainCollection struct {
	Items []v3Domain `json:"items"`
	Links []v3Link   `json:"links"`
}

type v3Error struct {
	Name          string `json:"name"`
	Message       string `json:"message"`
	CorrelationID string `json:"correlationId"`
}

// DomainOperation tracks an async v3 domain change, such as a nameserver update.
type DomainOperation struct {
	OperationID string   `json:"operationId"`
	Type        string   `json:"type"`
	Domain      string   `json:"domain,omitempty"`
	Status      string   `json:"status"`
	Error       *v3Error `json:"error,omitempty"`
}

type v3DNSRecord struct {
	RecordID string  `json:"recordId,omitempty"`
	Type     string  `json:"type"`
	Name     string  `json:"name"`
	Data     string  `json:"data"`
	TTL      int     `json:"ttl"`
	Priority *int    `json:"priority,omitempty"`
	Port     *int    `json:"port,omitempty"`
	Weight   *int    `json:"weight,omitempty"`
	Service  *string `json:"service,omitempty"`
	Protocol *string `json:"protocol,omitempty"`
}

type v3DNSRecordList struct {
	Items []v3DNSRecord `json:"items"`
}

// checkPathValues rejects values that would change which endpoint a v3 path
// points at once placed in it. url.PathEscape handles '/', '?' and '#', but
// leaves "." and ".." alone, and an empty value drops a path segment.
func checkPathValues(values map[string]string) error {
	for field, value := range values {
		if value == "" || value == "." || value == ".." {
			return fmt.Errorf("invalid %s %q", field, value)
		}
	}
	return nil
}

func (c *Client) useV3DNS(recordType string) bool {
	return c.UsesAPIToken() && v3DNSRecordTypes[strings.ToUpper(recordType)]
}

// useV3DNSWrite is useV3DNS for writes: v3 rejects a TTL above v3MaxTTL, so
// any such record sends the whole write to v1.
func (c *Client) useV3DNSWrite(recordType string, records ...DNSRecord) bool {
	if !c.useV3DNS(recordType) {
		return false
	}
	for _, record := range records {
		if record.TTL > v3MaxTTL {
			return false
		}
	}
	return true
}

func (d v3Domain) toDomain() Domain {
	domain := Domain{
		Domain:        d.Domain,
		Status:        d.Status,
		Expires:       d.ExpiresAt,
		Privacy:       d.Privacy,
		RenewAuto:     d.AutoRenew,
		RenewDeadline: d.RenewBy,
		CreatedAt:     d.CreatedAt,
	}
	if d.TransferLock != nil {
		domain.Locked = *d.TransferLock
	}
	if d.UpdatedAt != nil {
		domain.ModifiedAt = *d.UpdatedAt
	}
	return domain
}

func (c *Client) getDomainV3(ctx context.Context, domain string) (*DomainDetail, error) {
	var v3 v3Domain
	if err := c.Get(ctx, fmt.Sprintf("%s/domain-names/%s", v3Prefix, url.PathEscape(domain)), &v3); err != nil {
		return nil, err
	}

	// v3 does not return contacts or the expiration/transfer protection flags
	// yet. Read them from v1 so state does not drift, then let v3 win for
	// every field it does return.
	var detail DomainDetail
	if err := c.Get(ctx, fmt.Sprintf("/v1/domains/%s", url.PathEscape(domain)), &detail); err != nil {
		return nil, err
	}

	v3Fields := v3.toDomain()
	detail.Domain.Domain = v3Fields.Domain
	detail.Status = v3Fields.Status
	detail.Expires = v3Fields.Expires
	detail.Privacy = v3Fields.Privacy
	detail.RenewAuto = v3Fields.RenewAuto
	detail.RenewDeadline = v3Fields.RenewDeadline
	detail.CreatedAt = v3Fields.CreatedAt
	if !v3Fields.ModifiedAt.IsZero() {
		detail.ModifiedAt = v3Fields.ModifiedAt
	}
	// transferLock and nameServers are optional in the v3 schema. Keep the v1
	// values when v3 leaves them out, so a missing field does not look like a
	// change and trigger an update on every apply.
	if v3.TransferLock != nil {
		detail.Locked = *v3.TransferLock
	}
	if len(v3.NameServers) > 0 {
		detail.Nameservers = v3.NameServers
	}

	return &detail, nil
}

func (c *Client) listDomainsV3(ctx context.Context) ([]Domain, error) {
	var domains []Domain
	path := fmt.Sprintf("%s/domain-names?pageSize=200", v3Prefix)

	for page := 0; page < v3MaxPages && path != ""; page++ {
		var collection v3DomainCollection
		if err := c.Get(ctx, path, &collection); err != nil {
			return nil, err
		}
		for _, d := range collection.Items {
			domains = append(domains, d.toDomain())
		}
		path = nextPagePath(collection.Links)
	}

	return domains, nil
}

// nextPagePath returns the path and query of the "next" link, so the request
// still goes to the client's base URL. It ignores the link's host and only
// accepts a plain path under the domain list endpoint: a crafted href such
// as "x:@evil.example/steal" would otherwise be joined onto the base URL and
// send the token to another host.
func nextPagePath(links []v3Link) string {
	for _, link := range links {
		if link.Rel != "next" || link.Href == "" {
			continue
		}
		next, err := url.Parse(link.Href)
		if err != nil || next.Opaque != "" || next.User != nil {
			return ""
		}
		path := next.EscapedPath()
		if path != v3Prefix+"/domain-names" {
			return ""
		}
		if next.RawQuery != "" {
			path += "?" + next.RawQuery
		}
		return path
	}
	return ""
}

func (c *Client) updateDomainV3(ctx context.Context, domain string, update DomainUpdate) error {
	base := fmt.Sprintf("%s/domain-names/%s", v3Prefix, url.PathEscape(domain))

	if len(update.NameServers) > 0 {
		if err := c.runOperation(ctx, http.MethodPut, base+"/nameservers", update.NameServers); err != nil {
			return fmt.Errorf("nameservers: %w", err)
		}
	}

	if update.RenewAuto != nil {
		if err := c.runOperation(ctx, http.MethodPost, base+"/auto-renew", *update.RenewAuto); err != nil {
			return fmt.Errorf("auto-renew: %w", err)
		}
	}

	if update.Locked != nil {
		if err := c.runOperation(ctx, http.MethodPost, base+"/transfer-lock", *update.Locked); err != nil {
			return fmt.Errorf("transfer lock: %w", err)
		}
	}

	// v3 has no endpoint for these yet.
	if update.ExposeWhois != nil || update.SubaccountId != nil {
		v1Update := DomainUpdate{ExposeWhois: update.ExposeWhois, SubaccountId: update.SubaccountId}
		if err := c.Patch(ctx, fmt.Sprintf("/v1/domains/%s", url.PathEscape(domain)), v1Update); err != nil {
			return err
		}
	}

	return nil
}

// runOperation sends a v3 mutation that returns a DomainOperation and waits
// until GoDaddy reports it COMPLETED or FAILED.
func (c *Client) runOperation(ctx context.Context, method, path string, body interface{}) error {
	key, err := newIdempotencyKey()
	if err != nil {
		return err
	}

	resp, err := c.doRequestWithHeaders(ctx, method, path, body, map[string]string{"Idempotency-Key": key})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var op DomainOperation
	if err := json.NewDecoder(resp.Body).Decode(&op); err != nil {
		return fmt.Errorf("failed to decode operation: %w", err)
	}

	return c.waitForOperation(ctx, &op)
}

func (c *Client) waitForOperation(ctx context.Context, op *DomainOperation) error {
	deadline := time.Now().Add(c.operationTimeout)

	for {
		switch op.Status {
		case operationStatusCompleted:
			return nil
		case operationStatusConfirmed, operationStatusExecuting:
		default:
			// FAILED, or a status this client does not know. Polling an
			// unknown status would only wait out the timeout.
			if op.Error != nil {
				return fmt.Errorf("operation %s %s: %s: %s", op.OperationID, op.Status, op.Error.Name, op.Error.Message)
			}
			return fmt.Errorf("operation %s ended with status %q", op.OperationID, op.Status)
		}

		if op.OperationID == "" {
			return fmt.Errorf("operation has status %q and no operationId to poll", op.Status)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("operation %s still %s after %s", op.OperationID, op.Status, c.operationTimeout)
		}

		select {
		case <-time.After(c.operationPollInterval):
		case <-ctx.Done():
			return ctx.Err()
		}

		if err := c.Get(ctx, fmt.Sprintf("%s/operations/%s", v3Prefix, url.PathEscape(op.OperationID)), op); err != nil {
			return err
		}
	}
}

func newIdempotencyKey() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to create idempotency key: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// listDNSRecordsV3 reads every page of the zone. Empty recordType or name
// means no filter on that field.
func (c *Client) listDNSRecordsV3(ctx context.Context, domain, recordType, name string) ([]v3DNSRecord, error) {
	var records []v3DNSRecord

	for page := 1; page <= v3MaxPages; page++ {
		query := url.Values{}
		query.Set("page", strconv.Itoa(page))
		query.Set("pageSize", strconv.Itoa(v3DNSPageSize))
		if recordType != "" {
			query.Set("type", strings.ToUpper(recordType))
		}
		if name != "" {
			query.Set("name", name)
		}

		var list v3DNSRecordList
		if err := c.Get(ctx, fmt.Sprintf("%s/zones/%s/dns-records?%s", v3Prefix, url.PathEscape(domain), query.Encode()), &list); err != nil {
			return nil, err
		}

		for _, record := range list.Items {
			if recordType != "" && !strings.EqualFold(record.Type, recordType) {
				continue
			}
			if name != "" && !strings.EqualFold(record.Name, name) {
				continue
			}
			records = append(records, record)
		}

		if len(list.Items) < v3DNSPageSize {
			break
		}
	}

	return records, nil
}

func (r v3DNSRecord) toDNSRecord() DNSRecord {
	return DNSRecord{
		Type:     r.Type,
		Name:     r.Name,
		Data:     normalizeRecordData(r.Type, r.Data),
		TTL:      r.TTL,
		Priority: r.Priority,
		Port:     r.Port,
		Weight:   r.Weight,
		Service:  r.Service,
		Protocol: r.Protocol,
	}
}

func normalizeRecordData(recordType, data string) string {
	if hostnameRecordTypes[strings.ToUpper(recordType)] && data != "." {
		return strings.TrimSuffix(data, ".")
	}
	return data
}

func toV3DNSRecord(r DNSRecord) v3DNSRecord {
	return v3DNSRecord{
		Type:     strings.ToUpper(r.Type),
		Name:     r.Name,
		Data:     r.Data,
		TTL:      r.TTL,
		Priority: r.Priority,
		Port:     r.Port,
		Weight:   r.Weight,
		Service:  r.Service,
		Protocol: r.Protocol,
	}
}

func toDNSRecords(records []v3DNSRecord) []DNSRecord {
	result := make([]DNSRecord, len(records))
	for i, record := range records {
		result[i] = record.toDNSRecord()
	}
	return result
}

// getZoneRecordsWithToken lists the whole zone: v3 for the types it
// handles, v1 for the rest, so CAA and the other v1-only types come back in
// the same form as with a key.
func (c *Client) getZoneRecordsWithToken(ctx context.Context, domain string) ([]DNSRecord, error) {
	v3Records, err := c.listDNSRecordsV3(ctx, domain, "", "")
	if err != nil {
		return nil, err
	}

	var v1Records []DNSRecord
	if err := c.Get(ctx, fmt.Sprintf("/v1/domains/%s/records", url.PathEscape(domain)), &v1Records); err != nil {
		return nil, err
	}

	var result []DNSRecord
	for _, record := range v3Records {
		if v3DNSRecordTypes[strings.ToUpper(record.Type)] {
			result = append(result, record.toDNSRecord())
		}
	}
	for _, record := range v1Records {
		if !v3DNSRecordTypes[strings.ToUpper(record.Type)] {
			result = append(result, record)
		}
	}
	return result, nil
}

func (c *Client) getDNSRecordsV3(ctx context.Context, domain, recordType, name string) ([]DNSRecord, error) {
	records, err := c.listDNSRecordsV3(ctx, domain, recordType, name)
	if err != nil {
		return nil, err
	}
	return toDNSRecords(records), nil
}

func (c *Client) createDNSRecordV3(ctx context.Context, domain string, record DNSRecord) error {
	return c.Post(ctx, fmt.Sprintf("%s/zones/%s/dns-records", v3Prefix, url.PathEscape(domain)), toV3DNSRecord(record), nil)
}

func (c *Client) deleteDNSRecordsV3(ctx context.Context, domain, recordType, name string) error {
	records, err := c.listDNSRecordsV3(ctx, domain, recordType, name)
	if err != nil {
		return err
	}
	for _, record := range records {
		if err := c.Delete(ctx, fmt.Sprintf("%s/zones/%s/dns-records/%s", v3Prefix, url.PathEscape(domain), url.PathEscape(record.RecordID))); err != nil {
			return err
		}
	}
	return nil
}

// replaceDNSRecordSetV3 makes the records that match recordType (and name,
// when set) equal to desired. v3 works on single records by recordId, so this
// keeps records that already match, rewrites the rest in place, then creates
// or deletes the difference. Unlike the v1 PUT, it is not atomic.
func (c *Client) replaceDNSRecordSetV3(ctx context.Context, domain, recordType, name string, desired []DNSRecord) error {
	existing, err := c.listDNSRecordsV3(ctx, domain, recordType, name)
	if err != nil {
		return err
	}

	used := make([]bool, len(existing))
	var pending []DNSRecord
	for _, want := range desired {
		matched := false
		for i, have := range existing {
			if !used[i] && sameDNSRecord(have.toDNSRecord(), want) {
				used[i] = true
				matched = true
				break
			}
		}
		if !matched {
			pending = append(pending, want)
		}
	}

	var stale []v3DNSRecord
	for i, have := range existing {
		if !used[i] {
			stale = append(stale, have)
		}
	}

	for len(pending) > 0 && len(stale) > 0 {
		path := fmt.Sprintf("%s/zones/%s/dns-records/%s", v3Prefix, url.PathEscape(domain), url.PathEscape(stale[0].RecordID))
		if err := c.Put(ctx, path, toV3DNSRecord(pending[0])); err != nil {
			return err
		}
		pending, stale = pending[1:], stale[1:]
	}

	for _, record := range pending {
		if err := c.createDNSRecordV3(ctx, domain, record); err != nil {
			return err
		}
	}

	for _, record := range stale {
		if err := c.Delete(ctx, fmt.Sprintf("%s/zones/%s/dns-records/%s", v3Prefix, url.PathEscape(domain), url.PathEscape(record.RecordID))); err != nil {
			return err
		}
	}

	return nil
}

func sameDNSRecord(a, b DNSRecord) bool {
	return strings.EqualFold(a.Type, b.Type) &&
		strings.EqualFold(a.Name, b.Name) &&
		normalizeRecordData(a.Type, a.Data) == normalizeRecordData(b.Type, b.Data) &&
		a.TTL == b.TTL &&
		sameIntPtr(a.Priority, b.Priority) &&
		sameIntPtr(a.Port, b.Port) &&
		sameIntPtr(a.Weight, b.Weight) &&
		sameStringPtr(a.Service, b.Service) &&
		sameStringPtr(a.Protocol, b.Protocol)
}

func sameIntPtr(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func sameStringPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
