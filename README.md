# Terraform Provider for GoDaddy

[![Build Status](https://github.com/bearcode33/terraform-provider-godaddy/workflows/test/badge.svg)](https://github.com/bearcode33/terraform-provider-godaddy/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/bearcode33/terraform-provider-godaddy)](https://goreportcard.com/report/github.com/bearcode33/terraform-provider-godaddy)
[![License: MPL 2.0](https://img.shields.io/badge/License-MPL%202.0-brightgreen.svg)](https://opensource.org/licenses/MPL-2.0)

A comprehensive Terraform provider for managing GoDaddy domains and DNS records with full validation and support for all DNS record types.

## Features

- 🏗️ **Complete Domain Management**: Configure domain settings, nameservers, and contacts
- 🌐 **Full DNS Support**: All 18+ DNS record types with intelligent validation
- 📥 **Easy Import**: Import existing GoDaddy resources with automated discovery tools
- 🔒 **Security First**: Built-in validation, error handling, and secure API authentication
- 📚 **Production Ready**: Comprehensive tests, examples, and documentation
- 🚀 **Easy to Use**: Intuitive Terraform configuration with helpful error messages

### Supported DNS Record Types

- **Basic Records**: A, AAAA, CNAME, TXT, NS, PTR, SOA
- **Mail Records**: MX
- **Service Records**: SRV
- **Security Records**: CAA, SSHFP, TLSA, DS
- **Advanced Records**: NAPTR, URI, LOC, CERT, DNAME

## Quick Start

### Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.26 (for development)
- GoDaddy API credentials ([Get them here](https://developer.godaddy.com/keys))

### Installation

#### From Terraform Registry (Recommended)

```hcl
terraform {
  required_providers {
    godaddy = {
      source  = "bearcode33/godaddy"
      version = "~> 1.0"
    }
  }
}
```

#### Manual Installation

1. Download the latest release from [GitHub Releases](https://github.com/bearcode33/terraform-provider-godaddy/releases)
2. Extract to your Terraform plugins directory
3. Run `terraform init`

#### Build from Source

```bash
git clone https://github.com/bearcode33/terraform-provider-godaddy.git
cd terraform-provider-godaddy
make build
make install
```

## Configuration

### Provider Configuration

```hcl
provider "godaddy" {
  api_key     = var.godaddy_api_key    # Or set GODADDY_API_KEY
  api_secret  = var.godaddy_api_secret # Or set GODADDY_API_SECRET
  environment = "production"           # Or set GODADDY_ENVIRONMENT
}
```

### Environment Variables

Set these environment variables to avoid hardcoding credentials:

```bash
export GODADDY_API_KEY="your-api-key"
export GODADDY_API_SECRET="your-api-secret"
export GODADDY_ENVIRONMENT="production"  # or "test" for OTE
```

## Usage Examples

### Domain Management

```hcl
# Manage domain settings
resource "godaddy_domain" "example" {
  domain = "example.com"
  
  # Set only what you want to change. Unset settings keep their current value.
  locked     = true # Prevent unauthorized transfers
  renew_auto = true # Auto-renewal
  
  # Custom nameservers
  nameservers = [
    "ns1.example.com",
    "ns2.example.com"
  ]
  
  # Contact information (optional)
  contact_admin {
    name_first  = "John"
    name_last   = "Doe"
    email       = "admin@example.com"
    phone       = "+1.5555551234"
    address1    = "123 Main St"
    city        = "Anytown"
    state       = "CA"
    postal_code = "12345"
    country     = "US"
  }
}
```

### DNS Records

#### Basic Records

```hcl
# A Record - IPv4
resource "godaddy_dns_record" "root" {
  domain = "example.com"
  type   = "A"
  name   = "@"
  data   = "192.0.2.1"
  ttl    = 3600
}

# AAAA Record - IPv6
resource "godaddy_dns_record" "ipv6" {
  domain = "example.com"
  type   = "AAAA"
  name   = "@"
  data   = "2001:db8::1"
  ttl    = 3600
}

# CNAME Record - Alias
resource "godaddy_dns_record" "www" {
  domain = "example.com"
  type   = "CNAME"
  name   = "www"
  data   = "example.com"
  ttl    = 3600
}
```

#### Mail Records

```hcl
# MX Records - Mail servers
resource "godaddy_dns_record" "mx_primary" {
  domain   = "example.com"
  type     = "MX"
  name     = "@"
  data     = "mail.example.com"
  priority = 10
  ttl      = 3600
}

resource "godaddy_dns_record" "mx_backup" {
  domain   = "example.com"
  type     = "MX"
  name     = "@"
  data     = "mail2.example.com"
  priority = 20
  ttl      = 3600
}

# SPF Record
resource "godaddy_dns_record" "spf" {
  domain = "example.com"
  type   = "TXT"
  name   = "@"
  data   = "v=spf1 include:_spf.google.com ~all"
  ttl    = 3600
}

# DMARC Record
resource "godaddy_dns_record" "dmarc" {
  domain = "example.com"
  type   = "TXT"
  name   = "_dmarc"
  data   = "v=DMARC1; p=quarantine; rua=mailto:dmarc@example.com"
  ttl    = 3600
}
```

#### Service Records

```hcl
# SRV Record - Service location
resource "godaddy_dns_record" "sip" {
  domain   = "example.com"
  type     = "SRV"
  name     = "_sip._tcp"
  data     = "sipserver.example.com"
  priority = 10
  weight   = 60
  port     = 5060
  service  = "_sip"
  protocol = "_tcp"
  ttl      = 3600
}
```

#### Security Records

```hcl
# CAA Record - Certificate Authority Authorization
resource "godaddy_dns_record" "caa" {
  domain = "example.com"
  type   = "CAA"
  name   = "@"
  data   = "0 issue \"letsencrypt.org\""
  ttl    = 3600
}

# SSHFP Record - SSH fingerprint
resource "godaddy_dns_record" "sshfp" {
  domain = "example.com"
  type   = "SSHFP"
  name   = "@"
  data   = "1 1 123456789abcdef67890123456789abcdef67890"
  ttl    = 3600
}
```

### Data Sources

```hcl
# Get domain information
data "godaddy_domain" "example" {
  domain = "example.com"
}

# Get DNS records
data "godaddy_dns_records" "all" {
  domain = "example.com"
}

# Get specific record type
data "godaddy_dns_records" "mx_records" {
  domain = "example.com"
  type   = "MX"
}

# Output domain info
output "domain_expires" {
  value = data.godaddy_domain.example.expires
}

output "mx_servers" {
  value = [for record in data.godaddy_dns_records.mx_records.records : record.data]
}
```

## Resource Reference

### godaddy_domain

Manages a GoDaddy domain configuration.

#### Arguments

- `domain` (Required) - The domain name
- `locked` (Optional) - Lock domain to prevent transfers. Unset keeps the current value.
- `renew_auto` (Optional) - Enable auto-renewal. Unset keeps the current value.
- `privacy` (Optional) - WHOIS privacy. The provider cannot change it: apply fails if the value differs from the live one.
- `expiration_protected` (Optional) - Expiration protection. The provider cannot change it: apply fails if the value differs from the live one.
- `nameservers` (Optional) - List of custom nameservers
- `contact_admin` (Optional) - Administrative contact
- `contact_billing` (Optional) - Billing contact
- `contact_registrant` (Optional) - Registrant contact
- `contact_tech` (Optional) - Technical contact

#### Attributes

- `status` - Current domain status
- `expires` - Domain expiration date
- `hold_registrar` - Registrar hold status
- `transfer_protected` - Transfer protection status
- `created_at` - Domain creation date
- `modified_at` - Last modification date

### godaddy_dns_record

Manages DNS records for a GoDaddy domain with full validation.

#### Arguments

- `domain` (Required) - Domain name
- `type` (Required) - DNS record type (A, AAAA, CNAME, MX, TXT, SRV, CAA, etc.)
- `name` (Required) - Record name/subdomain (use "@" for root)
- `data` (Required) - Record value (format varies by type)
- `ttl` (Optional) - Time to live in seconds (600-2592000, default: 3600)
- `priority` (Optional) - Priority for MX, SRV, NAPTR, URI records (0-65535)
- `weight` (Optional) - Weight for SRV, URI records (0-65535)
- `port` (Optional) - Port for SRV records (1-65535)
- `service` (Optional) - Service for SRV records (must start with _)
- `protocol` (Optional) - Protocol for SRV records (must start with _)

#### Attributes

- `id` - Record identifier

### Data Sources

#### godaddy_domain

Retrieves domain information.

#### godaddy_dns_records

Retrieves DNS records with optional filtering by type and name.

## Validation and Error Handling

The provider includes comprehensive validation:

- **DNS Record Types**: Only valid types accepted
- **IP Addresses**: IPv4 and IPv6 format validation
- **Domain Names**: FQDN format validation
- **TTL Values**: Range validation (600-2592000 seconds)
- **Priority/Weight/Port**: Appropriate ranges for each field
- **Required Fields**: Context-aware field requirements per record type

Example error messages:
```
Error: Invalid DNS Record Type
DNS record type "INVALID" is not supported. Valid types are: [A, AAAA, CAA, CNAME, MX, NS, PTR, SOA, SRV, TXT, DS, SSHFP, TLSA, LOC, NAPTR, URI, CERT, DNAME]

Error: Invalid DNS Record Configuration
SRV records require both service and protocol fields
```

## Importing Existing Resources

This provider includes powerful tools to import your existing GoDaddy domains and DNS records into Terraform.

### Quick Import with Helper Script

```bash
# Discover all resources for a domain
./scripts/import-helper.sh -d example.com -t all

# List DNS records with import commands
./scripts/import-helper.sh -d example.com

# Get domain information
./scripts/import-helper.sh -d example.com -t domain
```

### Import Commands

```bash
# Import a domain
terraform import godaddy_domain.example "example.com"

# Import DNS records (simplified format - auto-detects data)
terraform import godaddy_dns_record.www_a "example.com/A/www"

# Import DNS records (full format)
terraform import godaddy_dns_record.www_a "example.com/A/www/192.0.2.1"
```

📖 **[Complete Import Guide](README-import.md)** - Detailed instructions with examples and troubleshooting

## Examples

- **Complete setup**: [examples/complete/](examples/complete/)
- **All DNS record types**: [examples/dns-record-types/](examples/dns-record-types/)
- **Domain resources**: [examples/resources/godaddy_domain/](examples/resources/godaddy_domain/)
- **DNS record resources**: [examples/resources/godaddy_dns_record/](examples/resources/godaddy_dns_record/)
- **Data sources**: [examples/data-sources/](examples/data-sources/)

## Development

### Building

```bash
# Build binary
make build

# Install locally
make install

# Run tests
make test

# Run acceptance tests (requires API credentials)
make testacc

# Generate documentation
make docs

# Format code
make fmt

# Lint code
make lint
```

### Testing

```bash
# Unit tests
go test ./internal/...

# With coverage
go test -cover ./internal/...

# Acceptance tests (requires real API credentials)
export GODADDY_API_KEY="your-test-key"
export GODADDY_API_SECRET="your-test-secret"
export GODADDY_ENVIRONMENT="test"
make testacc
```

### Project Structure

```
├── internal/
│   ├── godaddy/          # GoDaddy API client
│   └── provider/         # Terraform provider implementation
├── examples/             # Usage examples
├── docs/                # Generated documentation
├── tools/               # Development tools
└── tests/               # Additional tests
```

### Releases

The release workflow signs checksums with the GPG key in the `GPG_PRIVATE_KEY` and
`PASSPHRASE` secrets. Keep those secrets in the `release` environment, not in
repository secrets, and protect that environment (Settings > Environments > release):

- Require a reviewer.
- Allow deployments only from `main` and tags matching `v*`.

Every pull request runs the `Test` workflow: `go mod tidy -diff`, `go vet`,
`go test -race` and `govulncheck`.

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

Please ensure:
- All tests pass
- Code is properly formatted (`make fmt`)
- Documentation is updated
- Examples are provided for new features

## Troubleshooting

### Common Issues

#### Authentication Errors
```
Error: Missing API Key
The provider requires an API key to be configured.
```
**Solution**: Set `GODADDY_API_KEY` and `GODADDY_API_SECRET` environment variables or configure in provider block.

#### Rate Limiting
```
Error: API error: status 429
```
**Solution**: GoDaddy API has rate limits. Wait and retry, or implement exponential backoff.

#### Invalid Domain
```
Error: Domain not found or not accessible
```
**Solution**: Ensure the domain is registered in your GoDaddy account and API credentials have access.

### Debug Mode

Enable debug logging:
```bash
export TF_LOG=DEBUG
terraform apply
```

## Documentation

- [Supported DNS Record Types](SUPPORTED_DNS_RECORDS.md)
- [Provider Documentation](docs/)
- [API Reference](https://developer.godaddy.com/doc/endpoint/domains)

## License

This project is licensed under the Mozilla Public License v2.0 - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- HashiCorp for the Terraform Plugin Framework
- GoDaddy for their comprehensive Domains API
- The Terraform community for feedback and contributions

---

**Note**: This provider is not officially supported by GoDaddy. Use at your own discretion in production environments.