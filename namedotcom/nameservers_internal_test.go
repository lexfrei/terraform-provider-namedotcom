package namedotcom

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/namedotcom/go/v4/namecom"
)

// TestUpgradeNameserversV0ToV1 drives the actual v0 (list) -> v1 (set) state
// conversion, proving that state written by the SDKv2 provider before the
// list->set change upgrades to a correct set with the other attributes
// preserved. The framework does not coerce list to set automatically, so this
// conversion is the only behavioural migration logic in the provider.
func TestUpgradeNameserversV0ToV1(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	res := &domainNameServersResource{}

	upgrader, ok := res.UpgradeState(ctx)[0]
	if !ok {
		t.Fatal("no v0 upgrader registered")
	}

	priorState := nameserversV0State(t, *upgrader.PriorSchema)

	var schemaResp resource.SchemaResponse

	res.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	resp := resource.UpgradeStateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}

	upgradeNameserversV0ToV1(ctx, resource.UpgradeStateRequest{State: &priorState}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("upgrade returned diagnostics: %v", resp.Diagnostics)
	}

	var upgraded nameserversModel

	resp.Diagnostics.Append(resp.State.Get(ctx, &upgraded)...)

	if resp.Diagnostics.HasError() {
		t.Fatalf("reading upgraded state: %v", resp.Diagnostics)
	}

	if upgraded.DomainName.ValueString() != "example.com" {
		t.Errorf("domain_name = %q, want %q", upgraded.DomainName.ValueString(), "example.com")
	}

	if upgraded.ID.ValueString() != "example.com" {
		t.Errorf("id = %q, want %q", upgraded.ID.ValueString(), "example.com")
	}

	var nameservers []string

	upgraded.Nameservers.ElementsAs(ctx, &nameservers, false)

	if len(nameservers) != 2 {
		t.Fatalf("expected 2 nameservers after upgrade, got %d", len(nameservers))
	}
}

// TestDomainNameServersCreate_SetsState drives the framework Create method end
// to end: it confirms that the configured nameservers are sent and that the
// computed set is populated in state from the API read-back.
//
//nolint:paralleltest // exercises the global rate limiter
func TestDomainNameServersCreate_SetsState(t *testing.T) {
	InitRateLimiters(defaultPerSecondLimit, defaultPerHourLimit)

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com:setNameservers", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{}`)
	})
	mux.HandleFunc("/v4/domains/example.com", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{"domainName":"example.com","nameservers":["ns1.example.com","ns2.example.com"]}`)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	res := &domainNameServersResource{client: namecom.Mock("u", "t", server.URL)}

	nameservers, _ := types.SetValueFrom(context.Background(), types.StringType, []string{"ns1.example.com", "ns2.example.com"})

	req := resource.CreateRequest{
		Plan: nameserversV1Plan(t, nameserversModel{
			ID:          types.StringNull(),
			DomainName:  types.StringValue("example.com"),
			Nameservers: nameservers,
		}),
	}
	resp := resource.CreateResponse{State: tfsdk.State{Schema: nameserversV1Schema(t)}}

	res.Create(context.Background(), req, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	var got nameserversModel

	resp.State.Get(context.Background(), &got)

	if got.ID.ValueString() != "example.com" {
		t.Errorf("id = %q, want %q", got.ID.ValueString(), "example.com")
	}

	var nsList []string

	got.Nameservers.ElementsAs(context.Background(), &nsList, false)

	if len(nsList) != 2 {
		t.Fatalf("expected 2 nameservers in state, got %d", len(nsList))
	}
}

// nameserversV1Plan builds a tfsdk.Plan carrying the current nameservers schema.
func nameserversV1Plan(t *testing.T, model nameserversModel) tfsdk.Plan {
	t.Helper()

	plan := tfsdk.Plan{Schema: nameserversV1Schema(t)}

	diags := plan.Set(context.Background(), &model)
	if diags.HasError() {
		t.Fatalf("building nameservers plan: %v", diags)
	}

	return plan
}

func nameserversV1Schema(t *testing.T) rschema.Schema {
	t.Helper()

	var schemaResp resource.SchemaResponse

	(&domainNameServersResource{}).Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)

	return schemaResp.Schema
}

// nameserversV1State builds a tfsdk.State carrying the current nameservers schema.
func nameserversV1State(t *testing.T, model nameserversModel) tfsdk.State {
	t.Helper()

	state := tfsdk.State{Schema: nameserversV1Schema(t)}

	diags := state.Set(context.Background(), &model)
	if diags.HasError() {
		t.Fatalf("building nameservers state: %v", diags)
	}

	return state
}

// nameserversMock builds a mock client whose :setNameservers and GetDomain
// endpoints behave according to the provided status codes and domain payload.
func nameserversMock(t *testing.T, setStatus, getStatus int, getBody string) *namecom.NameCom {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com:setNameservers", func(writer http.ResponseWriter, _ *http.Request) {
		if setStatus != http.StatusOK {
			http.Error(writer, `{"message":"not found"}`, setStatus)

			return
		}

		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{}`)
	})
	mux.HandleFunc("/v4/domains/example.com", func(writer http.ResponseWriter, _ *http.Request) {
		if getStatus != http.StatusOK {
			http.Error(writer, `{"message":"not found"}`, getStatus)

			return
		}

		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, getBody)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return namecom.Mock("u", "t", server.URL)
}

const nameserversDomainBody = `{"domainName":"example.com","nameservers":["ns1.example.com","ns2.example.com"]}`

//nolint:paralleltest // exercises the global rate limiter
func TestDomainNameServersRead_Success(t *testing.T) {
	InitRateLimiters(defaultPerSecondLimit, defaultPerHourLimit)

	res := &domainNameServersResource{client: nameserversMock(t, http.StatusOK, http.StatusOK, nameserversDomainBody)}

	req := resource.ReadRequest{State: nameserversV1State(t, nameserversModel{
		ID:          types.StringValue("example.com"),
		DomainName:  types.StringValue("example.com"),
		Nameservers: types.SetNull(types.StringType),
	})}
	resp := resource.ReadResponse{State: tfsdk.State{Schema: nameserversV1Schema(t)}}

	res.Read(context.Background(), req, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	var got nameserversModel

	resp.State.Get(context.Background(), &got)

	var nsList []string

	got.Nameservers.ElementsAs(context.Background(), &nsList, false)

	if len(nsList) != 2 {
		t.Fatalf("expected 2 nameservers, got %d", len(nsList))
	}
}

//nolint:paralleltest // exercises the global rate limiter
func TestDomainNameServersRead_RemovesResourceOnNotFound(t *testing.T) {
	InitRateLimiters(defaultPerSecondLimit, defaultPerHourLimit)

	res := &domainNameServersResource{client: nameserversMock(t, http.StatusOK, http.StatusNotFound, "")}

	req := resource.ReadRequest{State: nameserversV1State(t, nameserversModel{
		ID:          types.StringValue("example.com"),
		DomainName:  types.StringValue("example.com"),
		Nameservers: types.SetNull(types.StringType),
	})}
	resp := resource.ReadResponse{State: nameserversV1State(t, nameserversModel{
		ID:          types.StringValue("example.com"),
		DomainName:  types.StringValue("example.com"),
		Nameservers: types.SetNull(types.StringType),
	})}

	res.Read(context.Background(), req, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	if !resp.State.Raw.IsNull() {
		t.Error("expected the resource to be removed from state after a 404")
	}
}

// TestDomainNameServersUpdate_RemovesResourceOnNotFound covers the Update
// not-found branch unique to this resource: a 404 from SetNameservers drops the
// resource from state instead of erroring.
//
//nolint:paralleltest // exercises the global rate limiter
func TestDomainNameServersUpdate_RemovesResourceOnNotFound(t *testing.T) {
	InitRateLimiters(defaultPerSecondLimit, defaultPerHourLimit)

	res := &domainNameServersResource{client: nameserversMock(t, http.StatusNotFound, http.StatusOK, "")}

	nameservers, _ := types.SetValueFrom(context.Background(), types.StringType, []string{"ns1.example.com"})

	req := resource.UpdateRequest{Plan: nameserversV1Plan(t, nameserversModel{
		ID:          types.StringValue("example.com"),
		DomainName:  types.StringValue("example.com"),
		Nameservers: nameservers,
	})}
	resp := resource.UpdateResponse{State: tfsdk.State{Schema: nameserversV1Schema(t)}}

	res.Update(context.Background(), req, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	if !resp.State.Raw.IsNull() {
		t.Error("expected the resource to be removed from state after a 404 during update")
	}
}

// TestDomainNameServersDelete_ResetsNameservers asserts Delete sends an empty
// nameserver list (resetting the domain to account defaults).
//
//nolint:paralleltest // exercises the global rate limiter
func TestDomainNameServersDelete_ResetsNameservers(t *testing.T) {
	InitRateLimiters(defaultPerSecondLimit, defaultPerHourLimit)

	var sentNameservers []string

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com:setNameservers", func(writer http.ResponseWriter, request *http.Request) {
		var payload namecom.SetNameserversRequest

		_ = json.NewDecoder(request.Body).Decode(&payload)
		sentNameservers = payload.Nameservers

		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{}`)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	res := &domainNameServersResource{client: namecom.Mock("u", "t", server.URL)}

	nameservers, _ := types.SetValueFrom(context.Background(), types.StringType, []string{"ns1.example.com"})

	req := resource.DeleteRequest{State: nameserversV1State(t, nameserversModel{
		ID:          types.StringValue("example.com"),
		DomainName:  types.StringValue("example.com"),
		Nameservers: nameservers,
	})}

	res.Delete(context.Background(), req, &resource.DeleteResponse{})

	if len(sentNameservers) != 0 {
		t.Errorf("expected an empty nameserver list on delete, got %v", sentNameservers)
	}
}

// nameserversV0State builds a schema-version-0 (list-shaped) prior state.
func nameserversV0State(t *testing.T, priorSchema rschema.Schema) tfsdk.State {
	t.Helper()

	ctx := context.Background()

	nsList, diags := types.ListValueFrom(ctx, types.StringType, []string{"ns1.example.com", "ns2.example.com"})
	if diags.HasError() {
		t.Fatalf("building prior list value: %v", diags)
	}

	state := tfsdk.State{Schema: priorSchema}

	diags = state.Set(ctx, &nameserversModelV0{
		ID:          types.StringValue("example.com"),
		DomainName:  types.StringValue("example.com"),
		Nameservers: nsList,
	})
	if diags.HasError() {
		t.Fatalf("building prior state: %v", diags)
	}

	return state
}
