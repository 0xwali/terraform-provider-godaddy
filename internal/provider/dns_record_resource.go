package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/bearcode33/terraform-provider-godaddy/internal/godaddy"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &DNSRecordResource{}
var _ resource.ResourceWithImportState = &DNSRecordResource{}

func NewDNSRecordResource() resource.Resource {
	return &DNSRecordResource{}
}

type DNSRecordResource struct {
	client *godaddy.Client
}

type DNSRecordResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Domain         types.String `tfsdk:"domain"`
	Type           types.String `tfsdk:"type"`
	Name           types.String `tfsdk:"name"`
	Data           types.String `tfsdk:"data"`
	TTL            types.Int32  `tfsdk:"ttl"`
	Priority       types.Int32  `tfsdk:"priority"`
	Port           types.Int32  `tfsdk:"port"`
	Weight         types.Int32  `tfsdk:"weight"`
	Service        types.String `tfsdk:"service"`
	Protocol       types.String `tfsdk:"protocol"`
	AllowOverwrite types.Bool   `tfsdk:"allow_overwrite"`
}

func (r *DNSRecordResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_record"
}

func (r *DNSRecordResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a DNS record for a GoDaddy domain.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier for the DNS record.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"domain": schema.StringAttribute{
				MarkdownDescription: "The domain name to add the record to.",
				Required:            true,
				Validators: []validator.String{
					DomainNameValidator(),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "The DNS record type. Supported types: A, AAAA, CAA, CNAME, MX, NS, PTR, SOA, SRV, TXT, DS, SSHFP, TLSA, LOC, NAPTR, URI, CERT, DNAME.",
				Required:            true,
				Validators: []validator.String{
					DNSRecordTypeValidator(),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "The subdomain name for the record. Use @ for the root domain.",
				Required:            true,
				Validators: []validator.String{
					DNSRecordNameValidator(),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"data": schema.StringAttribute{
				MarkdownDescription: "The value of the record. Format varies by record type:\n" +
					"- A: IPv4 address (e.g., 192.0.2.1)\n" +
					"- AAAA: IPv6 address (e.g., 2001:db8::1)\n" +
					"- CNAME: Target domain name (e.g., example.com)\n" +
					"- MX: Mail server hostname (e.g., mail.example.com)\n" +
					"- TXT: Text content (e.g., 'v=spf1 include:_spf.google.com ~all')\n" +
					"- SRV: Target hostname (e.g., target.example.com)\n" +
					"- CAA: CAA record format (e.g., '0 issue letsencrypt.org')\n" +
					"- NS: Nameserver hostname (e.g., ns1.example.com)",
				Required: true,
				Validators: []validator.String{
					DNSRecordDataValidator(),
				},
			},
			"ttl": schema.Int32Attribute{
				MarkdownDescription: "Time to live in seconds (minimum 600, maximum 2592000).",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(3600),
				Validators: []validator.Int32{
					TTLValidator(),
				},
			},
			"priority": schema.Int32Attribute{
				MarkdownDescription: "Priority for MX, SRV, NAPTR, and URI records (0-65535). Lower values have higher priority.",
				Optional:            true,
				Validators: []validator.Int32{
					PriorityValidator(),
				},
				PlanModifiers: []planmodifier.Int32{
					int32planmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"port": schema.Int32Attribute{
				MarkdownDescription: "Port number for SRV records (1-65535).",
				Optional:            true,
				Validators: []validator.Int32{
					PortValidator(),
				},
				PlanModifiers: []planmodifier.Int32{
					int32planmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"weight": schema.Int32Attribute{
				MarkdownDescription: "Weight for SRV and URI records (0-65535). Used for load balancing among records with the same priority.",
				Optional:            true,
				Validators: []validator.Int32{
					WeightValidator(),
				},
				PlanModifiers: []planmodifier.Int32{
					int32planmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"service": schema.StringAttribute{
				MarkdownDescription: "Service name for SRV records (e.g., _sip, _http). Must start with underscore.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"protocol": schema.StringAttribute{
				MarkdownDescription: "Protocol for SRV records (e.g., _tcp, _udp). Must start with underscore.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"allow_overwrite": schema.BoolAttribute{
				MarkdownDescription: "Allow overwriting existing DNS records with the same type and name. " +
					"When set to true, if a DNS record with the same domain, type, and name already exists, " +
					"it will be replaced with the new configuration. When false (default), creation will fail " +
					"if a conflicting record exists. This is useful for migrating existing DNS records to Terraform management.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
		},
	}
}

func (r *DNSRecordResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*godaddy.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *godaddy.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *DNSRecordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DNSRecordResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate record type and required fields
	if err := r.validateRecord(&data); err != nil {
		resp.Diagnostics.AddError(
			"Invalid DNS Record Configuration",
			err.Error(),
		)
		return
	}

	record := r.modelToRecord(&data)

	domain := data.Domain.ValueString()
	recordType := data.Type.ValueString()
	name := data.Name.ValueString()

	unlock := lockRecordSet(domain, recordType, name)
	defer unlock()

	// Read the set inside the lock so the decision below uses current data.
	existingRecords, err := r.client.GetDNSRecordsByTypeAndName(ctx, domain, recordType, name)
	if err != nil && !godaddy.IsNotFound(err) {
		resp.Diagnostics.AddError(
			"Error Reading DNS Records",
			fmt.Sprintf("Could not check for existing DNS records before creating %s %s: %s", recordType, name, err),
		)
		return
	}

	action, err := planCreate(existingRecords, record, data.AllowOverwrite.ValueBool())
	if err != nil {
		resp.Diagnostics.AddError("DNS Record Already Exists", err.Error())
		return
	}

	switch action {
	case createAdd:
		tflog.Debug(ctx, "Adding DNS record", map[string]interface{}{
			"domain": domain, "type": recordType, "name": name, "existingRecords": len(existingRecords),
		})
		err = r.client.AddDNSRecord(ctx, domain, record)
	case createAdopt:
		tflog.Info(ctx, "Adopting existing DNS record with the same data (allow_overwrite = true)", map[string]interface{}{
			"domain": domain, "type": recordType, "name": name,
		})
		records := make([]godaddy.DNSRecord, len(existingRecords))
		copy(records, existingRecords)
		for i := range records {
			if sameRecordData(recordType, records[i].Data, record.Data) {
				records[i] = record
				break
			}
		}
		err = r.client.ReplaceDNSRecordsByTypeAndName(ctx, domain, recordType, name, records)
	case createReplace:
		tflog.Info(ctx, "Replacing existing single-value DNS record (allow_overwrite = true)", map[string]interface{}{
			"domain": domain, "type": recordType, "name": name,
		})
		err = r.client.ReplaceDNSRecordsByTypeAndName(ctx, domain, recordType, name, []godaddy.DNSRecord{record})
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Creating DNS Record",
			fmt.Sprintf("Could not create DNS record: %s", err),
		)
		return
	}

	// Generate ID
	data.ID = types.StringValue(fmt.Sprintf("%s/%s/%s", data.Domain.ValueString(), data.Type.ValueString(), data.Name.ValueString()))

	tflog.Trace(ctx, "created DNS record resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DNSRecordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DNSRecordResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	records, err := r.client.GetDNSRecordsByTypeAndName(ctx, data.Domain.ValueString(), data.Type.ValueString(), data.Name.ValueString())
	if err != nil {
		if godaddy.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading DNS Record",
			fmt.Sprintf("Could not read DNS record: %s", err),
		)
		return
	}

	// Keeping the old state when nothing matches would hide changes made
	// outside Terraform. Either show the live record or drop it from state so
	// the next apply recreates it.
	record, found := findStoredRecord(records, data.Type.ValueString(), data.Data.ValueString())
	if !found {
		tflog.Warn(ctx, "DNS record not found, removing it from state", map[string]interface{}{
			"domain": data.Domain.ValueString(),
			"type":   data.Type.ValueString(),
			"name":   data.Name.ValueString(),
		})
		resp.State.RemoveResource(ctx)
		return
	}
	r.updateModelFromRecord(&data, record)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DNSRecordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data DNSRecordResourceModel
	var state DNSRecordResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	unlock := lockRecordSet(data.Domain.ValueString(), data.Type.ValueString(), data.Name.ValueString())
	defer unlock()

	// Get all records of this type and name
	records, err := r.client.GetDNSRecordsByTypeAndName(ctx, data.Domain.ValueString(), data.Type.ValueString(), data.Name.ValueString())
	if err != nil {
		// Handle 404 errors with upsert behavior (create if doesn't exist)
		if godaddy.IsNotFound(err) {
			tflog.Info(ctx, "DNS record not found during update - creating new record (upsert behavior)", map[string]interface{}{
				"domain": data.Domain.ValueString(),
				"type":   data.Type.ValueString(),
				"name":   data.Name.ValueString(),
				"data":   data.Data.ValueString(),
			})
			
			// Create the record since it doesn't exist
			record := r.modelToRecord(&data)
			err = r.client.AddDNSRecord(ctx, data.Domain.ValueString(), record)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error Creating DNS Record During Update",
					fmt.Sprintf("Could not create DNS record during update (upsert): %s", err),
				)
				return
			}
			
			// Update the state with the new record
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
		// DIAGNOSTIC LOG: Non-404 errors
		tflog.Error(ctx, "Update() DNS record read failed", map[string]interface{}{
			"domain": data.Domain.ValueString(),
			"type":   data.Type.ValueString(),
			"name":   data.Name.ValueString(),
			"error":  err.Error(),
		})
		resp.Diagnostics.AddError(
			"Error Reading DNS Records",
			fmt.Sprintf("Could not read DNS records: %s", err),
		)
		return
	}

	// Update the matching record
	updated := false
	for i, record := range records {
		if sameRecordData(data.Type.ValueString(), record.Data, state.Data.ValueString()) {
			// Update this record
			records[i] = r.modelToRecord(&data)
			updated = true
			break
		}
	}

	if !updated {
		// Enregistrement spécifique non trouvé dans la liste - comportement upsert (créer)
		tflog.Info(ctx, "Specific DNS record data not found in existing records - creating new record (upsert behavior)", map[string]interface{}{
			"domain": data.Domain.ValueString(),
			"type":   data.Type.ValueString(),
			"name":   data.Name.ValueString(),
			"data":   data.Data.ValueString(),
			"existingRecordsCount": len(records),
		})
		
		// Ajouter le nouvel enregistrement à la liste existante
		newRecord := r.modelToRecord(&data)
		records = append(records, newRecord)
		
		// Remplacer tous les enregistrements avec la nouvelle liste incluant le nouvel enregistrement
		err = r.client.ReplaceDNSRecordsByTypeAndName(ctx, data.Domain.ValueString(), data.Type.ValueString(), data.Name.ValueString(), records)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Creating DNS Record During Update",
				fmt.Sprintf("Could not create DNS record during update (upsert): %s", err),
			)
			return
		}
		
		// Update the state with the new record
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Replace all records of this type and name
	err = r.client.ReplaceDNSRecordsByTypeAndName(ctx, data.Domain.ValueString(), data.Type.ValueString(), data.Name.ValueString(), records)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating DNS Record",
			fmt.Sprintf("Could not update DNS record: %s", err),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DNSRecordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DNSRecordResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	unlock := lockRecordSet(data.Domain.ValueString(), data.Type.ValueString(), data.Name.ValueString())
	defer unlock()

	// Get all records of this type and name
	records, err := r.client.GetDNSRecordsByTypeAndName(ctx, data.Domain.ValueString(), data.Type.ValueString(), data.Name.ValueString())
	if err != nil {
		if godaddy.IsNotFound(err) {
			// Already deleted
			tflog.Debug(ctx, "DNS record already deleted (404)", map[string]interface{}{
				"domain": data.Domain.ValueString(),
				"type":   data.Type.ValueString(),
				"name":   data.Name.ValueString(),
			})
			return
		}
		resp.Diagnostics.AddError(
			"Error Reading DNS Records",
			fmt.Sprintf("Could not read DNS records: %s", err),
		)
		return
	}

	tflog.Debug(ctx, "Retrieved DNS records for deletion", map[string]interface{}{
		"domain":      data.Domain.ValueString(),
		"type":        data.Type.ValueString(),
		"name":        data.Name.ValueString(),
		"recordCount": len(records),
		"targetData":  data.Data.ValueString(),
	})

	// Check for empty records slice to prevent panic
	if len(records) == 0 {
		tflog.Warn(ctx, "No DNS records found for deletion, but no 404 error returned", map[string]interface{}{
			"domain": data.Domain.ValueString(),
			"type":   data.Type.ValueString(),
			"name":   data.Name.ValueString(),
		})
		// Already deleted or doesn't exist
		return
	}

	// Remove the matching record
	// CRITICAL FIX: Use len(records) instead of len(records)-1 for capacity
	// This prevents panic when len(records) could be 0
	newRecords := make([]godaddy.DNSRecord, 0, len(records))
	var recordFound bool
	for _, record := range records {
		if !sameRecordData(data.Type.ValueString(), record.Data, data.Data.ValueString()) {
			newRecords = append(newRecords, record)
		} else {
			recordFound = true
			tflog.Debug(ctx, "Found matching DNS record to delete", map[string]interface{}{
				"domain": data.Domain.ValueString(),
				"type":   data.Type.ValueString(),
				"name":   data.Name.ValueString(),
				"data":   record.Data,
			})
		}
	}

	if !recordFound {
		tflog.Warn(ctx, "Target DNS record not found in retrieved records", map[string]interface{}{
			"domain":     data.Domain.ValueString(),
			"type":       data.Type.ValueString(),
			"name":       data.Name.ValueString(),
			"targetData": data.Data.ValueString(),
		})
		// Record already deleted or doesn't match
		return
	}

	tflog.Debug(ctx, "Processing DNS record deletion", map[string]interface{}{
		"domain":         data.Domain.ValueString(),
		"type":           data.Type.ValueString(),
		"name":           data.Name.ValueString(),
		"remainingCount": len(newRecords),
		"willDeleteAll":  len(newRecords) == 0,
	})

	if len(newRecords) == 0 {
		// Delete all records of this type and name
		tflog.Debug(ctx, "Deleting all DNS records of this type and name")
		err = r.client.DeleteDNSRecord(ctx, data.Domain.ValueString(), data.Type.ValueString(), data.Name.ValueString())
	} else {
		// Replace with remaining records
		tflog.Debug(ctx, "Replacing with remaining DNS records", map[string]interface{}{
			"remainingRecords": len(newRecords),
		})
		err = r.client.ReplaceDNSRecordsByTypeAndName(ctx, data.Domain.ValueString(), data.Type.ValueString(), data.Name.ValueString(), newRecords)
	}

	if err != nil {
		if godaddy.IsNotFound(err) {
			tflog.Debug(ctx, "DNS record already deleted during final operation (404)")
		} else {
			resp.Diagnostics.AddError(
				"Error Deleting DNS Record",
				fmt.Sprintf("Could not delete DNS record: %s", err),
			)
			return
		}
	}

	tflog.Debug(ctx, "DNS record deletion completed successfully", map[string]interface{}{
		"domain": data.Domain.ValueString(),
		"type":   data.Type.ValueString(),
		"name":   data.Name.ValueString(),
		"data":   data.Data.ValueString(),
	})
}

func (r *DNSRecordResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Expected format: domain/type/name/data
	// Alternative format: domain/type/name (will auto-detect data from existing records)
	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 && len(parts) != 4 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be in the format: domain/type/name/data or domain/type/name (auto-detect data)",
		)
		return
	}

	domain := parts[0]
	recordType := parts[1]
	name := parts[2]
	var data string

	if err := ValidateDomainName(domain); err != nil {
		resp.Diagnostics.AddError("Invalid Import ID", err.Error())
		return
	}
	if err := ValidateRecordName(name); err != nil {
		resp.Diagnostics.AddError("Invalid Import ID", err.Error())
		return
	}

	if len(parts) == 4 {
		// Full format provided
		data = parts[3]
	} else {
		// Auto-detect data from existing records
		records, err := r.client.GetDNSRecordsByTypeAndName(ctx, domain, recordType, name)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error Reading DNS Records for Import",
				fmt.Sprintf("Could not read DNS records for %s/%s/%s: %s", domain, recordType, name, err),
			)
			return
		}

		if len(records) == 0 {
			resp.Diagnostics.AddError(
				"No DNS Records Found",
				fmt.Sprintf("No DNS records found for %s/%s/%s", domain, recordType, name),
			)
			return
		}

		if len(records) > 1 {
			resp.Diagnostics.AddError(
				"Multiple DNS Records Found",
				fmt.Sprintf("Multiple DNS records found for %s/%s/%s. Please specify the data value: %s/type/name/data", domain, recordType, name, domain),
			)
			return
		}

		data = records[0].Data
	}

	// Generate the full ID
	importID := fmt.Sprintf("%s/%s/%s/%s", domain, recordType, name, data)

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain"), domain)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("type"), recordType)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("data"), data)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), importID)...)
}

func (r *DNSRecordResource) modelToRecord(model *DNSRecordResourceModel) godaddy.DNSRecord {
	record := godaddy.DNSRecord{
		Type: model.Type.ValueString(),
		Name: model.Name.ValueString(),
		Data: model.Data.ValueString(),
		TTL:  int(model.TTL.ValueInt32()),
	}

	if !model.Priority.IsNull() {
		priority := int(model.Priority.ValueInt32())
		record.Priority = &priority
	}

	if !model.Port.IsNull() {
		port := int(model.Port.ValueInt32())
		record.Port = &port
	}

	if !model.Weight.IsNull() {
		weight := int(model.Weight.ValueInt32())
		record.Weight = &weight
	}

	if !model.Service.IsNull() {
		service := model.Service.ValueString()
		record.Service = &service
	}

	if !model.Protocol.IsNull() {
		protocol := model.Protocol.ValueString()
		record.Protocol = &protocol
	}

	return record
}

func (r *DNSRecordResource) updateModelFromRecord(model *DNSRecordResourceModel, record *godaddy.DNSRecord) {
	model.Type = types.StringValue(record.Type)
	model.Name = types.StringValue(record.Name)
	model.Data = types.StringValue(record.Data)
	model.TTL = types.Int32Value(int32(record.TTL))

	if record.Priority != nil {
		model.Priority = types.Int32Value(int32(*record.Priority))
	} else {
		model.Priority = types.Int32Null()
	}

	if record.Port != nil {
		model.Port = types.Int32Value(int32(*record.Port))
	} else {
		model.Port = types.Int32Null()
	}

	if record.Weight != nil {
		model.Weight = types.Int32Value(int32(*record.Weight))
	} else {
		model.Weight = types.Int32Null()
	}

	if record.Service != nil {
		model.Service = types.StringValue(*record.Service)
	} else {
		model.Service = types.StringNull()
	}

	if record.Protocol != nil {
		model.Protocol = types.StringValue(*record.Protocol)
	} else {
		model.Protocol = types.StringNull()
	}
}

func (r *DNSRecordResource) validateRecord(model *DNSRecordResourceModel) error {
	recordType := model.Type.ValueString()
	requiredFields := godaddy.DNSRecordRequiredFields(recordType)

	// Validate data format
	if err := godaddy.ValidateRecordData(recordType, model.Data.ValueString()); err != nil {
		return fmt.Errorf("invalid data for %s record: %w", recordType, err)
	}

	// Check required fields based on record type
	if requiredFields["priority"] && model.Priority.IsNull() {
		return fmt.Errorf("%s records require a priority value", recordType)
	}

	if requiredFields["port"] && model.Port.IsNull() {
		return fmt.Errorf("%s records require a port value", recordType)
	}

	if requiredFields["weight"] && model.Weight.IsNull() {
		return fmt.Errorf("%s records require a weight value", recordType)
	}

	// SRV-specific validations
	if recordType == "SRV" {
		if model.Service.IsNull() || model.Protocol.IsNull() {
			return fmt.Errorf("SRV records require both service and protocol fields")
		}

		service := model.Service.ValueString()
		protocol := model.Protocol.ValueString()

		if !strings.HasPrefix(service, "_") {
			return fmt.Errorf("SRV service must start with underscore (e.g., _sip)")
		}

		if !strings.HasPrefix(protocol, "_") {
			return fmt.Errorf("SRV protocol must start with underscore (e.g., _tcp)")
		}
	}

	return nil
}
