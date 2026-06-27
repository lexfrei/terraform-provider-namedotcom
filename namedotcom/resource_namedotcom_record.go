package namedotcom

import (
	"context"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/namedotcom/go/v4/namecom"
)

// Ensure the resource satisfies the required framework interfaces.
var (
	_ resource.Resource                = (*recordResource)(nil)
	_ resource.ResourceWithConfigure   = (*recordResource)(nil)
	_ resource.ResourceWithImportState = (*recordResource)(nil)
)

// recordResource manages a single Name.com DNS record.
type recordResource struct {
	client *namecom.NameCom
}

// recordModel maps the record schema to a Go struct.
type recordModel struct {
	ID         types.String `tfsdk:"id"`
	RecordID   types.Int32  `tfsdk:"record_id"`
	DomainName types.String `tfsdk:"domain_name"`
	Host       types.String `tfsdk:"host"`
	RecordType types.String `tfsdk:"record_type"`
	Answer     types.String `tfsdk:"answer"`
	Priority   types.Int32  `tfsdk:"priority"`
}

// NewRecordResource is the resource factory registered with the provider.
//
//nolint:ireturn // The framework contract requires returning the resource.Resource interface.
func NewRecordResource() resource.Resource {
	return &recordResource{}
}

func (r *recordResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_record"
}

func (r *recordResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "Unique record id assigned by Name.com.",
			},
			keyRecordID: schema.Int32Attribute{
				Computed:      true,
				PlanModifiers: []planmodifier.Int32{int32planmodifier.UseStateForUnknown()},
				Description:   "Unique record id assigned by Name.com (numeric form of `id`).",
			},
			keyDomainName: schema.StringAttribute{
				Optional:      true,
				PlanModifiers: []planmodifier.String{requiresReplaceOnDNSChange()},
				Description:   "DomainName is the zone that the record belongs to. Changing this forces a new resource.",
			},
			keyHost: schema.StringAttribute{
				Optional:    true,
				Description: "Host is the hostname relative to the zone.",
			},
			keyRecordType: schema.StringAttribute{
				Optional:      true,
				PlanModifiers: []planmodifier.String{requiresReplaceOnDNSChange()},
				Description:   "Type is one of the following: A, AAAA, ANAME, CNAME, MX, NS, SRV, or TXT. Changing this forces a new resource.",
			},
			keyAnswer: schema.StringAttribute{
				Optional: true,
				//nolint:lll // One sentence covering every supported record type.
				Description: "Answer is the record value: the IP address for A and AAAA records, the target for ANAME, CNAME, MX, NS, and SRV records, or the text for TXT records.",
			},
			keyPriority: schema.Int32Attribute{
				Optional: true,
				//nolint:lll // One sentence describing where priority applies.
				Description: "Priority is used by MX and SRV records, where a lower value is preferred; it is ignored for all other record types. Valid range is 0-65535.",
			},
		},
	}
}

func (r *recordResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client, ok := configureClient(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}

	r.client = client
}

func (r *recordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan recordModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	record, err := createRecordAPI(ctx, r.client, apiRecordFromModel(plan))
	if err != nil {
		resp.Diagnostics.AddError("Error creating record", err.Error())

		return
	}

	state := recordCreateState(plan, record)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *recordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state recordModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	recordID, err := parseRecordID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid record ID in state", err.Error())

		return
	}

	record, err := readRecordAPI(ctx, r.client, state.DomainName.ValueString(), recordID)
	if err != nil {
		// The record was deleted outside Terraform: drop it from state so the
		// next plan recreates it instead of failing.
		if isNotFoundError(err) {
			resp.State.RemoveResource(ctx)

			return
		}

		resp.Diagnostics.AddError("Error reading record", err.Error())

		return
	}

	refreshed := recordReadState(state, record)

	resp.Diagnostics.Append(resp.State.Set(ctx, &refreshed)...)
}

func (r *recordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan recordModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state recordModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	recordID, err := parseRecordID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid record ID in state", err.Error())

		return
	}

	record, err := updateRecordAPI(ctx, r.client, recordID, apiRecordFromModel(plan))
	if err != nil {
		// The record was deleted outside Terraform between plan and apply: drop
		// it from state so the next plan recreates it, matching the Read path.
		if isNotFoundError(err) {
			resp.State.RemoveResource(ctx)

			return
		}

		resp.Diagnostics.AddError("Error updating record", err.Error())

		return
	}

	updated := recordCreateState(plan, record)

	resp.Diagnostics.Append(resp.State.Set(ctx, &updated)...)
}

func (r *recordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state recordModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	recordID, err := parseRecordID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid record ID in state", err.Error())

		return
	}

	err = deleteRecordAPI(ctx, r.client, state.DomainName.ValueString(), recordID)
	if err != nil {
		resp.Diagnostics.AddError("Error deleting record", err.Error())

		return
	}
}

// ImportState parses a "domain:id" identifier, seeding domain_name and id so
// the subsequent Read can refresh the remaining attributes.
func (r *recordResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	domainName, recordID, err := resourceRecordImporterParseID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())

		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(keyDomainName), domainName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(keyID), recordID)...)
}

// resourceRecordImporterParseID splits an import identifier of the form
// "domain:id" into its two parts.
func resourceRecordImporterParseID(id string) (domain, recordID string, err error) {
	// Split the ID into two parts, the domain and the record ID.
	//nolint:mnd // 2 is the expected number of parts
	parts := strings.SplitN(id, ":", 2)

	// Check that the ID is in the expected format.
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errors.New("unexpected format of ID, expected domain:id")
	}

	return parts[0], parts[1], nil
}

// parseRecordID converts the string resource ID into the int32 the API expects.
func parseRecordID(id string) (int32, error) {
	parsed, err := strconv.ParseInt(id, 10, 32)
	if err != nil {
		return 0, errors.Wrap(err, "error converting record ID")
	}

	return int32(parsed), nil
}

// recordCreateState builds the state to persist after a Create or Update.
// The user-controlled attributes are echoed from the plan: unlike SDKv2, the
// framework does not enable the legacy type-system leniency, so writing the
// API's canonical form into a non-computed attribute that differs from the
// configured value would fail Terraform's "inconsistent result after apply"
// check. Only the server-assigned identifiers are taken from the API response.
func recordCreateState(plan recordModel, record *namecom.Record) recordModel {
	plan.ID = types.StringValue(strconv.Itoa(int(record.ID)))
	plan.RecordID = types.Int32Value(record.ID)

	return plan
}

// recordReadState refreshes the state from the API record. For the
// user-controlled attributes it keeps the representation already in state when
// that is semantically equal to the API value, so Name.com's canonicalization
// (upper-cased type, trailing-dot answer) does not surface as perpetual drift.
// A genuinely different value — or an empty one during import — is adopted from
// the API so real drift is still detected.
func recordReadState(state recordModel, record *namecom.Record) recordModel {
	state.ID = types.StringValue(strconv.Itoa(int(record.ID)))
	state.RecordID = types.Int32Value(record.ID)
	// domain_name is the resource key and carries RequiresReplace; GetRecord was
	// queried by it, so the returned domain always matches. It is deliberately
	// not adopted from the API response, so a punycode/case difference can never
	// trigger a spurious replacement on refresh.
	state.Host = reconcileHostValue(state.Host, record.Host)
	state.RecordType = reconcileDNSValue(state.RecordType, record.Type)
	state.Answer = reconcileDNSValue(state.Answer, record.Answer)
	state.Priority = reconcilePriority(state.Priority, record.Priority)

	return state
}

// reconcilePriority keeps an unset priority (null) when the API reports the
// zero default: record types that do not use priority always report 0, which
// would otherwise surface as perpetual drift against a config that omits the
// attribute. A configured value that matches the API is preserved as-is; any
// genuine difference is adopted from the API, so an MX record's priority is
// populated on import and real out-of-band changes are still detected.
func reconcilePriority(prior types.Int32, apiValue uint32) types.Int32 {
	if prior.IsNull() && apiValue == 0 {
		return prior
	}

	if !prior.IsNull() && priorityToUint32(prior) == apiValue {
		return prior
	}

	return types.Int32Value(priorityToInt32(apiValue))
}

// reconcileDNSValue keeps the prior state value when it is semantically equal
// to the API value (DNS names are case-insensitive and a trailing dot is not
// significant); otherwise it adopts the API value.
func reconcileDNSValue(prior types.String, apiValue string) types.String {
	if !prior.IsNull() && dnsEqual(prior.ValueString(), apiValue) {
		return prior
	}

	return types.StringValue(apiValue)
}

// reconcileHostValue is reconcileDNSValue for the host attribute, additionally
// treating the apex forms "@" and "" as equivalent.
func reconcileHostValue(prior types.String, apiValue string) types.String {
	if !prior.IsNull() && hostEqual(prior.ValueString(), apiValue) {
		return prior
	}

	return types.StringValue(apiValue)
}

func dnsEqual(a, b string) bool {
	return strings.EqualFold(strings.TrimSuffix(a, "."), strings.TrimSuffix(b, "."))
}

func hostEqual(a, b string) bool {
	return strings.EqualFold(normalizeHost(a), normalizeHost(b))
}

// normalizeHost treats the apex host "@" as the empty host.
func normalizeHost(host string) string {
	if host == "@" {
		return ""
	}

	return host
}

// dnsReplaceDescription documents the semantic RequiresReplace behaviour.
const dnsReplaceDescription = "Changing this to a different value forces a new resource; " +
	"differences that are only DNS canonicalization (letter case or a trailing dot) do not."

// requiresReplaceOnDNSChange forces replacement only when the attribute changes
// semantically. A difference that is purely DNS canonicalization (letter case
// or a trailing dot) does not trigger replacement, so upgrading from state
// written by the SDKv2 provider — which stored the API's canonical form — does
// not destroy and recreate resources over a cosmetic mismatch.
//
//nolint:ireturn // The framework schema requires a planmodifier.String value.
func requiresReplaceOnDNSChange() planmodifier.String {
	return stringplanmodifier.RequiresReplaceIf(
		func(_ context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
			resp.RequiresReplace = !dnsEqual(req.StateValue.ValueString(), req.PlanValue.ValueString())
		},
		dnsReplaceDescription,
		dnsReplaceDescription,
	)
}

// apiRecordFromModel builds the Name.com API record from the user-controlled
// attributes of the plan. The server-assigned id is set separately by the
// callers that need it (Update).
func apiRecordFromModel(model recordModel) *namecom.Record {
	return &namecom.Record{
		DomainName: model.DomainName.ValueString(),
		Host:       model.Host.ValueString(),
		Type:       model.RecordType.ValueString(),
		Answer:     model.Answer.ValueString(),
		Priority:   priorityToUint32(model.Priority),
	}
}

// priorityToUint32 converts the optional priority attribute to the uint32 the
// API expects. An unset, unknown, or negative value maps to 0, which the API
// ignores for record types that do not use priority.
func priorityToUint32(priority types.Int32) uint32 {
	if priority.IsNull() || priority.IsUnknown() || priority.ValueInt32() < 0 {
		return 0
	}

	//nolint:gosec // Guarded above: the value is non-negative, so it fits in uint32.
	return uint32(priority.ValueInt32())
}

// priorityToInt32 converts an API priority back to the schema's int32. DNS
// priority is a 16-bit field, so the value always fits in int32.
func priorityToInt32(apiValue uint32) int32 {
	//nolint:gosec // DNS priority is a 16-bit field; it always fits in int32.
	return int32(apiValue)
}

// createRecordAPI creates a record via the Name.com API.
func createRecordAPI(ctx context.Context, client *namecom.NameCom, input *namecom.Record) (*namecom.Record, error) {
	err := RespectRateLimits(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "rate limiting error")
	}

	record, err := client.CreateRecord(input)
	if err != nil {
		return nil, errors.Wrap(err, "Error CreateRecord")
	}

	return record, nil
}

// readRecordAPI fetches a record via the Name.com API.
func readRecordAPI(ctx context.Context, client *namecom.NameCom, domainName string, recordID int32) (*namecom.Record, error) {
	err := RespectRateLimits(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "rate limiting error")
	}

	record, err := client.GetRecord(&namecom.GetRecordRequest{
		DomainName: domainName,
		ID:         recordID,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Error GetRecord")
	}

	return record, nil
}

// updateRecordAPI updates a record via the Name.com API.
func updateRecordAPI(ctx context.Context, client *namecom.NameCom, recordID int32, input *namecom.Record) (*namecom.Record, error) {
	err := RespectRateLimits(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "rate limiting error")
	}

	input.ID = recordID

	record, err := client.UpdateRecord(input)
	if err != nil {
		return nil, errors.Wrap(err, "Error UpdateRecord")
	}

	return record, nil
}

// deleteRecordAPI deletes a record via the Name.com API.
func deleteRecordAPI(ctx context.Context, client *namecom.NameCom, domainName string, recordID int32) error {
	err := RespectRateLimits(ctx)
	if err != nil {
		return errors.Wrap(err, "rate limiting error")
	}

	_, err = client.DeleteRecord(&namecom.DeleteRecordRequest{
		DomainName: domainName,
		ID:         recordID,
	})
	if err != nil {
		return errors.Wrap(err, "Error DeleteRecord")
	}

	return nil
}
