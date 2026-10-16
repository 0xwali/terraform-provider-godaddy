package godaddy

import (
	"context"
	"fmt"
	"strings"
)

func (c *Client) GetDNSRecords(ctx context.Context, domain string) ([]DNSRecord, error) {
	var result []DNSRecord
	err := c.Get(ctx, fmt.Sprintf("/v1/domains/%s/records", pathSegment(domain)), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get DNS records for domain %s: %w", domain, err)
	}
	return result, nil
}

func (c *Client) GetDNSRecordsByType(ctx context.Context, domain, recordType string) ([]DNSRecord, error) {
	var result []DNSRecord
	err := c.Get(ctx, fmt.Sprintf("/v1/domains/%s/records/%s", pathSegment(domain), pathSegment(recordType)), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get DNS records of type %s for domain %s: %w", recordType, domain, err)
	}
	return result, nil
}

func (c *Client) GetDNSRecordsByTypeAndName(ctx context.Context, domain, recordType, name string) ([]DNSRecord, error) {
	if err := requireRecordSet(recordType, name); err != nil {
		return nil, err
	}
	var result []DNSRecord
	err := c.Get(ctx, fmt.Sprintf("/v1/domains/%s/records/%s/%s", pathSegment(domain), pathSegment(recordType), pathSegment(name)), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get DNS record %s.%s for domain %s: %w", name, recordType, domain, err)
	}
	return result, nil
}

func (c *Client) ReplaceDNSRecords(ctx context.Context, domain string, records []DNSRecord) error {
	err := c.Put(ctx, fmt.Sprintf("/v1/domains/%s/records", pathSegment(domain)), records)
	if err != nil {
		return fmt.Errorf("failed to replace DNS records for domain %s: %w", domain, err)
	}
	return nil
}

func (c *Client) ReplaceDNSRecordsByType(ctx context.Context, domain, recordType string, records []DNSRecord) error {
	err := c.Put(ctx, fmt.Sprintf("/v1/domains/%s/records/%s", pathSegment(domain), pathSegment(recordType)), records)
	if err != nil {
		return fmt.Errorf("failed to replace DNS records of type %s for domain %s: %w", recordType, domain, err)
	}
	return nil
}

func (c *Client) ReplaceDNSRecordsByTypeAndName(ctx context.Context, domain, recordType, name string, records []DNSRecord) error {
	if err := requireRecordSet(recordType, name); err != nil {
		return err
	}
	err := c.Put(ctx, fmt.Sprintf("/v1/domains/%s/records/%s/%s", pathSegment(domain), pathSegment(recordType), pathSegment(name)), records)
	if err != nil {
		return fmt.Errorf("failed to replace DNS record %s.%s for domain %s: %w", name, recordType, domain, err)
	}
	return nil
}

func (c *Client) AddDNSRecord(ctx context.Context, domain string, record DNSRecord) error {
	err := c.Patch(ctx, fmt.Sprintf("/v1/domains/%s/records", pathSegment(domain)), []DNSRecord{record})
	if err != nil {
		return fmt.Errorf("failed to add DNS record to domain %s: %w", domain, err)
	}
	return nil
}

func (c *Client) DeleteDNSRecord(ctx context.Context, domain, recordType, name string) error {
	if err := requireRecordSet(recordType, name); err != nil {
		return err
	}
	err := c.Delete(ctx, fmt.Sprintf("/v1/domains/%s/records/%s/%s", pathSegment(domain), pathSegment(recordType), pathSegment(name)))
	if err != nil {
		return fmt.Errorf("failed to delete DNS record %s.%s from domain %s: %w", name, recordType, domain, err)
	}
	return nil
}

// requireRecordSet refuses an empty type or name. Either one empty would
// turn a call on one record set into a call on the whole type or zone.
func requireRecordSet(recordType, name string) error {
	if strings.TrimSpace(recordType) == "" || strings.TrimSpace(name) == "" {
		return fmt.Errorf("DNS record type and name must not be empty (got type %q, name %q); use @ for the root domain", recordType, name)
	}
	return nil
}
