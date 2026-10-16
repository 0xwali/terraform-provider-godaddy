terraform {
  required_providers {
    godaddy = {
      source  = "bearcode33/godaddy"
      version = "~> 1.0"
    }
  }
}

# Configure the provider
provider "godaddy" {
  # API credentials can be set here or via environment variables:
  # GODADDY_API_KEY and GODADDY_API_SECRET

  # api_key    = var.godaddy_api_key
  # api_secret = var.godaddy_api_secret

  # Optional: Use "test" for OTE environment
  # environment = "production"
}

# Manage domain settings
resource "godaddy_domain" "example" {
  domain = "example.com"

  # Domain settings
  locked     = true # Prevent unauthorized transfers
  renew_auto = true # Auto-renewal

  # Custom nameservers (optional)
  nameservers = [
    "ns1.example.com",
    "ns2.example.com"
  ]
}

# DNS Records

# Root domain A record
resource "godaddy_dns_record" "root_a" {
  domain = godaddy_domain.example.domain
  type   = "A"
  name   = "@"
  data   = "192.0.2.1"
  ttl    = 3600
}

# WWW subdomain
resource "godaddy_dns_record" "www" {
  domain = godaddy_domain.example.domain
  type   = "CNAME"
  name   = "www"
  data   = "example.com"
  ttl    = 3600
}

# Mail server records
resource "godaddy_dns_record" "mx_10" {
  domain   = godaddy_domain.example.domain
  type     = "MX"
  name     = "@"
  data     = "mail.example.com"
  priority = 10
  ttl      = 3600
}

resource "godaddy_dns_record" "mx_20" {
  domain   = godaddy_domain.example.domain
  type     = "MX"
  name     = "@"
  data     = "mail2.example.com"
  priority = 20
  ttl      = 3600
}

# SPF record
resource "godaddy_dns_record" "spf" {
  domain = godaddy_domain.example.domain
  type   = "TXT"
  name   = "@"
  data   = "v=spf1 include:_spf.google.com ~all"
  ttl    = 3600
}

# DKIM record
resource "godaddy_dns_record" "dkim" {
  domain = godaddy_domain.example.domain
  type   = "TXT"
  name   = "google._domainkey"
  data   = "v=DKIM1; k=rsa; p=MIGfMA0GCSqGSIb3DQEBAQUAA4..."
  ttl    = 3600
}

# DMARC record
resource "godaddy_dns_record" "dmarc" {
  domain = godaddy_domain.example.domain
  type   = "TXT"
  name   = "_dmarc"
  data   = "v=DMARC1; p=quarantine; rua=mailto:dmarc@example.com"
  ttl    = 3600
}

# CAA record
resource "godaddy_dns_record" "caa" {
  domain = godaddy_domain.example.domain
  type   = "CAA"
  name   = "@"
  data   = "0 issue \"letsencrypt.org\""
  ttl    = 3600
}

# Data sources to read existing records
data "godaddy_dns_records" "all" {
  domain = "example.com"
}

output "all_dns_records" {
  value = data.godaddy_dns_records.all.records
}

# Read domain information
data "godaddy_domain" "info" {
  domain = "example.com"
}

output "domain_expires" {
  value = data.godaddy_domain.info.expires
}