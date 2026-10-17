# GoDaddy Provider

The GoDaddy provider allows you to manage GoDaddy domains and DNS records using Terraform.

## Example Usage

```terraform
terraform {
  required_providers {
    godaddy = {
      source  = "bearcode33/godaddy"
      version = "~> 1.0"
    }
  }
}

# Configure the GoDaddy Provider
provider "godaddy" {
  api_token   = var.godaddy_api_token
  environment = "production"
}

# Manage a domain
resource "godaddy_domain" "example" {
  domain     = "example.com"
  locked     = true
  renew_auto = true
}

# Create DNS records
resource "godaddy_dns_record" "www" {
  domain = godaddy_domain.example.domain
  type   = "CNAME"
  name   = "www"
  data   = "example.com"
  ttl    = 3600
}
```

## Authentication

The GoDaddy provider takes one of two credentials:

- **Personal Access Token (PAT)**, set with `api_token`. GoDaddy recommends it. The provider
  sends it as a Bearer token and uses the v3 Domains API.
- **Classic API key and secret**, set with `api_key` and `api_secret`. This works the same as
  before, on the v1 API. GoDaddy has deprecated it for Domains (through 2026), and it does not
  work with v3.

Values in the provider block beat environment variables. Setting `api_token` and
`api_key`/`api_secret` together in the provider block is an error, and so is setting
`GODADDY_API_TOKEN` together with `GODADDY_API_KEY`/`GODADDY_API_SECRET`. That way a
stray variable cannot switch Terraform to another GoDaddy account.

### API Credentials

You can create a Personal Access Token, or a classic key and secret, in the
[GoDaddy Developer Portal](https://developer.godaddy.com/keys).

### Personal Access Token scopes

| What you manage | Scopes |
|---|---|
| Read domains (`godaddy_domain` data source, plan, import) | `domains.domain:read` |
| `nameservers` on `godaddy_domain` | `domains.nameserver:update` |
| `renew_auto` and `locked` on `godaddy_domain` | `domains.domain:update` |
| `contact_*` blocks on `godaddy_domain` | `domains.contact:update` |
| `godaddy_dns_record` and the `godaddy_dns_records` data source | `domains.domain:read`, plus `domains.dns:update` to change records |

Anything that reads a domain or its records needs `domains.domain:read`. A token without
a scope gets `401 Unauthorized` with an empty body on the calls that need it (tested on
live GoDaddy; its docs say 403). The provider adds a hint about scopes to that error.
Scope names are from [GoDaddy's auth docs](https://developer.godaddy.com/docs/api-users/auth).

Only set what the token may change. Unset `locked` and `renew_auto` keep the domain's
current values, so a token with only `domains.domain:read` and `domains.nameserver:update`
can manage nameservers:

```terraform
resource "godaddy_domain" "example" {
  domain      = "example.com"
  nameservers = ["ns1.example-dns.com", "ns2.example-dns.com"]
}
```

### Which API the token uses

With a token, the provider calls the v3 Domains API wherever v3 has the endpoint. v3 does
not cover everything yet, so a few calls still go to v1. GoDaddy's docs say v1 accepts a
PAT as well.

| Operation | API with a token |
|---|---|
| Read a domain | v3, plus a v1 read for contacts and the protection flags that v3 does not return |
| Update nameservers, auto-renew, transfer lock | v3 (waits for the async operation to finish) |
| Update contacts | v1 |
| DNS records of type A, AAAA, CNAME, MX, SRV, TXT | v3 |
| DNS records of other types (CAA, NS, SOA, SSHFP, TLSA, ...) | v1 |
| DNS records with a TTL above 86400 (v3's maximum) | v1 |
| List every record in a zone | v3 for the types above, v1 for the rest |

v3 changes DNS records one at a time by record ID. When the provider replaces a group of
records of one type and name, it makes several calls, so the change is not atomic the way
the v1 replace is.

A domain read with a token always makes a v1 call too. If GoDaddy ever stops taking a PAT
on v1, domain reads fail with a v1 error.

### Configuration Methods

#### Method 1: Environment Variables (Recommended)

Set one of the two credential types, never both.

With a Personal Access Token:

```bash
export GODADDY_API_TOKEN="your-pat"
export GODADDY_ENVIRONMENT="production"  # or "test"
```

Or with the classic key and secret:

```bash
export GODADDY_API_KEY="your-api-key"
export GODADDY_API_SECRET="your-api-secret"
export GODADDY_ENVIRONMENT="production"  # or "test"
```

#### Method 2: Provider Configuration

Use one of the two blocks below, never both.

With a Personal Access Token:

```terraform
provider "godaddy" {
  api_token   = "your-pat"
  environment = "production"
}
```

Or with the classic key and secret:

```terraform
provider "godaddy" {
  api_key     = "your-api-key"
  api_secret  = "your-api-secret"
  environment = "production"
}
```

#### Method 3: Variables (Recommended for Terraform)

```terraform
variable "godaddy_api_token" {
  description = "GoDaddy Personal Access Token"
  type        = string
  sensitive   = true
}

provider "godaddy" {
  api_token = var.godaddy_api_token
}
```

Or with the classic key and secret:

```terraform
variable "godaddy_api_key" {
  description = "GoDaddy API Key"
  type        = string
  sensitive   = true
}

variable "godaddy_api_secret" {
  description = "GoDaddy API Secret"
  type        = string
  sensitive   = true
}

provider "godaddy" {
  api_key    = var.godaddy_api_key
  api_secret = var.godaddy_api_secret
}
```

## Schema

### Credentials

Set `api_token`, or set both `api_key` and `api_secret`.

- `api_token` (String, Sensitive) - GoDaddy Personal Access Token. When set, the provider uses the v3 Domains API with Bearer auth, and `api_key` and `api_secret` are not needed. Can also be set via `GODADDY_API_TOKEN` environment variable.
- `api_key` (String, Sensitive) - GoDaddy API key. Can also be set via `GODADDY_API_KEY` environment variable.
- `api_secret` (String, Sensitive) - GoDaddy API secret. Can also be set via `GODADDY_API_SECRET` environment variable.

### Optional

- `environment` (String) - GoDaddy API environment. Valid values are `production` (default) and `test`, in any case. Any other value is an error, so a typo never sends changes to production. Can also be set via `GODADDY_ENVIRONMENT` environment variable.

## Environment Support

### Production Environment

The default environment for live domains and DNS management.

```terraform
provider "godaddy" {
  environment = "production"  # Default
}
```

### Test Environment (OTE)

GoDaddy's Operational Test Environment for development and testing.

```terraform
provider "godaddy" {
  environment = "test"
}
```

## Resources

- [godaddy_domain](resources/godaddy_domain) - Manage domain configuration
- [godaddy_dns_record](resources/godaddy_dns_record) - Manage DNS records

## Data Sources

- [godaddy_domain](data-sources/godaddy_domain) - Get domain information
- [godaddy_dns_records](data-sources/godaddy_dns_records) - Get DNS records

## Import

Resources can be imported using their respective identifiers. See individual resource documentation for import syntax.

## Rate Limiting

The GoDaddy API has rate limits. If you encounter rate limit errors (HTTP 429), consider:

1. Adding delays between operations
2. Reducing concurrency with `-parallelism` flag
3. Implementing retry logic in your CI/CD pipeline

## Error Handling

The provider includes comprehensive error handling and validation:

- API errors are properly surfaced with helpful messages
- Input validation prevents invalid configurations
- Network errors are handled with appropriate retry logic
- Resource state is properly managed on failures