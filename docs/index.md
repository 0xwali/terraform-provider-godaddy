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
  api_key     = var.godaddy_api_key
  api_secret  = var.godaddy_api_secret
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

The GoDaddy provider requires API credentials to authenticate with the GoDaddy API.

### API Credentials

You can obtain your API credentials from the [GoDaddy Developer Portal](https://developer.godaddy.com/keys).

### Configuration Methods

#### Method 1: Environment Variables (Recommended)

```bash
export GODADDY_API_KEY="your-api-key"
export GODADDY_API_SECRET="your-api-secret"
export GODADDY_ENVIRONMENT="production"  # or "test"
```

#### Method 2: Provider Configuration

```terraform
provider "godaddy" {
  api_key     = "your-api-key"
  api_secret  = "your-api-secret"
  environment = "production"
}
```

#### Method 3: Variables (Recommended for Terraform)

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

### Required

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