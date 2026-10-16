// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"os"
	"strings"

	"github.com/bearcode33/terraform-provider-godaddy/internal/godaddy"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure GoDaddyProvider satisfies various provider interfaces.
var _ provider.Provider = &GoDaddyProvider{}

// GoDaddyProvider defines the provider implementation.
type GoDaddyProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// GoDaddyProviderModel describes the provider data model.
type GoDaddyProviderModel struct {
	APIKey      types.String `tfsdk:"api_key"`
	APISecret   types.String `tfsdk:"api_secret"`
	APIToken    types.String `tfsdk:"api_token"`
	Environment types.String `tfsdk:"environment"`
}

func (p *GoDaddyProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "godaddy"
	resp.Version = p.version
}

func (p *GoDaddyProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Terraform provider for managing GoDaddy domains and DNS records.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				MarkdownDescription: "GoDaddy API Key. Can also be set via GODADDY_API_KEY environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"api_secret": schema.StringAttribute{
				MarkdownDescription: "GoDaddy API Secret. Can also be set via GODADDY_API_SECRET environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"api_token": schema.StringAttribute{
				MarkdownDescription: "GoDaddy Personal Access Token (PAT). When set, the provider uses the v3 Domains API with Bearer auth, and `api_key` and `api_secret` are not needed. Setting both in the configuration is an error. Can also be set via GODADDY_API_TOKEN environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"environment": schema.StringAttribute{
				MarkdownDescription: "GoDaddy API environment. Valid values are 'production' (default) and 'test'. Can also be set via GODADDY_ENVIRONMENT environment variable.",
				Optional:            true,
			},
		},
	}
}

func (p *GoDaddyProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data GoDaddyProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Get API credentials from config or environment. Values in the provider
	// block beat environment variables, so a stray GODADDY_API_TOKEN cannot
	// switch a config that sets api_key and api_secret to another account.
	tokenInConfig := configured(data.APIToken)
	keyInConfig := configured(data.APIKey) || configured(data.APISecret)

	if tokenInConfig && keyInConfig {
		resp.Diagnostics.AddError(
			"Conflicting GoDaddy Credentials",
			"Set either api_token, or api_key and api_secret, in the provider configuration, not both.",
		)
		return
	}

	apiKey := os.Getenv("GODADDY_API_KEY")
	if !data.APIKey.IsNull() {
		apiKey = data.APIKey.ValueString()
	}

	apiSecret := os.Getenv("GODADDY_API_SECRET")
	if !data.APISecret.IsNull() {
		apiSecret = data.APISecret.ValueString()
	}

	apiToken := ""
	switch {
	case tokenInConfig:
		apiToken = data.APIToken.ValueString()
	case !keyInConfig:
		apiToken = os.Getenv("GODADDY_API_TOKEN")
		if strings.TrimSpace(apiToken) != "" && (apiKey != "" || apiSecret != "") {
			resp.Diagnostics.AddError(
				"Conflicting GoDaddy Credentials",
				"GODADDY_API_TOKEN and GODADDY_API_KEY/GODADDY_API_SECRET are both set. "+
					"Unset one of them, or set the credential you want in the provider configuration.",
			)
			return
		}
	}

	environment := os.Getenv("GODADDY_ENVIRONMENT")
	if !data.Environment.IsNull() {
		environment = data.Environment.ValueString()
	}

	var opts []godaddy.ClientOption
	if environment == "test" {
		opts = append(opts, godaddy.WithTestEnvironment())
	}

	if strings.TrimSpace(apiToken) != "" {
		client := godaddy.NewTokenClient(apiToken, opts...)
		resp.DataSourceData = client
		resp.ResourceData = client
		return
	}

	// Validate required fields
	if apiKey == "" {
		resp.Diagnostics.AddError(
			"Missing API Key",
			"The provider requires an API key or an API token to be configured. "+
				"Set the api_key attribute in the provider configuration or the GODADDY_API_KEY environment variable, "+
				"or set api_token / GODADDY_API_TOKEN to use a Personal Access Token.",
		)
	}

	if apiSecret == "" {
		resp.Diagnostics.AddError(
			"Missing API Secret",
			"The provider requires an API secret to be configured. "+
				"Set the api_secret attribute in the provider configuration or the GODADDY_API_SECRET environment variable, "+
				"or set api_token / GODADDY_API_TOKEN to use a Personal Access Token.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Create GoDaddy client
	client := godaddy.NewClient(apiKey, apiSecret, opts...)
	resp.DataSourceData = client
	resp.ResourceData = client
}

// configured reports whether a provider attribute was set to a non-blank value.
func configured(value types.String) bool {
	return !value.IsNull() && !value.IsUnknown() && strings.TrimSpace(value.ValueString()) != ""
}

func (p *GoDaddyProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewDomainResource,
		NewDNSRecordResource,
	}
}

func (p *GoDaddyProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewDomainDataSource,
		NewDNSRecordsDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &GoDaddyProvider{
			version: version,
		}
	}
}
