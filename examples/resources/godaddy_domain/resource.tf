resource "godaddy_domain" "example" {
  domain     = "example.com"
  locked     = true
  renew_auto = true

  nameservers = [
    "ns1.domaincontrol.com",
    "ns2.domaincontrol.com"
  ]

  # Contact information (optional - will use existing if not specified)
  contact_admin {
    name_first  = "John"
    name_last   = "Doe"
    email       = "john.doe@example.com"
    phone       = "+1.5555551234"
    address1    = "123 Main St"
    city        = "Anytown"
    state       = "CA"
    postal_code = "12345"
    country     = "US"
  }
}