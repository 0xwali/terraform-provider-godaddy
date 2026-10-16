package godaddy

import (
	"context"
	"fmt"
	"net/url"
)

func (c *Client) GetDomain(ctx context.Context, domain string) (*DomainDetail, error) {
	var result DomainDetail
	err := c.Get(ctx, fmt.Sprintf("/v1/domains/%s", pathSegment(domain)), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get domain %s: %w", domain, err)
	}
	return &result, nil
}

func (c *Client) ListDomains(ctx context.Context) ([]Domain, error) {
	var result []Domain
	err := c.Get(ctx, "/v1/domains", &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list domains: %w", err)
	}
	return result, nil
}

func (c *Client) UpdateDomain(ctx context.Context, domain string, update DomainUpdate) error {
	err := c.Patch(ctx, fmt.Sprintf("/v1/domains/%s", pathSegment(domain)), update)
	if err != nil {
		return fmt.Errorf("failed to update domain %s: %w", domain, err)
	}
	return nil
}

func (c *Client) CheckDomainAvailability(ctx context.Context, domain string) (*DomainAvailability, error) {
	var result DomainAvailability
	err := c.Get(ctx, "/v1/domains/available?"+url.Values{"domain": {domain}}.Encode(), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to check domain availability for %s: %w", domain, err)
	}
	return &result, nil
}

func (c *Client) PurchaseDomain(ctx context.Context, purchase DomainPurchase) error {
	err := c.Post(ctx, "/v1/domains/purchase", purchase, nil)
	if err != nil {
		return fmt.Errorf("failed to purchase domain %s: %w", purchase.Domain, err)
	}
	return nil
}

func (c *Client) DeleteDomain(ctx context.Context, domain string) error {
	err := c.Delete(ctx, fmt.Sprintf("/v1/domains/%s", pathSegment(domain)))
	if err != nil {
		return fmt.Errorf("failed to delete domain %s: %w", domain, err)
	}
	return nil
}

func (c *Client) GetDomainContacts(ctx context.Context, domain string) (*DomainContactsResponse, error) {
	var result DomainContactsResponse
	err := c.Get(ctx, fmt.Sprintf("/v1/domains/%s/contacts", pathSegment(domain)), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get domain contacts for %s: %w", domain, err)
	}
	return &result, nil
}

func (c *Client) UpdateDomainContacts(ctx context.Context, domain string, contacts DomainContacts) error {
	err := c.Patch(ctx, fmt.Sprintf("/v1/domains/%s/contacts", pathSegment(domain)), contacts)
	if err != nil {
		return fmt.Errorf("failed to update domain contacts for %s: %w", domain, err)
	}
	return nil
}

type DomainContactsResponse struct {
	ContactAdmin      DomainContact `json:"contactAdmin"`
	ContactBilling    DomainContact `json:"contactBilling"`
	ContactRegistrant DomainContact `json:"contactRegistrant"`
	ContactTech       DomainContact `json:"contactTech"`
}

type DomainContacts struct {
	ContactAdmin      *DomainContact `json:"contactAdmin,omitempty"`
	ContactBilling    *DomainContact `json:"contactBilling,omitempty"`
	ContactRegistrant *DomainContact `json:"contactRegistrant,omitempty"`
	ContactTech       *DomainContact `json:"contactTech,omitempty"`
}
