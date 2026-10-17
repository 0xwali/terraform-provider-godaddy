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
- A 401 or 403 on a token call adds a hint about missing scopes. GoDaddy answers a call outside
  the token's scopes with 401 and an empty body.

### Security
- DNS Create no longer deletes records. For types that allow several records at one name
  (TXT, MX, A and the rest), a new record is added next to the existing ones, with or
  without `allow_overwrite`. Before, `allow_overwrite = true` replaced the whole set, so
  adding a TXT record could delete an SPF record.
- DNS read, create, update and delete on one domain, type and name now run one at a time,
  so parallel resources on the same set cannot overwrite each other's records.
- DNS refresh no longer hides changes made outside Terraform. See "Changed" below.
- `domain` and `name` are validated, and every value placed in an API URL is escaped. A
  name such as `../other.com` could reach another domain's API path before.
- The client refuses HTTP redirects. Go's default would resend the Authorization header.
- The wait asked for by a 429 response is capped at 60 seconds. Before, a server value
  could make the provider sleep for years.
- Error bodies are read up to 64 KiB. Rate-limit logs go to the Terraform log, not stdout.
- The domain transfer code (`authCode`) is no longer read into memory.
- Contact blocks on `godaddy_domain` are marked sensitive (personal data).
- `environment` must be `production` or `test`. Any other value is an error. Before, a
  typo such as `staging` sent changes to production.
- Release workflow: the manual tag input is validated and passed through an env var
  (it was pasted into a shell script), GoReleaser is pinned to v2.18.3, the job runs in
  a `release` environment, and the release fails on an untidy `go.mod` instead of
  changing it. New `Test` workflow on every PR: vet, race tests, govulncheck.
- Go 1.26 and dependency updates (`golang.org/x/net`, `golang.org/x/text`,
  `google.golang.org/grpc`) fix the vulnerabilities govulncheck reported.
- `scripts/import-helper.sh` sends credentials to curl on stdin, not on the command line,
  and escapes API data written into generated `.tf` files.
- `scripts/security-check.sh` no longer contains fragments of a specific key.

### Changed
- `godaddy_domain`: `locked`, `renew_auto`, `privacy` and `expiration_protected` have no
  default. Unset means "keep the current value". Before, unset meant `locked = true`,
  `renew_auto = true` and `privacy = false`, so an apply could lock a domain during a
  transfer or turn on auto-renewal (and its charge). If you relied on those defaults,
  set the values in your config.
- `godaddy_domain`: the provider cannot change `privacy` or `expiration_protected`. If
  the config asks for a value that differs from the live one, the update now fails with
  an error. Before, the change was ignored and the plan showed it forever.
- `godaddy_dns_record`: when no live record matches the stored data, a CNAME, SOA or
  DNAME record shows its live value in the plan. Other types are removed from state and
  created again on the next apply. Before, the old state was kept.
- `api_key` and `api_secret` are trimmed.
- Building from source needs Go 1.26 or later.

### Fixed
- Not-found checks match the HTTP status, not the text "404" anywhere in the error.
- `godaddy_domain` create applied nothing: it read the live values over the plan before
  applying, so Terraform failed with "inconsistent result after apply".
- `godaddy_domain` update panicked ("attr.Value is nil") when a contact block was not set.
- `godaddy_domain` showed a change on every plan: unset contact blocks and the read-only
  fields came back as "known after apply". They now keep their last value.
- GoDaddy nameservers: asking for any `*.domaincontrol.com` hosts no longer fails when GoDaddy
  assigns the account's own pair. Moving a domain back to GoDaddy with hosts GoDaddy does not
  run (`ns1.domaincontrol.com` gets `422 NAMESERVER_REJECTED`) now sends the pair from the
  zone's `NS` records instead. Other nameservers are compared without case, order or a
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
