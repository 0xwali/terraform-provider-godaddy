# godaddy_dns_record (Resource)

Manages DNS records for a GoDaddy domain with support for all major DNS record types and comprehensive validation.

## Example Usage

### Basic Records

```terraform
# A Record - IPv4 address
resource "godaddy_dns_record" "root" {
  domain = "example.com"
  type   = "A"
  name   = "@"
  data   = "192.0.2.1"
  ttl    = 3600
}

# AAAA Record - IPv6 address
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

### Mail Records

```terraform
# MX Records
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

# DKIM Record
resource "godaddy_dns_record" "dkim" {
  domain = "example.com"
  type   = "TXT"
  name   = "google._domainkey"
  data   = "v=DKIM1; k=rsa; p=MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQC..."
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

### Service Records

```terraform
# SRV Record - SIP service
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

# SRV Record - XMPP
resource "godaddy_dns_record" "xmpp" {
  domain   = "example.com"
  type     = "SRV"
  name     = "_xmpp-server._tcp"
  data     = "xmpp.example.com"
  priority = 5
  weight   = 0
  port     = 5269
  service  = "_xmpp-server"
  protocol = "_tcp"
  ttl      = 3600
}
```

### Security Records

```terraform
# CAA Record - Certificate Authority Authorization
resource "godaddy_dns_record" "caa_issue" {
  domain = "example.com"
  type   = "CAA"
  name   = "@"
  data   = "0 issue \"letsencrypt.org\""
  ttl    = 3600
}

resource "godaddy_dns_record" "caa_wildcard" {
  domain = "example.com"
  type   = "CAA"
  name   = "@"
  data   = "0 issuewild \"letsencrypt.org\""
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

# TLSA Record - TLS certificate association
resource "godaddy_dns_record" "tlsa" {
  domain = "example.com"
  type   = "TLSA"
  name   = "_443._tcp"
  data   = "3 1 1 abc123def456..."
  ttl    = 3600
}
```

### Conflict Resolution

```terraform
# A second TXT record at @ is added next to the existing ones.
# Nothing is deleted, and allow_overwrite is not needed.
resource "godaddy_dns_record" "site_verification" {
  domain = "example.com"
  type   = "TXT"
  name   = "@"
  data   = "google-site-verification=abc123"
}

# Take over a CNAME that already exists with other data.
resource "godaddy_dns_record" "www" {
  domain          = "example.com"
  type            = "CNAME"
  name            = "www"
  data            = "app.example.net"
  allow_overwrite = true # Replaces the existing www CNAME
}
```

### Advanced Records

```terraform
# NAPTR Record
resource "godaddy_dns_record" "naptr" {
  domain   = "example.com"
  type     = "NAPTR"
  name     = "@"
  data     = "100 10 \"u\" \"E2U+sip\" \"!^.*$!sip:info@example.com!\" ."
  priority = 100
  ttl      = 3600
}

# URI Record
resource "godaddy_dns_record" "uri" {
  domain   = "example.com"
  type     = "URI"
  name     = "_http._tcp"
  data     = "https://example.com/"
  priority = 10
  weight   = 1
  ttl      = 3600
}

# LOC Record - Geographic location
resource "godaddy_dns_record" "location" {
  domain = "example.com"
  type   = "LOC"
  name   = "@"
  data   = "37 23 30.000 N 122 02 30.000 W 10m 100m 100m 10m"
  ttl    = 3600
}
```

## Schema

### Required

- `domain` (String) - The domain name to add the record to.
- `type` (String) - The DNS record type. Supported types: A, AAAA, CAA, CNAME, MX, NS, PTR, SOA, SRV, TXT, DS, SSHFP, TLSA, LOC, NAPTR, URI, CERT, DNAME.
- `name` (String) - The subdomain name for the record. Use "@" for the root domain.
- `data` (String) - The value of the record. Format varies by record type.

### Optional

- `ttl` (Number) - Time to live in seconds. Must be between 600 and 2,592,000. Default: `3600`.
- `priority` (Number) - Priority for MX, SRV, NAPTR, and URI records. Range: 0-65535. Required for MX and SRV records.
- `weight` (Number) - Weight for SRV and URI records. Range: 0-65535. Required for SRV records.
- `port` (Number) - Port number for SRV records. Range: 1-65535. Required for SRV records.
- `service` (String) - Service name for SRV records (must start with underscore). Required for SRV records.
- `protocol` (String) - Protocol for SRV records (must start with underscore). Required for SRV records.
- `allow_overwrite` (Boolean) - Default: `false`. Controls what Create does when records already exist at the same type and name:
  - A record with the same data exists: with `false`, Create fails and asks you to import it. With `true`, the resource takes over that record and sets its TTL and other fields from the config.
  - Types that allow several records at one name (TXT, MX, A, AAAA, NS, CAA, SRV and the rest): the new record is added next to the existing ones. Existing records are never deleted.
  - Types that allow one record per name (CNAME, SOA, DNAME): with `false`, Create fails. With `true`, the existing record is replaced.

### Read-Only

- `id` (String) - The unique identifier for the DNS record.

## Supported Record Types

### Basic Records

| Type | Description | Data Format | Required Fields | Optional Fields |
|------|-------------|-------------|----------------|-----------------|
| **A** | IPv4 address | `192.0.2.1` | `data` | `ttl` |
| **AAAA** | IPv6 address | `2001:db8::1` | `data` | `ttl` |
| **CNAME** | Canonical name | `example.com` | `data` | `ttl` |
| **TXT** | Text record | `"v=spf1 include:_spf.google.com ~all"` | `data` | `ttl` |
| **NS** | Name server | `ns1.example.com` | `data` | `ttl` |
| **PTR** | Pointer | `server.example.com` | `data` | `ttl` |

### Mail Records

| Type | Description | Data Format | Required Fields | Optional Fields |
|------|-------------|-------------|----------------|-----------------|
| **MX** | Mail exchange | `mail.example.com` | `data`, `priority` | `ttl` |

### Service Records

| Type | Description | Data Format | Required Fields | Optional Fields |
|------|-------------|-------------|----------------|-----------------|
| **SRV** | Service location | `target.example.com` | `data`, `priority`, `weight`, `port`, `service`, `protocol` | `ttl` |

### Security Records

| Type | Description | Data Format | Required Fields | Optional Fields |
|------|-------------|-------------|----------------|-----------------|
| **CAA** | Certificate Authority Authorization | `0 issue "letsencrypt.org"` | `data` | `ttl` |
| **SSHFP** | SSH fingerprint | `1 1 abc123...` | `data` | `ttl` |
| **TLSA** | TLS certificate association | `3 1 1 abc123...` | `data` | `ttl` |
| **DS** | DNSSEC Delegation Signer | `12345 7 1 abc123...` | `data` | `ttl` |

### Advanced Records

| Type | Description | Data Format | Required Fields | Optional Fields |
|------|-------------|-------------|----------------|-----------------|
| **NAPTR** | Naming Authority Pointer | `100 10 "u" "E2U+sip" "!^.*$!sip:info@example.com!" .` | `data`, `priority` | `ttl` |
| **URI** | Uniform Resource Identifier | `https://example.com/` | `data`, `priority`, `weight` | `ttl` |
| **LOC** | Geographic location | `37 23 30.000 N 122 02 30.000 W 10m 100m 100m 10m` | `data` | `ttl` |
| **CERT** | Certificate | `1 0 0 MIGfMA0...` | `data` | `ttl` |
| **DNAME** | Delegation Name | `new.example.com` | `data` | `ttl` |

## Validation

The provider includes comprehensive validation:

### Record Type Validation
- Only supported record types are accepted
- Helpful error messages for invalid types

### Data Format Validation
- IPv4 addresses are validated for A records
- IPv6 addresses are validated for AAAA records
- Domain names are validated for CNAME, MX, NS records
- CAA records are validated for proper format

### Field Requirements
- Required fields are enforced based on record type
- SRV records require priority, weight, port, service, and protocol
- MX records require priority

### Value Range Validation
- TTL: 600-2,592,000 seconds
- Priority: 0-65535
- Weight: 0-65535
- Port: 1-65535

## Import

DNS records can be imported using the format `domain/type/name/data`:

```bash
terraform import godaddy_dns_record.example "example.com/A/@/192.0.2.1"
terraform import godaddy_dns_record.www "example.com/CNAME/www/example.com"
terraform import godaddy_dns_record.mx "example.com/MX/@/mail.example.com"
```

## Notes

### Record Name Conventions
- Use "@" for the root domain
- Subdomains are specified as just the subdomain part (e.g., "www", "mail")
- SRV records use the full service format (e.g., "_sip._tcp")

### TTL Considerations
- Lower TTL values provide faster propagation but increase DNS query load
- Higher TTL values reduce DNS queries but slow down changes
- Default TTL of 3600 seconds (1 hour) is appropriate for most use cases

### Multiple Records
- Multiple records of the same type and name are supported (e.g., multiple MX records)
- Each record is managed as a separate Terraform resource

### Record Conflicts and Overwriting
- Create never deletes records of a type that allows several records at one name. A new TXT or MX record is added next to the existing ones.
- Creating a record whose data already exists fails unless `allow_overwrite = true`. Then the resource takes over the existing record.
- For CNAME, SOA and DNAME, a name holds one record. `allow_overwrite = true` replaces it. Use it with care: it changes live DNS.

### Changes Made Outside Terraform
- If the record's data changes outside Terraform, the next refresh shows it:
  - For CNAME, SOA and DNAME, the plan shows the new live value, and apply sets it back.
  - For other types, the resource is removed from state, and the next apply creates it again. Other records at the same name are left alone.

### Names
- `name` must be `@` or a name relative to the domain made of letters, digits, `_`, `-`, `*` and `.`. It cannot start with `.` or contain `..`, `/`, `?`, `#`, `%` or spaces.
- `domain` must be a hostname such as `example.com`.

### DNS Propagation
- DNS changes may take time to propagate globally
- Use online DNS propagation checkers to verify changes

## Example Error Messages

```
Error: Invalid DNS Record Type
DNS record type "INVALID" is not supported. Valid types are: [A, AAAA, CAA, CNAME, MX, NS, PTR, SOA, SRV, TXT, DS, SSHFP, TLSA, LOC, NAPTR, URI, CERT, DNAME]

Error: Invalid DNS Record Configuration
SRV records require both service and protocol fields

Error: Invalid TTL
TTL must be at least 600 seconds (10 minutes)

Error: Invalid Priority
Priority must be between 0 and 65535

Error: DNS Record Already Exists
a CNAME record "www" already exists with data [old.example.com], and a name can hold only one CNAME record. Import it with terraform import, or set allow_overwrite = true to replace it
```