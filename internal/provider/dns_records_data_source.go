package provider

import (
	"context"
	"fmt"

	"github.com/bearcode33/terraform-provider-godaddy/internal/godaddy"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &DNSRecordsDataSource{}

func NewDNSRecordsDataSource() datasource.DataSource {
	return &DNSRecordsDataSource{}
}

type DNSRecordsDataSource struct {
	client *godaddy.Client
}

type DNSRecordsDataSourceModel struct {
	Domain  types.String `tfsdk:"domain"`
	Type    types.String `tfsdk:"type"`
	Name    types.String `tfsdk:"name"`
	Records types.List   `tfsdk:"records"`
}

type DNSRecordModel struct {
	Type     types.String `tfsdk:"type"`
	Name     types.String `tfsdk:"name"`
	Data     types.String `tfsdk:"data"`
	TTL      types.Int32  `tfsdk:"ttl"`
	Priority types.Int32  `tfsdk:"priority"`
	Port     types.Int32  `tfsdk:"port"`
	Weight   types.Int32  `tfsdk:"weight"`
	Service  types.String `tfsdk:"service"`
	Protocol types.String `tfsdk:"protocol"`
}

func (d *DNSRecordsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_records"
}

func (d *DNSRecordsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Data source for retrieving DNS records for a GoDaddy domain.",
		Attributes: map[string]schema.Attribute{
			"domain": schema.StringAttribute{
				MarkdownDescription: "The domain name to retrieve DNS records for.",
				Required:            true,
				Validators: []validator.String{
					DomainNameValidator(),
				},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Filter by record type (A, AAAA, CNAME, MX, NS, SOA, SRV, TXT, etc.).",
				Optional:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Filter by record name.",
				Optional:            true,
				Validators: []validator.String{
					DNSRecordNameValidator(),
				},
			},
			"records": schema.ListNestedAttribute{
				MarkdownDescription: "List of DNS records.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							MarkdownDescription: "The record type.",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "The record name.",
							Computed:            true,
						},
						"data": schema.StringAttribute{
							MarkdownDescription: "The record data.",
							Computed:            true,
						},
						"ttl": schema.Int32Attribute{
							MarkdownDescription: "Time to live in seconds.",
							Computed:            true,
						},
						"priority": schema.Int32Attribute{
							MarkdownDescription: "Priority for MX and SRV records.",
							Computed:            true,
						},
						"port": schema.Int32Attribute{
							MarkdownDescription: "Port for SRV records.",
							Computed:            true,
						},
						"weight": schema.Int32Attribute{
							MarkdownDescription: "Weight for SRV records.",
							Computed:            true,
						},
						"service": schema.StringAttribute{
							MarkdownDescription: "Service for SRV records.",
							Computed:            true,
						},
						"protocol": schema.StringAttribute{
							MarkdownDescription: "Protocol for SRV records.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *DNSRecordsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DNSRecordsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DNSRecordsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var records []godaddy.DNSRecord
	var err error

	if !data.Type.IsNull() && !data.Name.IsNull() {
		// Get specific type and name
		records, err = d.client.GetDNSRecordsByTypeAndName(ctx, data.Domain.ValueString(), data.Type.ValueString(), data.Name.ValueString())
	} else if !data.Type.IsNull() {
		// Get all records of a specific type
		records, err = d.client.GetDNSRecordsByType(ctx, data.Domain.ValueString(), data.Type.ValueString())
	} else {
		// Get all records
		records, err = d.client.GetDNSRecords(ctx, data.Domain.ValueString())
	}

	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading DNS Records",
			fmt.Sprintf("Could not read DNS records for domain %s: %s", data.Domain.ValueString(), err),
		)
		return
	}

	// Filter by name if specified and not already filtered
	if !data.Name.IsNull() && data.Type.IsNull() {
		nameFilter := data.Name.ValueString()
		filteredRecords := make([]godaddy.DNSRecord, 0)
		for _, record := range records {
			if record.Name == nameFilter {
				filteredRecords = append(filteredRecords, record)
			}
		}
		records = filteredRecords
	}

	// Convert records to model
	recordModels := make([]DNSRecordModel, len(records))
	for i, record := range records {
		recordModels[i] = DNSRecordModel{
			Type: types.StringValue(record.Type),
			Name: types.StringValue(record.Name),
			Data: types.StringValue(record.Data),
			TTL:  types.Int32Value(int32(record.TTL)),
		}

		if record.Priority != nil {
			recordModels[i].Priority = types.Int32Value(int32(*record.Priority))
		} else {
			recordModels[i].Priority = types.Int32Null()
		}

		if record.Port != nil {
			recordModels[i].Port = types.Int32Value(int32(*record.Port))
		} else {
			recordModels[i].Port = types.Int32Null()
		}

		if record.Weight != nil {
			recordModels[i].Weight = types.Int32Value(int32(*record.Weight))
		} else {
			recordModels[i].Weight = types.Int32Null()
		}

		if record.Service != nil {
			recordModels[i].Service = types.StringValue(*record.Service)
		} else {
			recordModels[i].Service = types.StringNull()
		}

		if record.Protocol != nil {
			recordModels[i].Protocol = types.StringValue(*record.Protocol)
		} else {
			recordModels[i].Protocol = types.StringNull()
		}
	}

	recordsList, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dnsRecordAttributeTypes()}, recordModels)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Records = recordsList
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func dnsRecordAttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"type":     types.StringType,
		"name":     types.StringType,
		"data":     types.StringType,
		"ttl":      types.Int32Type,
		"priority": types.Int32Type,
		"port":     types.Int32Type,
		"weight":   types.Int32Type,
		"service":  types.StringType,
		"protocol": types.StringType,
	}
}
