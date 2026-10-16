package godaddy

import (
	"context"
	"fmt"
)

func (c *Client) GetDNSRecords(ctx context.Context, domain string) ([]DNSRecord, error) {
	if c.UsesAPIToken() {
		if err := checkPathValues(map[string]string{"domain": domain}); err != nil {
			return nil, err
		}
		result, err := c.getZoneRecordsWithToken(ctx, domain)
		if err != nil {
			return nil, fmt.Errorf("failed to get DNS records for domain %s: %w", domain, err)
		}
		return result, nil
	}

	var result []DNSRecord
	err := c.Get(ctx, fmt.Sprintf("/v1/domains/%s/records", domain), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get DNS records for domain %s: %w", domain, err)
	}
	return result, nil
}

func (c *Client) GetDNSRecordsByType(ctx context.Context, domain, recordType string) ([]DNSRecord, error) {
	if c.useV3DNS(recordType) {
		if err := checkPathValues(map[string]string{"domain": domain, "record type": recordType}); err != nil {
			return nil, err
		}
		result, err := c.getDNSRecordsV3(ctx, domain, recordType, "")
		if err != nil {
			return nil, fmt.Errorf("failed to get DNS records of type %s for domain %s: %w", recordType, domain, err)
		}
		return result, nil
	}

	var result []DNSRecord
	err := c.Get(ctx, fmt.Sprintf("/v1/domains/%s/records/%s", domain, recordType), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get DNS records of type %s for domain %s: %w", recordType, domain, err)
	}
	return result, nil
}

func (c *Client) GetDNSRecordsByTypeAndName(ctx context.Context, domain, recordType, name string) ([]DNSRecord, error) {
	if c.useV3DNS(recordType) {
		if err := checkPathValues(map[string]string{"domain": domain, "record type": recordType, "record name": name}); err != nil {
			return nil, err
		}
		result, err := c.getDNSRecordsV3(ctx, domain, recordType, name)
		if err != nil {
			return nil, fmt.Errorf("failed to get DNS record %s.%s for domain %s: %w", name, recordType, domain, err)
		}
		return result, nil
	}

	var result []DNSRecord
	err := c.Get(ctx, fmt.Sprintf("/v1/domains/%s/records/%s/%s", domain, recordType, name), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get DNS record %s.%s for domain %s: %w", name, recordType, domain, err)
	}
	return result, nil
}

func (c *Client) ReplaceDNSRecords(ctx context.Context, domain string, records []DNSRecord) error {
	// Whole-zone replace has no v3 endpoint. With a PAT this goes to v1,
	// which accepts the token too.
	err := c.Put(ctx, fmt.Sprintf("/v1/domains/%s/records", domain), records)
	if err != nil {
		return fmt.Errorf("failed to replace DNS records for domain %s: %w", domain, err)
	}
	return nil
}

func (c *Client) ReplaceDNSRecordsByType(ctx context.Context, domain, recordType string, records []DNSRecord) error {
	if c.useV3DNSWrite(recordType, records...) {
		if err := checkPathValues(map[string]string{"domain": domain, "record type": recordType}); err != nil {
			return err
		}
		if err := c.replaceDNSRecordSetV3(ctx, domain, recordType, "", records); err != nil {
			return fmt.Errorf("failed to replace DNS records of type %s for domain %s: %w", recordType, domain, err)
		}
		return nil
	}

	err := c.Put(ctx, fmt.Sprintf("/v1/domains/%s/records/%s", domain, recordType), records)
	if err != nil {
		return fmt.Errorf("failed to replace DNS records of type %s for domain %s: %w", recordType, domain, err)
	}
	return nil
}

func (c *Client) ReplaceDNSRecordsByTypeAndName(ctx context.Context, domain, recordType, name string, records []DNSRecord) error {
	if c.useV3DNSWrite(recordType, records...) {
		if err := checkPathValues(map[string]string{"domain": domain, "record type": recordType, "record name": name}); err != nil {
			return err
		}
		if err := c.replaceDNSRecordSetV3(ctx, domain, recordType, name, records); err != nil {
			return fmt.Errorf("failed to replace DNS record %s.%s for domain %s: %w", name, recordType, domain, err)
		}
		return nil
	}

	err := c.Put(ctx, fmt.Sprintf("/v1/domains/%s/records/%s/%s", domain, recordType, name), records)
	if err != nil {
		return fmt.Errorf("failed to replace DNS record %s.%s for domain %s: %w", name, recordType, domain, err)
	}
	return nil
}

func (c *Client) AddDNSRecord(ctx context.Context, domain string, record DNSRecord) error {
	if c.useV3DNSWrite(record.Type, record) {
		if err := checkPathValues(map[string]string{"domain": domain}); err != nil {
			return err
		}
		if err := c.createDNSRecordV3(ctx, domain, record); err != nil {
			return fmt.Errorf("failed to add DNS record to domain %s: %w", domain, err)
		}
		return nil
	}

	err := c.Patch(ctx, fmt.Sprintf("/v1/domains/%s/records", domain), []DNSRecord{record})
	if err != nil {
		return fmt.Errorf("failed to add DNS record to domain %s: %w", domain, err)
	}
	return nil
}

func (c *Client) DeleteDNSRecord(ctx context.Context, domain, recordType, name string) error {
	if c.useV3DNS(recordType) {
		if err := checkPathValues(map[string]string{"domain": domain, "record type": recordType, "record name": name}); err != nil {
			return err
		}
		if err := c.deleteDNSRecordsV3(ctx, domain, recordType, name); err != nil {
			return fmt.Errorf("failed to delete DNS record %s.%s from domain %s: %w", name, recordType, domain, err)
		}
		return nil
	}

	err := c.Delete(ctx, fmt.Sprintf("/v1/domains/%s/records/%s/%s", domain, recordType, name))
	if err != nil {
		return fmt.Errorf("failed to delete DNS record %s.%s from domain %s: %w", name, recordType, domain, err)
	}
	return nil
}
