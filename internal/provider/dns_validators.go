package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/bearcode33/terraform-provider-godaddy/internal/godaddy"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// dnsRecordTypeValidator validates DNS record types
type dnsRecordTypeValidator struct{}

func (d dnsRecordTypeValidator) Description(ctx context.Context) string {
	return "validates DNS record type"
}

func (d dnsRecordTypeValidator) MarkdownDescription(ctx context.Context) string {
	return "Validates that the DNS record type is supported by GoDaddy"
}

func (d dnsRecordTypeValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	recordType := req.ConfigValue.ValueString()
	validTypes := godaddy.ValidDNSRecordTypes()

	for _, validType := range validTypes {
		if recordType == validType {
			return
		}
	}

	resp.Diagnostics.AddAttributeError(
		req.Path,
		"Invalid DNS Record Type",
		fmt.Sprintf("DNS record type %q is not supported. Valid types are: %v", recordType, validTypes),
	)
}

func DNSRecordTypeValidator() validator.String {
	return dnsRecordTypeValidator{}
}

// dnsRecordDataValidator validates DNS record data based on type
type dnsRecordDataValidator struct{}

func (d dnsRecordDataValidator) Description(ctx context.Context) string {
	return "validates DNS record data based on record type"
}

func (d dnsRecordDataValidator) MarkdownDescription(ctx context.Context) string {
	return "Validates that the DNS record data is valid for the specified record type"
}

func (d dnsRecordDataValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	data := req.ConfigValue.ValueString()

	// We need to get the record type from the configuration
	// This is a simplified validation - in a real implementation, you'd access the full config
	// For now, we'll do basic validation
	if len(data) == 0 {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid DNS Record Data",
			"DNS record data cannot be empty",
		)
	}

	if len(data) > 65535 {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid DNS Record Data",
			"DNS record data too long (maximum 65535 characters)",
		)
	}
}

func DNSRecordDataValidator() validator.String {
	return dnsRecordDataValidator{}
}

// ttlValidator validates TTL values
type ttlValidator struct{}

func (t ttlValidator) Description(ctx context.Context) string {
	return "validates TTL value"
}

func (t ttlValidator) MarkdownDescription(ctx context.Context) string {
	return "Validates that the TTL value is within acceptable range (600-2592000 seconds)"
}

func (t ttlValidator) ValidateInt32(ctx context.Context, req validator.Int32Request, resp *validator.Int32Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	ttl := req.ConfigValue.ValueInt32()

	if ttl < 600 {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid TTL",
			"TTL must be at least 600 seconds (10 minutes)",
		)
	}

	if ttl > 2592000 {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid TTL",
			"TTL cannot exceed 2592000 seconds (30 days)",
		)
	}
}

func TTLValidator() validator.Int32 {
	return ttlValidator{}
}

// priorityValidator validates priority values for MX and SRV records
type priorityValidator struct{}

func (p priorityValidator) Description(ctx context.Context) string {
	return "validates priority value"
}

func (p priorityValidator) MarkdownDescription(ctx context.Context) string {
	return "Validates that the priority value is within acceptable range (0-65535)"
}

func (p priorityValidator) ValidateInt32(ctx context.Context, req validator.Int32Request, resp *validator.Int32Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	priority := req.ConfigValue.ValueInt32()

	if priority < 0 || priority > 65535 {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Priority",
			"Priority must be between 0 and 65535",
		)
	}
}

func PriorityValidator() validator.Int32 {
	return priorityValidator{}
}

// portValidator validates port values for SRV records
type portValidator struct{}

func (p portValidator) Description(ctx context.Context) string {
	return "validates port value"
}

func (p portValidator) MarkdownDescription(ctx context.Context) string {
	return "Validates that the port value is within acceptable range (1-65535)"
}

func (p portValidator) ValidateInt32(ctx context.Context, req validator.Int32Request, resp *validator.Int32Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	port := req.ConfigValue.ValueInt32()

	if port < 1 || port > 65535 {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Port",
			"Port must be between 1 and 65535",
		)
	}
}

func PortValidator() validator.Int32 {
	return portValidator{}
}

// weightValidator validates weight values for SRV records
type weightValidator struct{}

func (w weightValidator) Description(ctx context.Context) string {
	return "validates weight value"
}

func (w weightValidator) MarkdownDescription(ctx context.Context) string {
	return "Validates that the weight value is within acceptable range (0-65535)"
}

func (w weightValidator) ValidateInt32(ctx context.Context, req validator.Int32Request, resp *validator.Int32Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	weight := req.ConfigValue.ValueInt32()

	if weight < 0 || weight > 65535 {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Weight",
			"Weight must be between 0 and 65535",
		)
	}
}

func WeightValidator() validator.Int32 {
	return weightValidator{}
}

var (
	hostnameLabel  = `[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?`
	domainNameRe   = regexp.MustCompile(`^` + hostnameLabel + `(?:\.` + hostnameLabel + `)+$`)
	recordNameRe   = regexp.MustCompile(`^[a-zA-Z0-9_*.-]+$`)
	maxDomainChars = 253
)

// ValidateDomainName checks that domain is a plain hostname such as
// example.com. The value goes into API URL paths, so anything else is
// rejected before a request is made.
func ValidateDomainName(domain string) error {
	if len(domain) > maxDomainChars || !domainNameRe.MatchString(domain) {
		return fmt.Errorf("%q is not a valid domain name (expected a hostname such as example.com)", domain)
	}
	return nil
}

// ValidateRecordName checks a DNS record name relative to the domain: "@"
// for the apex, or labels made of letters, digits, '_', '-', '*' and '.'.
// The value goes into API URL paths, so '/', '?', '#', '%', spaces and ".."
// are rejected.
func ValidateRecordName(name string) error {
	switch {
	case name == "":
		return fmt.Errorf("record name cannot be empty; use @ for the root domain")
	case name == "@":
		return nil
	case strings.HasPrefix(name, "."):
		return fmt.Errorf("record name %q cannot start with '.'", name)
	case strings.Contains(name, ".."):
		return fmt.Errorf("record name %q cannot contain '..'", name)
	case !recordNameRe.MatchString(name):
		return fmt.Errorf("record name %q may only contain letters, digits, '_', '-', '*' and '.', or be @", name)
	}
	return nil
}

type domainNameValidator struct{}

func (v domainNameValidator) Description(ctx context.Context) string {
	return "validates a domain name"
}

func (v domainNameValidator) MarkdownDescription(ctx context.Context) string {
	return "Validates that the value is a hostname such as `example.com`"
}

func (v domainNameValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if err := ValidateDomainName(req.ConfigValue.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid Domain Name", err.Error())
	}
}

func DomainNameValidator() validator.String {
	return domainNameValidator{}
}

type recordNameValidator struct{}

func (v recordNameValidator) Description(ctx context.Context) string {
	return "validates a DNS record name"
}

func (v recordNameValidator) MarkdownDescription(ctx context.Context) string {
	return "Validates that the value is `@` or a DNS name relative to the domain"
}

func (v recordNameValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if err := ValidateRecordName(req.ConfigValue.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid Record Name", err.Error())
	}
}

func DNSRecordNameValidator() validator.String {
	return recordNameValidator{}
}
