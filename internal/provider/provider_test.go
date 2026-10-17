package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

const (
	providerConfig = `
provider "godaddy" {
  api_key    = "test-key"
  api_secret = "test-secret"
  environment = "test"
}
`
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"godaddy": providerserver.NewProtocol6WithError(New("test")()),
}

func testAccPreCheck(t *testing.T) {
	if v := os.Getenv("GODADDY_API_TOKEN"); v != "" {
		return
	}
	if v := os.Getenv("GODADDY_API_KEY"); v == "" {
		t.Skip("GODADDY_API_TOKEN, or GODADDY_API_KEY and GODADDY_API_SECRET, must be set for acceptance tests")
	}
	if v := os.Getenv("GODADDY_API_SECRET"); v == "" {
		t.Skip("GODADDY_API_SECRET must be set for acceptance tests")
	}
}
