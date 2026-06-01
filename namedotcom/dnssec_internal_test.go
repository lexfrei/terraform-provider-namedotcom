//nolint:paralleltest // The Read test exercises the global rate limiter.
package namedotcom

import (
	"context"
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

func fullDNSSECModel() dnssecModel {
	return dnssecModel{
		ID:         types.StringValue("example.com"),
		DomainName: types.StringValue("example.com"),
		KeyTag:     types.Int32Value(12345),
		Algorithm:  types.Int32Value(8),
		DigestType: types.Int32Value(2),
		Digest:     types.StringValue("AABBCCDD"),
	}
}

// TestDNSSECUpdate_RoundTrips covers the mandatory-but-unreachable Update
// method: every attribute forces replacement, so Update only round-trips the
// plan into state and must not call the API.
func TestDNSSECUpdate_RoundTrips(t *testing.T) {
	t.Parallel()

	res := &dnssecResource{}

	req := resource.UpdateRequest{Plan: dnssecPlan(t, fullDNSSECModel())}
	resp := resource.UpdateResponse{State: tfsdk.State{Schema: dnssecSchema(t)}}

	res.Update(context.Background(), req, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	var got dnssecModel

	resp.State.Get(context.Background(), &got)

	if got.KeyTag.ValueInt32() != 12345 {
		t.Errorf("key_tag = %d, want 12345", got.KeyTag.ValueInt32())
	}
}

// TestDNSSECDelete_Succeeds drives the framework Delete method end to end.
func TestDNSSECDelete_Succeeds(t *testing.T) {
	InitRateLimiters(defaultPerSecondLimit, defaultPerHourLimit)

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com/dnssec/AABBCCDD", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{}`)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	res := &dnssecResource{client: namecom.Mock("u", "t", server.URL)}

	req := resource.DeleteRequest{State: dnssecState(t, fullDNSSECModel())}

	res.Delete(context.Background(), req, &resource.DeleteResponse{})
}

// TestDNSSECRead_RemovesResourceOnNotFound drives the framework Read method and
// asserts that a 404 removes the resource from state instead of erroring,
// matching the behaviour of the record and nameservers resources.
func TestDNSSECRead_RemovesResourceOnNotFound(t *testing.T) {
	InitRateLimiters(defaultPerSecondLimit, defaultPerHourLimit)

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com/dnssec/AABBCCDD", func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, `{"message":"DNSSEC not found"}`, http.StatusNotFound)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	res := &dnssecResource{client: namecom.Mock("u", "t", server.URL)}

	req := resource.ReadRequest{
		State: dnssecState(t, dnssecModel{
			ID:         types.StringValue("example.com"),
			DomainName: types.StringValue("example.com"),
			KeyTag:     types.Int32Value(12345),
			Algorithm:  types.Int32Value(8),
			DigestType: types.Int32Value(2),
			Digest:     types.StringValue("AABBCCDD"),
		}),
	}
	resp := resource.ReadResponse{State: dnssecState(t, dnssecModel{ID: types.StringValue("example.com")})}

	res.Read(context.Background(), req, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	if !resp.State.Raw.IsNull() {
		t.Error("expected the resource to be removed from state after a 404")
	}
}

// TestDNSSECCreate_SetsState drives the framework Create method end to end.
func TestDNSSECCreate_SetsState(t *testing.T) {
	InitRateLimiters(defaultPerSecondLimit, defaultPerHourLimit)

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com/dnssec", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{"domainName":"example.com","keyTag":12345,"algorithm":8,"digestType":2,"digest":"AABBCCDD"}`)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	res := &dnssecResource{client: namecom.Mock("u", "t", server.URL)}

	req := resource.CreateRequest{Plan: dnssecPlan(t, fullDNSSECModel())}
	resp := resource.CreateResponse{State: tfsdk.State{Schema: dnssecSchema(t)}}

	res.Create(context.Background(), req, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	var got dnssecModel

	resp.State.Get(context.Background(), &got)

	if got.ID.ValueString() != "example.com" {
		t.Errorf("id = %q, want %q", got.ID.ValueString(), "example.com")
	}

	if got.KeyTag.ValueInt32() != 12345 {
		t.Errorf("key_tag = %d, want 12345", got.KeyTag.ValueInt32())
	}
}

// TestDNSSECImportState parses "DomainName_Digest" and seeds the lookup fields.
func TestDNSSECImportState(t *testing.T) {
	t.Parallel()

	res := &dnssecResource{}
	resp := resource.ImportStateResponse{State: dnssecState(t, dnssecModel{})}

	res.ImportState(context.Background(), resource.ImportStateRequest{ID: "example.com_AABBCCDD"}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	var got dnssecModel

	resp.State.Get(context.Background(), &got)

	if got.DomainName.ValueString() != "example.com" {
		t.Errorf("domain_name = %q, want %q", got.DomainName.ValueString(), "example.com")
	}

	if got.Digest.ValueString() != "AABBCCDD" {
		t.Errorf("digest = %q, want %q", got.Digest.ValueString(), "AABBCCDD")
	}
}

// TestDNSSECImportState_InvalidFormat rejects an ID without an underscore.
func TestDNSSECImportState_InvalidFormat(t *testing.T) {
	t.Parallel()

	res := &dnssecResource{}
	resp := resource.ImportStateResponse{State: dnssecState(t, dnssecModel{})}

	res.ImportState(context.Background(), resource.ImportStateRequest{ID: "no-underscore"}, &resp)

	if !resp.Diagnostics.HasError() {
		t.Error("expected an error for an import ID without an underscore")
	}
}

// TestDNSSECRead_PreservesConfiguredKeys confirms Read keeps the immutable
// lookup keys (here, the digest's configured case) rather than adopting the
// API's canonical form, which would force a spurious replacement.
//
//nolint:paralleltest // exercises the global rate limiter
func TestDNSSECRead_PreservesConfiguredKeys(t *testing.T) {
	InitRateLimiters(defaultPerSecondLimit, defaultPerHourLimit)

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com/dnssec/aabbccdd", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		// The API echoes the digest upper-cased and reports the numeric fields.
		fmt.Fprint(writer, `{"domainName":"example.com","keyTag":12345,"algorithm":8,"digestType":2,"digest":"AABBCCDD"}`)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	res := &dnssecResource{client: namecom.Mock("u", "t", server.URL)}

	model := dnssecModel{
		ID:         types.StringValue("example.com"),
		DomainName: types.StringValue("example.com"),
		KeyTag:     types.Int32Value(12345),
		Algorithm:  types.Int32Value(8),
		DigestType: types.Int32Value(2),
		Digest:     types.StringValue("aabbccdd"),
	}

	resp := resource.ReadResponse{State: dnssecState(t, model)}

	res.Read(context.Background(), resource.ReadRequest{State: dnssecState(t, model)}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	var got dnssecModel

	resp.State.Get(context.Background(), &got)

	if got.Digest.ValueString() != "aabbccdd" {
		t.Errorf("digest = %q, want the configured %q preserved (no case drift)", got.Digest.ValueString(), "aabbccdd")
	}
}

func dnssecSchema(t *testing.T) rschema.Schema {
	t.Helper()

	var schemaResp resource.SchemaResponse

	(&dnssecResource{}).Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)

	return schemaResp.Schema
}

// dnssecState builds a tfsdk.State carrying the DNSSEC schema and the given model.
func dnssecState(t *testing.T, model dnssecModel) tfsdk.State {
	t.Helper()

	state := tfsdk.State{Schema: dnssecSchema(t)}

	diags := state.Set(context.Background(), &model)
	if diags.HasError() {
		t.Fatalf("building DNSSEC state: %v", diags)
	}

	return state
}

// dnssecPlan builds a tfsdk.Plan carrying the DNSSEC schema and the given model.
func dnssecPlan(t *testing.T, model dnssecModel) tfsdk.Plan {
	t.Helper()

	plan := tfsdk.Plan{Schema: dnssecSchema(t)}

	diags := plan.Set(context.Background(), &model)
	if diags.HasError() {
		t.Fatalf("building DNSSEC plan: %v", diags)
	}

	return plan
}
