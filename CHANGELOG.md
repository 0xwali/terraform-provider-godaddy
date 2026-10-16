# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `api_token` provider setting (or `GODADDY_API_TOKEN`) for GoDaddy Personal Access Tokens.
  With a token, the provider uses the v3 Domains API with Bearer auth: domain reads,
  nameserver, auto-renew and transfer-lock updates (it waits for the async operation), and
  DNS records of type A, AAAA, CNAME, MX, SRV and TXT. Calls that v3 does not cover yet
  (contacts, other record types, TTLs above 86400, contact and protection fields on reads)
  go to v1 with the same token.

- `assigned_nameservers` on `godaddy_domain`: the nameservers GoDaddy reports.

### Fixed
- Not-found checks match the HTTP status, not the text "404" anywhere in the error.
- `godaddy_domain` create applied nothing: it read the live values over the plan before
  applying, so Terraform failed with "inconsistent result after apply".
- `godaddy_domain` update panicked ("attr.Value is nil") when a contact block was not set.
- `godaddy_domain` showed a change on every plan: unset contact blocks and the read-only
  fields came back as "known after apply". They now keep their last value.
- GoDaddy nameservers: asking for any `*.domaincontrol.com` hosts no longer fails when GoDaddy
  assigns the account's own pair. Other nameservers are compared without case, order or a
  trailing dot, and a mismatch after apply is a clear "Nameservers Not Applied" error.

### Unchanged
- `api_key` and `api_secret` work as before, on v1 with `sso-key` auth. Provider-block
  values beat environment variables. A token and a key set at the same level is an error.

## [1.0.0] - 2025-07-23

### Added
- Initial release of the GoDaddy Terraform Provider
- Complete GoDaddy API client implementation with authentication
- Domain resource for managing domain settings (locked, privacy, auto-renewal, nameservers)
- DNS record resource supporting all 18+ DNS record types:
  - A, AAAA, CAA, CNAME, MX, NS, PTR, SOA, SRV, TXT
  - DS, SSHFP, TLSA, LOC, NAPTR, URI, CERT, DNAME
- Data sources for querying domains and DNS records
- Comprehensive validation for all DNS record types
- Custom validators for TTL, priority, weight, and port values
- Support for both production and test (OTE) environments
- Complete documentation and examples
- Terraform Plugin Framework v1.15 implementation
- Import/export functionality for existing resources

### Features
- **Domain Management**: Lock/unlock domains, configure privacy settings, manage auto-renewal
- **DNS Management**: Full support for all DNS record types with proper validation
- **Environment Support**: Switch between production and test environments
- **Authentication**: API key/secret authentication with environment variable support
- **Validation**: Type-specific validation for DNS records (e.g., IP addresses, domains, emails)
- **Error Handling**: Comprehensive error handling with helpful messages

### Documentation
- Complete provider documentation with authentication setup
- Resource and data source documentation with all attributes
- Extensive examples for all DNS record types
- Development guide and contribution instructions
- Terraform Registry manifest for publication

### Examples
- Basic domain and DNS record management
- Complete setup with multiple record types
- Advanced configurations with custom nameservers
- Email security setup (SPF, DKIM, DMARC)
- SSL certificate configuration (CAA records)
