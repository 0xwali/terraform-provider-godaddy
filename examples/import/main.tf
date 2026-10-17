terraform {
  required_providers {
    godaddy = {
      source  = "bearcode33/godaddy"
      version = "~> 1.0"
    }
  }
}

# Configure the GoDaddy Provider
# API credentials should be set via environment variables:
# GODADDY_API_KEY, GODADDY_API_SECRET, GODADDY_ENVIRONMENT
provider "godaddy" {
  # Credentials are read from environment variables
  # api_key     = var.godaddy_api_key     # Alternative: use variables
  # api_secret  = var.godaddy_api_secret
  # environment = var.godaddy_environment
}

# Example domain resource (to be imported)
resource "godaddy_domain" "example" {
  domain = "example.com"  # Replace with your domain
  
  # Configure domain settings
  locked              = true
  renew_auto          = true
}

# Example DNS record resources (to be imported)
resource "godaddy_dns_record" "root_a" {
  domain = "example.com"  # Replace with your domain
  type   = "A"
  name   = "@"
  data   = "192.0.2.1"    # Replace with actual IP
  ttl    = 3600
}

resource "godaddy_dns_record" "www_a" {
  domain = "example.com"  # Replace with your domain
  type   = "A"
  name   = "www"
  data   = "192.0.2.1"    # Replace with actual IP
  ttl    = 3600
}

resource "godaddy_dns_record" "mail_mx" {
  domain   = "example.com"  # Replace with your domain
  type     = "MX"
  name     = "@"
  data     = "mail.example.com"  # Replace with actual mail server
  ttl      = 3600
  priority = 10
}

# Outputs to verify imported data
output "domain_status" {
  description = "Current domain status"
  value       = godaddy_domain.example.status
}

output "domain_expires" {
  description = "Domain expiration date"
  value       = godaddy_domain.example.expires
}