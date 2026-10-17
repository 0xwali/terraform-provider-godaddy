package provider

import (
	"testing"

	"github.com/bearcode33/terraform-provider-godaddy/internal/godaddy"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func stringList(values ...string) types.List {
	elems := make([]attr.Value, len(values))
	for i, v := range values {
		elems[i] = types.StringValue(v)
	}
	return types.ListValueMust(types.StringType, elems)
}

func TestNameserversMatch(t *testing.T) {
	tests := []struct {
		name string
		want []string
		have []string
		ok   bool
	}{
		{"same list", []string{"a.ns.cloudflare.com", "b.ns.cloudflare.com"}, []string{"a.ns.cloudflare.com", "b.ns.cloudflare.com"}, true},
		{"order, case and trailing dot ignored", []string{"A.ns.cloudflare.com.", "b.ns.cloudflare.com"}, []string{"b.ns.cloudflare.com", "a.ns.cloudflare.com"}, true},
		{"GoDaddy pair assigned by GoDaddy", []string{"ns1.domaincontrol.com", "ns2.domaincontrol.com"}, []string{"ns45.domaincontrol.com", "ns46.domaincontrol.com"}, true},
		{"GoDaddy wanted, external live", []string{"ns1.domaincontrol.com", "ns2.domaincontrol.com"}, []string{"a.ns.cloudflare.com", "b.ns.cloudflare.com"}, false},
		{"external wanted, GoDaddy live", []string{"a.ns.cloudflare.com", "b.ns.cloudflare.com"}, []string{"ns45.domaincontrol.com", "ns46.domaincontrol.com"}, false},
		{"different external hosts", []string{"a.ns.cloudflare.com", "b.ns.cloudflare.com"}, []string{"a.ns.cloudflare.com", "c.ns.cloudflare.com"}, false},
		{"mixed set is not a GoDaddy set", []string{"ns1.domaincontrol.com", "a.ns.cloudflare.com"}, []string{"ns45.domaincontrol.com", "ns46.domaincontrol.com"}, false},
		{"look-alike suffix", []string{"ns1.evildomaincontrol.com", "ns2.evildomaincontrol.com"}, []string{"ns45.domaincontrol.com", "ns46.domaincontrol.com"}, false},
		{"nothing live", []string{"ns1.domaincontrol.com", "ns2.domaincontrol.com"}, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nameserversMatch(tt.want, tt.have); got != tt.ok {
				t.Errorf("nameserversMatch(%v, %v) = %v, want %v", tt.want, tt.have, got, tt.ok)
			}
		})
	}
}

func TestUpdateModelFromDomain_KeepsConfiguredGoDaddyPair(t *testing.T) {
	r := &DomainResource{}
	model := DomainResourceModel{Nameservers: stringList("ns1.domaincontrol.com", "ns2.domaincontrol.com")}
	r.updateModelFromDomain(&model, &godaddy.DomainDetail{Nameservers: []string{"ns45.domaincontrol.com", "ns46.domaincontrol.com"}})

	if got := listStrings(model.Nameservers); len(got) != 2 || got[0] != "ns1.domaincontrol.com" {
		t.Errorf("nameservers = %v, want the configured list kept", got)
	}
	if got := listStrings(model.AssignedNameservers); len(got) != 2 || got[0] != "ns45.domaincontrol.com" {
		t.Errorf("assigned_nameservers = %v, want the pair GoDaddy reports", got)
	}
}

func TestUpdateModelFromDomain_ShowsOutsideChange(t *testing.T) {
	r := &DomainResource{}
	model := DomainResourceModel{Nameservers: stringList("ns1.domaincontrol.com", "ns2.domaincontrol.com")}
	r.updateModelFromDomain(&model, &godaddy.DomainDetail{Nameservers: []string{"a.ns.cloudflare.com", "b.ns.cloudflare.com"}})

	if got := listStrings(model.Nameservers); len(got) != 2 || got[0] != "a.ns.cloudflare.com" {
		t.Errorf("nameservers = %v, want the live list so the plan shows the change", got)
	}
}

func TestUpdateModelFromDomain_ImportTakesLiveList(t *testing.T) {
	r := &DomainResource{}
	model := DomainResourceModel{Nameservers: types.ListNull(types.StringType)}
	r.updateModelFromDomain(&model, &godaddy.DomainDetail{Nameservers: []string{"ns45.domaincontrol.com", "ns46.domaincontrol.com"}})

	if got := listStrings(model.Nameservers); len(got) != 2 || got[0] != "ns45.domaincontrol.com" {
		t.Errorf("nameservers = %v, want the live list on import", got)
	}
}

func TestCheckNameserversApplied(t *testing.T) {
	r := &DomainResource{}
	model := DomainResourceModel{Domain: types.StringValue("example.com"), Nameservers: stringList("a.ns.cloudflare.com", "b.ns.cloudflare.com")}

	var diags diag.Diagnostics
	r.checkNameserversApplied(&model, &godaddy.DomainDetail{Nameservers: []string{"ns45.domaincontrol.com", "ns46.domaincontrol.com"}}, &diags)
	if diags.ErrorsCount() != 1 {
		t.Errorf("errors = %d, want 1 when GoDaddy ignored the external nameservers", diags.ErrorsCount())
	}

	diags = nil
	r.checkNameserversApplied(&model, &godaddy.DomainDetail{Nameservers: []string{"b.ns.cloudflare.com.", "a.ns.cloudflare.com"}}, &diags)
	if diags.HasError() {
		t.Errorf("diagnostics = %v, want none when the nameservers match", diags)
	}
}

func TestObjectToContact_UnknownAndNull(t *testing.T) {
	if got := objectToContact(types.ObjectUnknown(contactAttributeTypes())); got != nil {
		t.Errorf("objectToContact(unknown) = %+v, want nil", got)
	}
	if got := objectToContact(types.ObjectNull(contactAttributeTypes())); got != nil {
		t.Errorf("objectToContact(null) = %+v, want nil", got)
	}
}

func TestUpdateModelFromDomain_NoLiveNameserversClearsUnknown(t *testing.T) {
	r := &DomainResource{}
	model := DomainResourceModel{Nameservers: types.ListUnknown(types.StringType)}
	r.updateModelFromDomain(&model, &godaddy.DomainDetail{})

	if model.Nameservers.IsUnknown() || !model.Nameservers.IsNull() {
		t.Errorf("nameservers = %v, want null, never unknown, after apply", model.Nameservers)
	}
}
