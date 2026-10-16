package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/bearcode33/terraform-provider-godaddy/internal/godaddy"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &DomainDataSource{}

func NewDomainDataSource() datasource.DataSource {
	return &DomainDataSource{}
}

type DomainDataSource struct {
	client *godaddy.Client
}

type DomainDataSourceModel struct {
	Domain              types.String `tfsdk:"domain"`
	Status              types.String `tfsdk:"status"`
	Expires             types.String `tfsdk:"expires"`
	ExpirationProtected types.Bool   `tfsdk:"expiration_protected"`
	HoldRegistrar       types.Bool   `tfsdk:"hold_registrar"`
	Locked              types.Bool   `tfsdk:"locked"`
	Privacy             types.Bool   `tfsdk:"privacy"`
	RenewAuto           types.Bool   `tfsdk:"renew_auto"`
	TransferProtected   types.Bool   `tfsdk:"transfer_protected"`
	Nameservers         types.List   `tfsdk:"nameservers"`
	CreatedAt           types.String `tfsdk:"created_at"`
	ModifiedAt          types.String `tfsdk:"modified_at"`
}

func (d *DomainDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (d *DomainDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Data source for retrieving information about a GoDaddy domain.",
		Attributes: map[string]schema.Attribute{
			"domain": schema.StringAttribute{
				MarkdownDescription: "The domain name to look up.",
				Required:            true,
				Validators: []validator.String{
					DomainNameValidator(),
				},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "The current status of the domain.",
				Computed:            true,
			},
			"expires": schema.StringAttribute{
				MarkdownDescription: "The expiration date of the domain.",
				Computed:            true,
			},
			"expiration_protected": schema.BoolAttribute{
				MarkdownDescription: "Whether the domain is protected from expiration.",
				Computed:            true,
			},
			"hold_registrar": schema.BoolAttribute{
				MarkdownDescription: "Whether the domain has a registrar hold.",
				Computed:            true,
			},
			"locked": schema.BoolAttribute{
				MarkdownDescription: "Whether the domain is locked to prevent transfers.",
				Computed:            true,
			},
			"privacy": schema.BoolAttribute{
				MarkdownDescription: "Whether WHOIS privacy is enabled.",
				Computed:            true,
			},
			"renew_auto": schema.BoolAttribute{
				MarkdownDescription: "Whether the domain is set to auto-renew.",
				Computed:            true,
			},
			"transfer_protected": schema.BoolAttribute{
				MarkdownDescription: "Whether the domain is protected from transfers.",
				Computed:            true,
			},
			"nameservers": schema.ListAttribute{
				MarkdownDescription: "The nameservers for the domain.",
				Computed:            true,
				ElementType:         types.StringType,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "When the domain was created.",
				Computed:            true,
			},
			"modified_at": schema.StringAttribute{
				MarkdownDescription: "When the domain was last modified.",
				Computed:            true,
			},
		},
	}
}

func (d *DomainDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*godaddy.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *godaddy.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *DomainDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DomainDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domain, err := d.client.GetDomain(ctx, data.Domain.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Domain",
			fmt.Sprintf("Could not read domain %s: %s", data.Domain.ValueString(), err),
		)
		return
	}

	// Update the model with the domain data
	data.Status = types.StringValue(domain.Status)

	if domain.Expires != nil {
		data.Expires = types.StringValue(domain.Expires.Format(time.RFC3339))
	} else {
		data.Expires = types.StringNull()
	}

	data.ExpirationProtected = types.BoolValue(domain.ExpirationProtected)
	data.HoldRegistrar = types.BoolValue(domain.HoldRegistrar)
	data.Locked = types.BoolValue(domain.Locked)
	data.Privacy = types.BoolValue(domain.Privacy)
	data.RenewAuto = types.BoolValue(domain.RenewAuto)
	data.TransferProtected = types.BoolValue(domain.TransferProtected)
	data.CreatedAt = types.StringValue(domain.CreatedAt.Format(time.RFC3339))

	if !domain.ModifiedAt.IsZero() {
		data.ModifiedAt = types.StringValue(domain.ModifiedAt.Format(time.RFC3339))
	} else {
		data.ModifiedAt = types.StringNull()
	}

	// Convert nameservers
	if len(domain.Nameservers) > 0 {
		nameservers := make([]attr.Value, len(domain.Nameservers))
		for i, ns := range domain.Nameservers {
			nameservers[i] = types.StringValue(ns)
		}
		data.Nameservers = types.ListValueMust(types.StringType, nameservers)
	} else {
		data.Nameservers = types.ListNull(types.StringType)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
