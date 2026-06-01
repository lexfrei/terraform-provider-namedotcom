package namedotcom

import (
	"context"
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
	_ resource.Resource                = (*dnssecResource)(nil)
	_ resource.ResourceWithConfigure   = (*dnssecResource)(nil)
	_ resource.ResourceWithImportState = (*dnssecResource)(nil)
)

// dnssecResource manages DNSSEC settings for a domain.
type dnssecResource struct {
	client *namecom.NameCom
}

// dnssecModel maps the DNSSEC schema to a Go struct.
type dnssecModel struct {
	ID         types.String `tfsdk:"id"`
	DomainName types.String `tfsdk:"domain_name"`
	KeyTag     types.Int32  `tfsdk:"key_tag"`
	Algorithm  types.Int32  `tfsdk:"algorithm"`
	DigestType types.Int32  `tfsdk:"digest_type"`
	Digest     types.String `tfsdk:"digest"`
}

// NewDNSSECResource is the resource factory registered with the provider.
//
//nolint:ireturn // The framework contract requires returning the resource.Resource interface.
func NewDNSSECResource() resource.Resource {
	return &dnssecResource{}
}

func (r *dnssecResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dnssec"
}

func (r *dnssecResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			keyID: schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   descIDIsDomainName,
			},
			keyDomainName: schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{requiresReplaceOnDNSChange()},
				Description:   "DomainName is the zone that the DNSSEC belongs to. Changing this forces a new resource.",
			},
			keyKeyTag: schema.Int32Attribute{
				Required:      true,
				PlanModifiers: []planmodifier.Int32{int32planmodifier.RequiresReplace()},
				Description:   "KeyTag contains the key tag value of the DNSKEY RR that validates this signature. Changing this forces a new resource.",
			},
			keyAlgorithm: schema.Int32Attribute{
				Required:      true,
				PlanModifiers: []planmodifier.Int32{int32planmodifier.RequiresReplace()},
				Description:   "Algorithm is an integer identifying the algorithm used for signing. Changing this forces a new resource.",
			},
			keyDigestType: schema.Int32Attribute{
				Required:      true,
				PlanModifiers: []planmodifier.Int32{int32planmodifier.RequiresReplace()},
				Description:   "DigestType is an integer identifying the algorithm used to create the digest. Changing this forces a new resource.",
			},
			keyDigest: schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{requiresReplaceOnDNSChange()},
				Description:   "Digest is a digest of the DNSKEY RR that is registered with the registry. Changing this forces a new resource.",
			},
		},
	}
}

func (r *dnssecResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client, ok := configureClient(req.ProviderData, &resp.Diagnostics)
	if !ok {
		return
	}

	r.client = client
}

func (r *dnssecResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dnssecModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := createDNSSECAPI(
		ctx, r.client,
		plan.DomainName.ValueString(),
		plan.KeyTag.ValueInt32(),
		plan.Algorithm.ValueInt32(),
		plan.DigestType.ValueInt32(),
		plan.Digest.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Error creating DNSSEC", err.Error())

		return
	}

	plan.ID = plan.DomainName

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dnssecResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dnssecModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	dnssec, err := readDNSSECAPI(ctx, r.client, state.DomainName.ValueString(), state.Digest.ValueString())
	if err != nil {
		// The DNSSEC key was removed outside Terraform: drop it from state so
		// the next plan recreates it instead of failing.
		if isNotFoundError(err) {
			resp.State.RemoveResource(ctx)

			return
		}

		resp.Diagnostics.AddError("Error reading DNSSEC", err.Error())

		return
	}

	// domain_name and digest are the immutable lookup keys (RequiresReplace) and
	// were used to query the API, so the stored values are kept rather than
	// adopted from the response — a canonicalization difference (e.g. digest
	// case) must not force a spurious replacement on refresh. Only the numeric
	// fields are refreshed; they are populated from the API on import.
	state.ID = state.DomainName
	state.KeyTag = types.Int32Value(dnssec.KeyTag)
	state.Algorithm = types.Int32Value(dnssec.Algorithm)
	state.DigestType = types.Int32Value(dnssec.DigestType)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is required by the resource.Resource interface but is never reached:
// every attribute forces replacement, so the framework destroys and recreates
// instead of updating in place. It simply round-trips the plan into state.
func (r *dnssecResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan dnssecModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dnssecResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dnssecModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := deleteDNSSECAPI(ctx, r.client, state.DomainName.ValueString(), state.Digest.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting DNSSEC", err.Error())

		return
	}
}

// ImportState parses a "DomainName_Digest" identifier, seeding the domain_name
// and digest so the subsequent Read can populate the remaining attributes.
func (r *dnssecResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	domainName, digest, err := resourceDNSSECImporterParseID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())

		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(keyDomainName), domainName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(keyDigest), digest)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(keyID), domainName)...)
}

// resourceDNSSECImporterParseID parses an import identifier of the form
// "DomainName_Digest". It splits at the last underscore so that domain names
// containing underscores are handled correctly.
func resourceDNSSECImporterParseID(importID string) (domainName, digest string, err error) {
	idx := strings.LastIndex(importID, "_")
	if idx <= 0 || idx == len(importID)-1 {
		return "", "", errors.New("unexpected format of ID, expected DomainName_Digest")
	}

	return importID[:idx], importID[idx+1:], nil
}

// createDNSSECAPI registers a DNSSEC key via the Name.com API.
func createDNSSECAPI(
	ctx context.Context,
	client *namecom.NameCom,
	domainName string,
	keyTag, algorithm, digestType int32,
	digest string,
) error {
	err := RespectRateLimits(ctx)
	if err != nil {
		return errors.Wrap(err, "rate limiting error")
	}

	_, err = client.CreateDNSSEC(&namecom.DNSSEC{
		DomainName: domainName,
		KeyTag:     keyTag,
		Algorithm:  algorithm,
		DigestType: digestType,
		Digest:     digest,
	})
	if err != nil {
		return errors.Wrap(err, "Error CreateDNSSEC")
	}

	return nil
}

// readDNSSECAPI fetches a DNSSEC key via the Name.com API.
func readDNSSECAPI(ctx context.Context, client *namecom.NameCom, domainName, digest string) (*namecom.DNSSEC, error) {
	err := RespectRateLimits(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "rate limiting error")
	}

	dnssec, err := client.GetDNSSEC(&namecom.GetDNSSECRequest{
		DomainName: domainName,
		Digest:     digest,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Error GetDNSSEC")
	}

	return dnssec, nil
}

// deleteDNSSECAPI removes a DNSSEC key via the Name.com API.
func deleteDNSSECAPI(ctx context.Context, client *namecom.NameCom, domainName, digest string) error {
	err := RespectRateLimits(ctx)
	if err != nil {
		return errors.Wrap(err, "rate limiting error")
	}

	_, err = client.DeleteDNSSEC(&namecom.DeleteDNSSECRequest{
		DomainName: domainName,
		Digest:     digest,
	})
	if err != nil {
		return errors.Wrap(err, "Error DeleteDNSSEC")
	}

	return nil
}
