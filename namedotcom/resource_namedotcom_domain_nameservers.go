package namedotcom

import (
	"context"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/namedotcom/go/v4/namecom"
)

// Ensure the resource satisfies the required framework interfaces.
var (
	_ resource.Resource                 = (*domainNameServersResource)(nil)
	_ resource.ResourceWithConfigure    = (*domainNameServersResource)(nil)
	_ resource.ResourceWithUpgradeState = (*domainNameServersResource)(nil)
)

// nameserversSchemaVersion is the current schema version. It was bumped from 0
// to 1 when nameservers switched from a list to a set, so state written by the
// SDKv2 provider (which also used version 1) is read without an upgrade.
const nameserversSchemaVersion = 1

// domainNameServersResource manages the nameservers configured for a domain.
type domainNameServersResource struct {
	client *namecom.NameCom
}

// nameserversModel maps the nameservers schema to a Go struct.
type nameserversModel struct {
	ID          types.String `tfsdk:"id"`
	DomainName  types.String `tfsdk:"domain_name"`
	Nameservers types.Set    `tfsdk:"nameservers"`
}

// NewDomainNameServersResource is the resource factory registered with the provider.
//
//nolint:ireturn // The framework contract requires returning the resource.Resource interface.
func NewDomainNameServersResource() resource.Resource {
	return &domainNameServersResource{}
}

func (r *domainNameServersResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain_nameservers"
}

//nolint:lll // Attribute descriptions are intentionally verbose for the registry docs.
func (r *domainNameServersResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: nameserversSchemaVersion,
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   descIDIsDomainName,
			},
			keyDomainName: schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{requiresReplaceOnDNSChange()},
				Description:   "DomainName is the punycode encoded value of the domain name. Changing this forces a new resource.",
			},
			keyNameservers: schema.SetAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "Nameservers is the set of nameservers for this domain. Order is not significant; the registry treats nameservers as a set. If unspecified it defaults to your account default nameservers.",
			},
		},
	}
}

func (r *domainNameServersResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client, ok := configureClient(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}

	r.client = client
}

func (r *domainNameServersResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan nameserversModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	nameservers, diags := extractNameservers(ctx, plan.Nameservers)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := setNameserversAPI(ctx, r.client, plan.DomainName.ValueString(), nameservers)
	if err != nil {
		resp.Diagnostics.AddError("Error setting nameservers", err.Error())

		return
	}

	r.refreshState(ctx, &plan, &resp.State, &resp.Diagnostics)
}

func (r *domainNameServersResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state nameserversModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.refreshState(ctx, &state, &resp.State, &resp.Diagnostics)
}

func (r *domainNameServersResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan nameserversModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	nameservers, diags := extractNameservers(ctx, plan.Nameservers)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := setNameserversAPI(ctx, r.client, plan.DomainName.ValueString(), nameservers)
	if err != nil {
		if isNotFoundError(err) {
			resp.State.RemoveResource(ctx)

			return
		}

		resp.Diagnostics.AddError("Error setting nameservers", err.Error())

		return
	}

	r.refreshState(ctx, &plan, &resp.State, &resp.Diagnostics)
}

func (r *domainNameServersResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state nameserversModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Resetting the nameservers to an empty list restores the account defaults.
	err := setNameserversAPI(ctx, r.client, state.DomainName.ValueString(), nil)
	if err != nil && !isNotFoundError(err) {
		resp.Diagnostics.AddError("Error resetting nameservers", err.Error())

		return
	}
}

// UpgradeState ports the SDKv2 v0 -> v1 migration: nameservers was a list of
// strings in schema version 0 and is a set of strings in version 1. The
// framework does not coerce list to set automatically, so the values are
// copied explicitly.
func (r *domainNameServersResource) UpgradeState(_ context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema: &schema.Schema{
				Attributes: map[string]schema.Attribute{
					keyID:         schema.StringAttribute{Computed: true},
					keyDomainName: schema.StringAttribute{Required: true},
					keyNameservers: schema.ListAttribute{
						ElementType: types.StringType,
						Optional:    true,
						Computed:    true,
					},
				},
			},
			StateUpgrader: upgradeNameserversV0ToV1,
		},
	}
}

// nameserversModelV0 is the schema-version-0 shape, where nameservers was a list.
type nameserversModelV0 struct {
	ID          types.String `tfsdk:"id"`
	DomainName  types.String `tfsdk:"domain_name"`
	Nameservers types.List   `tfsdk:"nameservers"`
}

func upgradeNameserversV0ToV1(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
	var prior nameserversModelV0

	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}

	nameservers, diags := types.SetValue(types.StringType, prior.Nameservers.Elements())
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &nameserversModel{
		ID:          prior.ID,
		DomainName:  prior.DomainName,
		Nameservers: nameservers,
	})...)
}

// refreshState fetches the domain from the API and writes the current
// nameservers into state, removing the resource on a not-found error.
func (r *domainNameServersResource) refreshState(
	ctx context.Context,
	model *nameserversModel,
	state *tfsdk.State,
	diags *diag.Diagnostics,
) {
	domain, found, err := readNameserversAPI(ctx, r.client, model.DomainName.ValueString())
	if err != nil {
		diags.AddError("Error reading domain", err.Error())

		return
	}

	if !found {
		state.RemoveResource(ctx)

		return
	}

	nameservers, convertDiags := types.SetValueFrom(ctx, types.StringType, domain.Nameservers)
	diags.Append(convertDiags...)

	if diags.HasError() {
		return
	}

	model.ID = types.StringValue(domain.DomainName)
	model.DomainName = types.StringValue(domain.DomainName)
	model.Nameservers = nameservers

	diags.Append(state.Set(ctx, model)...)
}

// extractNameservers reads a nameservers set into a slice, treating a null or
// unknown value as an empty list (which resets the domain to account defaults).
func extractNameservers(ctx context.Context, set types.Set) ([]string, diag.Diagnostics) {
	if set.IsNull() || set.IsUnknown() {
		return nil, nil
	}

	var nameservers []string

	diags := set.ElementsAs(ctx, &nameservers, false)

	return nameservers, diags
}

// setNameserversAPI sets the nameservers for a domain via the Name.com API.
func setNameserversAPI(ctx context.Context, client *namecom.NameCom, domainName string, nameservers []string) error {
	err := RespectRateLimits(ctx)
	if err != nil {
		return errors.Wrap(err, "rate limiting error")
	}

	_, err = client.SetNameservers(&namecom.SetNameserversRequest{
		DomainName:  domainName,
		Nameservers: nameservers,
	})
	if err != nil {
		return errors.Wrap(err, "Error SetNameservers")
	}

	return nil
}

// readNameserversAPI fetches a domain via the Name.com API. The boolean result
// is false when the domain no longer exists.
func readNameserversAPI(ctx context.Context, client *namecom.NameCom, domainName string) (*namecom.Domain, bool, error) {
	err := RespectRateLimits(ctx)
	if err != nil {
		return nil, false, errors.Wrap(err, "rate limiting error")
	}

	domain, err := client.GetDomain(&namecom.GetDomainRequest{
		DomainName: domainName,
	})
	if err != nil {
		if isNotFoundError(err) {
			return nil, false, nil
		}

		return nil, false, errors.Wrap(err, "Error GetDomain")
	}

	return domain, true, nil
}

// isNotFoundError reports whether the API error indicates the resource (domain,
// record, or DNSSEC key) no longer exists, so callers can drop it from state.
// The Name.com SDK surfaces the API's error message and does not include the
// HTTP status code in the error string, so this matches on the "not found"
// message text rather than a numeric status that would risk false positives.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(strings.ToLower(err.Error()), "not found")
}
