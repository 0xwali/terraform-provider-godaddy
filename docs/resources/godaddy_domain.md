# godaddy_domain (Resource)

Manages a GoDaddy domain configuration including settings, nameservers, and contact information.

## Example Usage

### Basic Domain Management

```terraform
resource "godaddy_domain" "example" {
  domain = "example.com"
  
  # Set only what you want to change. Unset settings keep their current value.
  locked     = true
  renew_auto = true
}
```

### Complete Domain Configuration

```terraform
resource "godaddy_domain" "example" {
  domain = "example.com"
  
  # Domain settings
  locked     = true
  renew_auto = true
  
  # Custom nameservers
  nameservers = [
    "ns1.example.com",
    "ns2.example.com",
    "ns3.example.com",
    "ns4.example.com"
  ]
  
  # Contact information
  contact_admin {
    name_first   = "John"
    name_last    = "Doe"
    email        = "admin@example.com"
    phone        = "+1.5555551234"
    organization = "Example Corp"
    job_title    = "Administrator"
    
    address1     = "123 Main Street"
    address2     = "Suite 100"
    city         = "Anytown"
    state        = "CA"
    postal_code  = "12345"
    country      = "US"
  }
  
  contact_billing {
    name_first  = "Jane"
    name_last   = "Smith"
    email       = "billing@example.com"
    phone       = "+1.5555555678"
    
    address1    = "456 Finance Ave"
    city        = "Moneytown"
    state       = "NY"
    postal_code = "67890"
    country     = "US"
  }
  
  contact_registrant {
    name_first  = "Example"
    name_last   = "Corporation"
    email       = "legal@example.com"
    phone       = "+1.5555559999"
    
    address1    = "789 Corporate Blvd"
    city        = "Business City"
    state       = "TX"
    postal_code = "13579"
    country     = "US"
  }
  
  contact_tech {
    name_first  = "Tech"
    name_last   = "Support"
    email       = "tech@example.com"
    phone       = "+1.5555550000"
    
    address1    = "321 Tech Drive"
    city        = "Silicon Valley"
    state       = "CA"
    postal_code = "24680"
    country     = "US"
  }
}
```

## Schema

### Required

- `domain` (String) - The domain name to manage.

### Optional

- `locked` (Boolean) - Whether the domain is locked to prevent unauthorized transfers. No default: if unset, the provider keeps the current value.
- `renew_auto` (Boolean) - Whether the domain is set to auto-renew. No default: if unset, the provider keeps the current value.
- `privacy` (Boolean) - Whether WHOIS privacy protection is enabled. The provider reads it but cannot change it. If the config sets a value that differs from the live one, apply fails with an error. Leave it unset.
- `expiration_protected` (Boolean) - Whether the domain is protected from expiration. The provider reads it but cannot change it, the same as `privacy`. Leave it unset.
- `nameservers` (List of String) - Nameservers for the domain. If not set, the provider leaves them as they are. A list of GoDaddy nameservers (`*.domaincontrol.com`) matches whichever GoDaddy pair the account gets. See [Nameserver Changes](#nameserver-changes).
- `contact_admin` (Block, Sensitive) - Administrative contact information. See [contact block](#contact-block) below.
- `contact_billing` (Block, Sensitive) - Billing contact information. See [contact block](#contact-block) below.
- `contact_registrant` (Block, Sensitive) - Registrant contact information. See [contact block](#contact-block) below.
- `contact_tech` (Block, Sensitive) - Technical contact information. See [contact block](#contact-block) below.

### Read-Only

- `status` (String) - Current status of the domain.
- `expires` (String) - Domain expiration date in RFC3339 format.
- `hold_registrar` (Boolean) - Whether the domain has a registrar hold.
- `transfer_protected` (Boolean) - Whether the domain is protected from transfers.
- `assigned_nameservers` (List of String) - The nameservers GoDaddy reports for the domain. With GoDaddy nameservers this can differ from `nameservers`.
- `created_at` (String) - Domain creation date in RFC3339 format.
- `modified_at` (String) - Last modification date in RFC3339 format.

### Contact Block

The `contact_admin`, `contact_billing`, `contact_registrant`, and `contact_tech` blocks support:

#### Required

- `name_first` (String) - First name.
- `name_last` (String) - Last name.
- `email` (String) - Email address.
- `phone` (String) - Phone number in international format (e.g., "+1.5555551234").
- `address1` (String) - Primary address line.
- `city` (String) - City.
- `state` (String) - State or province.
- `postal_code` (String) - Postal/ZIP code.
- `country` (String) - Two-letter country code (ISO 3166-1 alpha-2).

#### Optional

- `name_middle` (String) - Middle name.
- `organization` (String) - Organization name.
- `job_title` (String) - Job title.
- `fax` (String) - Fax number.
- `address2` (String) - Secondary address line.

## Import

Domains can be imported using the domain name:

```bash
terraform import godaddy_domain.example example.com
```

## Notes

### Domain Registration

This resource manages existing domains in your GoDaddy account. It does not register new domains. To register a new domain, use the GoDaddy web interface or API directly.

### Contact Information

If contact information is not provided, the existing contact information from GoDaddy will be preserved. Partial updates are supported - you only need to specify the contact blocks you want to modify.

### Nameserver Changes

Changing nameservers may affect DNS resolution. Ensure your new nameservers are properly configured before applying changes.

GoDaddy picks the GoDaddy nameservers (hosts ending in `domaincontrol.com`, such as
`ns45.domaincontrol.com`) itself, and it can pick a new pair each time a domain moves back to
them. It also rejects hosts it does not run, such as `ns1.domaincontrol.com`, with
`422 NAMESERVER_REJECTED`. So:

- Any list of `*.domaincontrol.com` hosts in `nameservers` means "use GoDaddy's
  nameservers". When the domain already uses a GoDaddy pair, that is a match: no error and no
  diff. `assigned_nameservers` shows the pair GoDaddy picked.
- When the domain moves back to GoDaddy, the provider sends the pair from the zone's `NS`
  records at `@` (GoDaddy keeps them while the domain uses other nameservers), not the hosts
  in your list. GoDaddy then assigns its pair.
- After `terraform import`, the first plan can show `nameservers` changing from the live pair
  to your list. Applying it makes no API call; it only saves your list in the state.

Other nameservers, such as Cloudflare's, must match exactly (case, order and a trailing dot do
not matter). If GoDaddy reports different ones after the update, the apply fails with
"Nameservers Not Applied".

### Rate Limiting

GoDaddy API has rate limits. If you manage many domains, consider using the `-parallelism` flag to reduce concurrent operations:

```bash
terraform apply -parallelism=5
```

## Example Error Handling

```terraform
resource "godaddy_domain" "example" {
  domain = "example.com"
  
  # Use lifecycle to prevent accidental deletion
  lifecycle {
    prevent_destroy = true
  }
}
```