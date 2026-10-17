package provider

import (
	"context"
	"testing"

	"github.com/bearcode33/terraform-provider-godaddy/internal/godaddy"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// configureProvider runs Configure with the given HCL attribute values. Unset
// attributes are null, as when they are left out of the provider block.
func configureProvider(t *testing.T, values map[string]string) (*godaddy.Client, diag.Diagnostics) {
	t.Helper()
	ctx := context.Background()
	p := New("test")()

	var schemaResp provider.SchemaResponse
	p.Schema(ctx, provider.SchemaRequest{}, &schemaResp)

	raw := map[string]tftypes.Value{}
	for name := range schemaResp.Schema.Attributes {
		raw[name] = tftypes.NewValue(tftypes.String, nil)
	}
	for name, value := range values {
		raw[name] = tftypes.NewValue(tftypes.String, value)
	}

	config := tfsdk.Config{
		Schema: schemaResp.Schema,
		Raw:    tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), raw),
	}

	var resp provider.ConfigureResponse
	p.Configure(ctx, provider.ConfigureRequest{Config: config}, &resp)

	client, _ := resp.ResourceData.(*godaddy.Client)
	return client, resp.Diagnostics
}

func clearGoDaddyEnv(t *testing.T) {
	for _, name := range []string{"GODADDY_API_KEY", "GODADDY_API_SECRET", "GODADDY_API_TOKEN", "GODADDY_ENVIRONMENT"} {
		t.Setenv(name, "")
	}
}

func TestConfigure_KeyAndSecretStillWork(t *testing.T) {
	clearGoDaddyEnv(t)

	client, diags := configureProvider(t, map[string]string{"api_key": "key", "api_secret": "secret"})
	if diags.HasError() || diags.WarningsCount() > 0 {
		t.Fatalf("diagnostics = %v, want none", diags)
	}
	if client == nil || client.UsesAPIToken() {
		t.Fatal("want a key/secret client")
	}
}

func TestConfigure_KeyAndSecretFromEnv(t *testing.T) {
	clearGoDaddyEnv(t)
	t.Setenv("GODADDY_API_KEY", "key")
	t.Setenv("GODADDY_API_SECRET", "secret")

	client, diags := configureProvider(t, nil)
	if diags.HasError() {
		t.Fatalf("diagnostics = %v", diags)
	}
	if client == nil || client.UsesAPIToken() {
		t.Fatal("want a key/secret client")
	}
}

func TestConfigure_TokenOnly(t *testing.T) {
	clearGoDaddyEnv(t)

	client, diags := configureProvider(t, map[string]string{"api_token": "pat"})
	if diags.HasError() || diags.WarningsCount() > 0 {
		t.Fatalf("diagnostics = %v, want none", diags)
	}
	if client == nil || !client.UsesAPIToken() {
		t.Fatal("want a token client")
	}
}

func TestConfigure_TokenFromEnv(t *testing.T) {
	clearGoDaddyEnv(t)
	t.Setenv("GODADDY_API_TOKEN", "pat")

	client, diags := configureProvider(t, nil)
	if diags.HasError() {
		t.Fatalf("diagnostics = %v", diags)
	}
	if client == nil || !client.UsesAPIToken() {
		t.Fatal("want a token client")
	}
}

func TestConfigure_TokenAndKeyInConfigIsAnError(t *testing.T) {
	clearGoDaddyEnv(t)

	client, diags := configureProvider(t, map[string]string{"api_key": "key", "api_secret": "secret", "api_token": "pat"})
	if diags.ErrorsCount() != 1 {
		t.Errorf("errors = %d, want 1", diags.ErrorsCount())
	}
	if client != nil {
		t.Error("want no client")
	}
}

func TestConfigure_KeyInConfigBeatsTokenInEnv(t *testing.T) {
	clearGoDaddyEnv(t)
	t.Setenv("GODADDY_API_TOKEN", "pat")

	client, diags := configureProvider(t, map[string]string{"api_key": "key", "api_secret": "secret"})
	if diags.HasError() || diags.WarningsCount() > 0 {
		t.Fatalf("diagnostics = %v, want none", diags)
	}
	if client == nil || client.UsesAPIToken() {
		t.Fatal("want the key/secret client from config, not the env token")
	}
}

func TestConfigure_TokenInConfigBeatsKeyInEnv(t *testing.T) {
	clearGoDaddyEnv(t)
	t.Setenv("GODADDY_API_KEY", "key")
	t.Setenv("GODADDY_API_SECRET", "secret")

	client, diags := configureProvider(t, map[string]string{"api_token": "pat"})
	if diags.HasError() {
		t.Fatalf("diagnostics = %v", diags)
	}
	if client == nil || !client.UsesAPIToken() {
		t.Fatal("want the token client from config")
	}
}

func TestConfigure_TokenAndKeyInEnvIsAnError(t *testing.T) {
	clearGoDaddyEnv(t)
	t.Setenv("GODADDY_API_TOKEN", "pat")
	t.Setenv("GODADDY_API_KEY", "key")
	t.Setenv("GODADDY_API_SECRET", "secret")

	client, diags := configureProvider(t, nil)
	if diags.ErrorsCount() != 1 {
		t.Errorf("errors = %d, want 1", diags.ErrorsCount())
	}
	if client != nil {
		t.Error("want no client")
	}
}

func TestConfigure_NoCredentials(t *testing.T) {
	clearGoDaddyEnv(t)

	client, diags := configureProvider(t, nil)
	if diags.ErrorsCount() != 2 {
		t.Errorf("errors = %d, want 2 (key and secret)", diags.ErrorsCount())
	}
	if client != nil {
		t.Error("want no client")
	}
}

func TestConfigure_BlankTokenFallsBackToKey(t *testing.T) {
	clearGoDaddyEnv(t)

	client, diags := configureProvider(t, map[string]string{"api_token": "   ", "api_key": "key", "api_secret": "secret"})
	if diags.HasError() {
		t.Fatalf("diagnostics = %v", diags)
	}
	if client == nil || client.UsesAPIToken() {
		t.Fatal("want a key/secret client")
	}
}

func TestConfigure_BlankKeyInEnvDoesNotConflictWithToken(t *testing.T) {
	clearGoDaddyEnv(t)
	t.Setenv("GODADDY_API_TOKEN", "pat")
	t.Setenv("GODADDY_API_KEY", "   ")

	client, diags := configureProvider(t, nil)
	if diags.HasError() {
		t.Fatalf("diagnostics = %v, want none for a whitespace-only key", diags)
	}
	if client == nil || !client.UsesAPIToken() {
		t.Fatal("want the token client")
	}
}
