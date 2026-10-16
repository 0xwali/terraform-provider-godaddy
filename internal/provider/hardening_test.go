package provider

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bearcode33/terraform-provider-godaddy/internal/godaddy"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func txt(data string) godaddy.DNSRecord {
	return godaddy.DNSRecord{Type: "TXT", Name: "@", Data: data, TTL: 600}
}

func cname(data string) godaddy.DNSRecord {
	return godaddy.DNSRecord{Type: "CNAME", Name: "www", Data: data, TTL: 600}
}

func TestPlanCreate(t *testing.T) {
	tests := []struct {
		name           string
		existing       []godaddy.DNSRecord
		desired        godaddy.DNSRecord
		allowOverwrite bool
		want           createAction
		wantErr        string
	}{
		{name: "empty set adds", desired: txt("a"), want: createAdd},
		{name: "second TXT is added next to the first", existing: []godaddy.DNSRecord{txt("spf")}, desired: txt("verify"), want: createAdd},
		{name: "second TXT with allow_overwrite still only adds", existing: []godaddy.DNSRecord{txt("spf")}, desired: txt("verify"), allowOverwrite: true, want: createAdd},
		{name: "same data without allow_overwrite asks for import", existing: []godaddy.DNSRecord{txt("spf")}, desired: txt("spf"), wantErr: "already exists"},
		{name: "same data with allow_overwrite adopts", existing: []godaddy.DNSRecord{txt("spf"), txt("other")}, desired: txt("spf"), allowOverwrite: true, want: createAdopt},
		{name: "CNAME conflict without allow_overwrite fails", existing: []godaddy.DNSRecord{cname("old.example.com")}, desired: cname("new.example.com"), wantErr: "only one CNAME"},
		{name: "CNAME conflict with allow_overwrite replaces", existing: []godaddy.DNSRecord{cname("old.example.com")}, desired: cname("new.example.com"), allowOverwrite: true, want: createReplace},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := planCreate(tt.existing, tt.desired, tt.allowOverwrite)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("planCreate() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("planCreate() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("planCreate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindStoredRecord(t *testing.T) {
	if r, ok := findStoredRecord([]godaddy.DNSRecord{txt("a"), txt("b")}, "TXT", "b"); !ok || r.Data != "b" {
		t.Errorf("exact match: got %v, %v", r, ok)
	}
	// A TXT changed outside Terraform: the other TXT at @ belongs to someone
	// else, so the resource must not adopt it.
	if _, ok := findStoredRecord([]godaddy.DNSRecord{txt("someone-else")}, "TXT", "mine"); ok {
		t.Error("multi-value type adopted a record with other data")
	}
	// A CNAME changed outside Terraform is the same record with new data.
	if r, ok := findStoredRecord([]godaddy.DNSRecord{cname("changed.example.com")}, "CNAME", "old.example.com"); !ok || r.Data != "changed.example.com" {
		t.Errorf("single-value drift: got %v, %v, want the live record", r, ok)
	}
	mx := godaddy.DNSRecord{Type: "MX", Name: "@", Data: "Mail.Example.com.", TTL: 600}
	if _, ok := findStoredRecord([]godaddy.DNSRecord{mx}, "MX", "mail.example.com"); !ok {
		t.Error("MX hostname with other case and a trailing dot did not match")
	}
	if _, ok := findStoredRecord([]godaddy.DNSRecord{txt("ABC")}, "TXT", "abc"); ok {
		t.Error("TXT data matched case-insensitively; TXT data is case-sensitive")
	}
	if _, err := planCreate([]godaddy.DNSRecord{mx}, godaddy.DNSRecord{Type: "MX", Name: "@", Data: "mail.example.com"}, false); err == nil {
		t.Error("planCreate added a second copy of an MX record that differs only in case")
	}
	if _, ok := findStoredRecord(nil, "CNAME", "x"); ok {
		t.Error("empty set returned a record")
	}
}

func TestLockRecordSetSerializes(t *testing.T) {
	var inside, maxInside atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Same set, different spelling of domain, type and name.
			domains := []string{"Example.com", "example.com"}
			types := []string{"txt", "TXT"}
			unlock := lockRecordSet(domains[i%2], types[i%2], "@")
			defer unlock()
			n := inside.Add(1)
			for {
				m := maxInside.Load()
				if n <= m || maxInside.CompareAndSwap(m, n) {
					break
				}
			}
			time.Sleep(2 * time.Millisecond)
			inside.Add(-1)
		}(i)
	}
	wg.Wait()
	if maxInside.Load() != 1 {
		t.Errorf("max goroutines inside the lock = %d, want 1", maxInside.Load())
	}

	// Different sets do not block each other.
	unlockA := lockRecordSet("example.com", "TXT", "a")
	done := make(chan struct{})
	go func() {
		unlockB := lockRecordSet("example.com", "TXT", "b")
		unlockB()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("lock on another name blocked")
	}
	unlockA()
}

func TestValidateRecordName(t *testing.T) {
	valid := []string{"@", "www", "*", "*.dev", "_dmarc", "_sip._tcp", "a-b.c", "selector1._domainkey"}
	for _, name := range valid {
		if err := ValidateRecordName(name); err != nil {
			t.Errorf("ValidateRecordName(%q) = %v, want nil", name, err)
		}
	}
	invalid := []string{"", ".www", "a..b", "../victim", "a/b", "a?b", "a#b", "a%2Fb", "a b", "@www", "www@"}
	for _, name := range invalid {
		if err := ValidateRecordName(name); err == nil {
			t.Errorf("ValidateRecordName(%q) = nil, want an error", name)
		}
	}
}

func TestValidateDomainName(t *testing.T) {
	valid := []string{"example.com", "sub.example.co.uk", "xn--bcher-kva.example", "a-b.io"}
	for _, domain := range valid {
		if err := ValidateDomainName(domain); err != nil {
			t.Errorf("ValidateDomainName(%q) = %v, want nil", domain, err)
		}
	}
	invalid := []string{"", "localhost", "example.com/", "../example.com", "example.com?x", "-a.com", "a-.com", "a..com", "exa mple.com", strings.Repeat("a", 250) + ".com"}
	for _, domain := range invalid {
		if err := ValidateDomainName(domain); err == nil {
			t.Errorf("ValidateDomainName(%q) = nil, want an error", domain)
		}
	}
}

func TestParseEnvironment(t *testing.T) {
	tests := map[string]bool{"": false, "production": false, "Production": false, "test": true, " TEST ": true}
	for in, wantTest := range tests {
		got, err := parseEnvironment(in)
		if err != nil || got != wantTest {
			t.Errorf("parseEnvironment(%q) = %v, %v; want %v, nil", in, got, err, wantTest)
		}
	}
	for _, in := range []string{"ote", "staging", "prod", "testing"} {
		if _, err := parseEnvironment(in); err == nil {
			t.Errorf("parseEnvironment(%q) = nil error, want an error", in)
		}
	}
}

func configureForTest(t *testing.T, values map[string]string) diag.Diagnostics {
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

	var resp provider.ConfigureResponse
	p.Configure(ctx, provider.ConfigureRequest{Config: tfsdk.Config{
		Schema: schemaResp.Schema,
		Raw:    tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), raw),
	}}, &resp)
	return resp.Diagnostics
}

func TestConfigure_RejectsUnknownEnvironment(t *testing.T) {
	t.Setenv("GODADDY_ENVIRONMENT", "")
	diags := configureForTest(t, map[string]string{"api_key": "k", "api_secret": "s", "environment": "staging"})
	if !diags.HasError() {
		t.Fatal("environment \"staging\" was accepted")
	}

	t.Setenv("GODADDY_ENVIRONMENT", "ote")
	diags = configureForTest(t, map[string]string{"api_key": "k", "api_secret": "s"})
	if !diags.HasError() {
		t.Fatal("GODADDY_ENVIRONMENT=ote was accepted")
	}

	t.Setenv("GODADDY_ENVIRONMENT", "")
	diags = configureForTest(t, map[string]string{"api_key": "k", "api_secret": "s", "environment": "Test"})
	if diags.HasError() {
		t.Fatalf("environment \"Test\" rejected: %v", diags)
	}
}

func TestConfigure_BlankKeyIsMissing(t *testing.T) {
	t.Setenv("GODADDY_API_KEY", "")
	t.Setenv("GODADDY_API_SECRET", "")
	t.Setenv("GODADDY_ENVIRONMENT", "")
	diags := configureForTest(t, map[string]string{"api_key": "  ", "api_secret": "s"})
	if diags.ErrorsCount() != 1 {
		t.Fatalf("errors = %d, want 1 (blank key)", diags.ErrorsCount())
	}
}

func TestCheckReadOnlyDomainSettings(t *testing.T) {
	current := &godaddy.DomainDetail{Domain: godaddy.Domain{Privacy: true, ExpirationProtected: false}}

	model := &DomainResourceModel{
		Domain:              types.StringValue("example.com"),
		Privacy:             types.BoolValue(false),
		ExpirationProtected: types.BoolNull(),
	}
	if err := checkReadOnlyDomainSettings(model, current); err == nil || !strings.Contains(err.Error(), "privacy") {
		t.Errorf("privacy change: error = %v, want a privacy error", err)
	}

	model.Privacy = types.BoolValue(true)
	model.ExpirationProtected = types.BoolValue(true)
	if err := checkReadOnlyDomainSettings(model, current); err == nil || !strings.Contains(err.Error(), "expiration_protected") {
		t.Errorf("expiration_protected change: error = %v, want an expiration_protected error", err)
	}

	model.Privacy = types.BoolUnknown()
	model.ExpirationProtected = types.BoolNull()
	if err := checkReadOnlyDomainSettings(model, current); err != nil {
		t.Errorf("unset values: error = %v, want nil", err)
	}
}

func TestDomainSchemaHasNoSettingDefaults(t *testing.T) {
	r := &DomainResource{}
	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)

	for _, name := range []string{"locked", "renew_auto", "privacy", "expiration_protected"} {
		attr, ok := resp.Schema.Attributes[name].(resourceschema.BoolAttribute)
		if !ok {
			t.Fatalf("%s is not a bool attribute", name)
		}
		if attr.Default != nil {
			t.Errorf("%s has a default; unset must mean leave the domain as it is", name)
		}
	}
	for _, name := range []string{"contact_admin", "contact_billing", "contact_registrant", "contact_tech"} {
		if !resp.Schema.Attributes[name].IsSensitive() {
			t.Errorf("%s is not sensitive", name)
		}
	}
}
