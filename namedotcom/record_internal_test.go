//nolint:paralleltest // The Read test exercises the global rate limiter.
package namedotcom

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/namedotcom/go/v4/namecom"
)

// TestRecordCreateState verifies that Create/Update persist the server-assigned
// identifiers from the API response while echoing the user-controlled
// attributes from the plan. Echoing (rather than reading back the API's
// canonical form) is required: the framework does not enable the SDKv2 legacy
// type-system leniency, so a non-computed attribute that changed during apply
// would fail Terraform's "inconsistent result after apply" check.
func TestRecordCreateState(t *testing.T) {
	plan := recordModel{
		DomainName: types.StringValue("example.com"),
		Host:       types.StringValue("www"),
		RecordType: types.StringValue("cname"),
		Answer:     types.StringValue("bar.com"),
	}

	record := &namecom.Record{
		ID:         42,
		DomainName: "example.com",
		Host:       "www",
		Type:       "CNAME",
		Answer:     "bar.com.",
	}

	got := recordCreateState(plan, record)

	if got.ID.ValueString() != "42" {
		t.Errorf("id = %q, want %q", got.ID.ValueString(), "42")
	}

	if got.RecordID.ValueInt32() != 42 {
		t.Errorf("record_id = %d, want 42", got.RecordID.ValueInt32())
	}

	if got.RecordType.ValueString() != "cname" {
		t.Errorf("record_type = %q, want the configured %q", got.RecordType.ValueString(), "cname")
	}

	if got.Answer.ValueString() != "bar.com" {
		t.Errorf("answer = %q, want the configured %q", got.Answer.ValueString(), "bar.com")
	}
}

// TestRecordReadState_PreservesConfiguredRepresentation is the regression guard
// for the perpetual-diff bug: when the API returns a value that is only
// canonically different from what is in state (upper-cased type, trailing-dot
// answer), Read must keep the stored representation so no drift is reported.
func TestRecordReadState_PreservesConfiguredRepresentation(t *testing.T) {
	state := recordModel{
		ID:         types.StringValue("42"),
		DomainName: types.StringValue("example.com"),
		Host:       types.StringValue("www"),
		RecordType: types.StringValue("cname"),
		Answer:     types.StringValue("bar.com"),
	}

	record := &namecom.Record{
		ID:         42,
		DomainName: "example.com",
		Host:       "www",
		Type:       "CNAME",
		Answer:     "bar.com.",
	}

	got := recordReadState(state, record)

	if got.RecordID.ValueInt32() != 42 {
		t.Errorf("record_id = %d, want 42", got.RecordID.ValueInt32())
	}

	if got.RecordType.ValueString() != "cname" {
		t.Errorf("record_type = %q, want the configured %q (no canonicalization drift)", got.RecordType.ValueString(), "cname")
	}

	if got.Answer.ValueString() != "bar.com" {
		t.Errorf("answer = %q, want the configured %q (no trailing-dot drift)", got.Answer.ValueString(), "bar.com")
	}
}

// TestRecordReadState_AdoptsAPIValueWhenStateEmpty covers import, where the
// user attributes are null and must be populated from the API.
func TestRecordReadState_AdoptsAPIValueWhenStateEmpty(t *testing.T) {
	state := recordModel{
		ID:         types.StringValue("42"),
		DomainName: types.StringValue("example.com"),
	}

	record := &namecom.Record{
		ID:         42,
		DomainName: "example.com",
		Host:       "www",
		Type:       "CNAME",
		Answer:     "bar.com.",
	}

	got := recordReadState(state, record)

	if got.Host.ValueString() != "www" {
		t.Errorf("host = %q, want %q", got.Host.ValueString(), "www")
	}

	if got.RecordType.ValueString() != "CNAME" {
		t.Errorf("record_type = %q, want %q", got.RecordType.ValueString(), "CNAME")
	}

	if got.Answer.ValueString() != "bar.com." {
		t.Errorf("answer = %q, want %q", got.Answer.ValueString(), "bar.com.")
	}
}

// TestRecordReadState_UpgradesZeroRecordID pins the documented upgrade path:
// the SDKv2 provider never wrote record_id to state, so existing state carries
// record_id = 0; the first Read must overwrite it with the real API id rather
// than leaving the stale zero.
func TestRecordReadState_UpgradesZeroRecordID(t *testing.T) {
	t.Parallel()

	state := recordModel{
		ID:         types.StringValue("42"),
		RecordID:   types.Int32Value(0),
		DomainName: types.StringValue("example.com"),
		Host:       types.StringValue("www"),
		RecordType: types.StringValue("A"),
		Answer:     types.StringValue("192.0.2.1"),
	}

	record := &namecom.Record{ID: 42, DomainName: "example.com", Host: "www", Type: "A", Answer: "192.0.2.1"}

	got := recordReadState(state, record)

	if got.RecordID.ValueInt32() != 42 {
		t.Errorf("record_id = %d, want the API id 42 (stale 0 must be corrected)", got.RecordID.ValueInt32())
	}
}

// TestRecordReadState_PreservesApexHost confirms a configured apex "@" survives
// a Read where the API reports the equivalent empty host.
func TestRecordReadState_PreservesApexHost(t *testing.T) {
	t.Parallel()

	state := recordModel{
		ID:         types.StringValue("42"),
		DomainName: types.StringValue("example.com"),
		Host:       types.StringValue("@"),
		RecordType: types.StringValue("A"),
		Answer:     types.StringValue("192.0.2.1"),
	}

	record := &namecom.Record{ID: 42, DomainName: "example.com", Host: "", Type: "A", Answer: "192.0.2.1"}

	got := recordReadState(state, record)

	if got.Host.ValueString() != "@" {
		t.Errorf("host = %q, want the configured apex %q preserved", got.Host.ValueString(), "@")
	}
}

// TestRecordReadState_AdoptsApexHostOnImport confirms that on import (null host)
// the empty apex host from the API is adopted rather than left null.
func TestRecordReadState_AdoptsApexHostOnImport(t *testing.T) {
	t.Parallel()

	state := recordModel{
		ID:         types.StringValue("42"),
		DomainName: types.StringValue("example.com"),
	}

	record := &namecom.Record{ID: 42, DomainName: "example.com", Host: "", Type: "A", Answer: "192.0.2.1"}

	got := recordReadState(state, record)

	if got.Host.IsNull() || got.Host.ValueString() != "" {
		t.Errorf("host should be the empty apex from the API, got null=%v value=%q", got.Host.IsNull(), got.Host.ValueString())
	}
}

// TestRecordReadState_DetectsRealDrift confirms that a genuinely different API
// value is still adopted (real out-of-band changes are not hidden).
func TestRecordReadState_DetectsRealDrift(t *testing.T) {
	state := recordModel{
		ID:         types.StringValue("42"),
		DomainName: types.StringValue("example.com"),
		Host:       types.StringValue("www"),
		RecordType: types.StringValue("A"),
		Answer:     types.StringValue("1.2.3.4"),
	}

	record := &namecom.Record{
		ID:         42,
		DomainName: "example.com",
		Host:       "www",
		Type:       "A",
		Answer:     "5.6.7.8",
	}

	got := recordReadState(state, record)

	if got.Answer.ValueString() != "5.6.7.8" {
		t.Errorf("answer = %q, want the drifted API value %q", got.Answer.ValueString(), "5.6.7.8")
	}
}

// TestRecordRead_RemovesResourceOnNotFound drives the framework Read method and
// asserts that a 404 from the API removes the resource from state (so the next
// plan recreates it) rather than returning an error.
func TestRecordRead_RemovesResourceOnNotFound(t *testing.T) {
	InitRateLimiters(defaultPerSecondLimit, defaultPerHourLimit)

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com/records/42", func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, `{"message":"Record not found"}`, http.StatusNotFound)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	res := &recordResource{client: namecom.Mock("u", "t", server.URL)}

	req := resource.ReadRequest{
		State: recordState(t, recordModel{
			ID:         types.StringValue("42"),
			DomainName: types.StringValue("example.com"),
			Host:       types.StringValue("www"),
			RecordType: types.StringValue("A"),
			Answer:     types.StringValue("1.2.3.4"),
		}),
	}
	resp := resource.ReadResponse{State: recordState(t, recordModel{ID: types.StringValue("42")})}

	res.Read(context.Background(), req, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	if !resp.State.Raw.IsNull() {
		t.Error("expected the resource to be removed from state after a 404")
	}
}

// TestRecordCreate_SetsState drives the framework Create method end to end:
// it confirms the plan/helper/state wiring persists the server-assigned id and
// record_id while echoing the configured attributes.
func TestRecordCreate_SetsState(t *testing.T) {
	InitRateLimiters(defaultPerSecondLimit, defaultPerHourLimit)

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com/records", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{"id":42,"domainName":"example.com","host":"www","type":"A","answer":"192.0.2.1"}`)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	res := &recordResource{client: namecom.Mock("u", "t", server.URL)}

	req := resource.CreateRequest{
		Plan: recordPlan(t, recordModel{
			ID:         types.StringNull(),
			RecordID:   types.Int32Null(),
			DomainName: types.StringValue("example.com"),
			Host:       types.StringValue("www"),
			RecordType: types.StringValue("A"),
			Answer:     types.StringValue("192.0.2.1"),
		}),
	}
	resp := resource.CreateResponse{State: recordState(t, recordModel{ID: types.StringNull()})}

	res.Create(context.Background(), req, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	var got recordModel

	resp.State.Get(context.Background(), &got)

	if got.ID.ValueString() != "42" {
		t.Errorf("id = %q, want %q", got.ID.ValueString(), "42")
	}

	if got.RecordID.ValueInt32() != 42 {
		t.Errorf("record_id = %d, want 42", got.RecordID.ValueInt32())
	}

	if got.Answer.ValueString() != "192.0.2.1" {
		t.Errorf("answer = %q, want %q", got.Answer.ValueString(), "192.0.2.1")
	}
}

// TestRecordUpdate_SetsState drives the framework Update method end to end and
// confirms the new configured values are echoed into state.
func TestRecordUpdate_SetsState(t *testing.T) {
	InitRateLimiters(defaultPerSecondLimit, defaultPerHourLimit)

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com/records/42", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{"id":42,"domainName":"example.com","host":"www","type":"A","answer":"192.0.2.9"}`)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	res := &recordResource{client: namecom.Mock("u", "t", server.URL)}

	model := func(answer string) recordModel {
		return recordModel{
			ID:         types.StringValue("42"),
			RecordID:   types.Int32Value(42),
			DomainName: types.StringValue("example.com"),
			Host:       types.StringValue("www"),
			RecordType: types.StringValue("A"),
			Answer:     types.StringValue(answer),
		}
	}

	req := resource.UpdateRequest{Plan: recordPlan(t, model("192.0.2.9")), State: recordState(t, model("192.0.2.1"))}
	resp := resource.UpdateResponse{State: recordState(t, model("192.0.2.1"))}

	res.Update(context.Background(), req, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	var got recordModel

	resp.State.Get(context.Background(), &got)

	if got.Answer.ValueString() != "192.0.2.9" {
		t.Errorf("answer = %q, want the updated %q", got.Answer.ValueString(), "192.0.2.9")
	}
}

// TestRecordUpdate_RemovesResourceOnNotFound covers the Update not-found path:
// a 404 from the API drops the record from state instead of erroring, matching
// the Read path and the nameservers resource.
func TestRecordUpdate_RemovesResourceOnNotFound(t *testing.T) {
	InitRateLimiters(defaultPerSecondLimit, defaultPerHourLimit)

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com/records/42", func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, `{"message":"Record not found"}`, http.StatusNotFound)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	res := &recordResource{client: namecom.Mock("u", "t", server.URL)}

	model := recordModel{
		ID:         types.StringValue("42"),
		RecordID:   types.Int32Value(42),
		DomainName: types.StringValue("example.com"),
		Host:       types.StringValue("www"),
		RecordType: types.StringValue("A"),
		Answer:     types.StringValue("192.0.2.9"),
	}

	req := resource.UpdateRequest{Plan: recordPlan(t, model), State: recordState(t, model)}
	resp := resource.UpdateResponse{State: recordState(t, model)}

	res.Update(context.Background(), req, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	if !resp.State.Raw.IsNull() {
		t.Error("expected the resource to be removed from state after a 404 during update")
	}
}

// TestRecordDelete_Succeeds drives the framework Delete method end to end.
func TestRecordDelete_Succeeds(t *testing.T) {
	InitRateLimiters(defaultPerSecondLimit, defaultPerHourLimit)

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com/records/42", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{}`)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	res := &recordResource{client: namecom.Mock("u", "t", server.URL)}

	req := resource.DeleteRequest{State: recordState(t, recordModel{
		ID:         types.StringValue("42"),
		DomainName: types.StringValue("example.com"),
	})}

	res.Delete(context.Background(), req, &resource.DeleteResponse{})
}

// TestRecordImportState parses "domain:id" and seeds domain_name and id.
func TestRecordImportState(t *testing.T) {
	res := &recordResource{}
	resp := resource.ImportStateResponse{State: recordState(t, recordModel{})}

	res.ImportState(context.Background(), resource.ImportStateRequest{ID: "example.com:42"}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	var got recordModel

	resp.State.Get(context.Background(), &got)

	if got.DomainName.ValueString() != "example.com" {
		t.Errorf("domain_name = %q, want %q", got.DomainName.ValueString(), "example.com")
	}

	if got.ID.ValueString() != "42" {
		t.Errorf("id = %q, want %q", got.ID.ValueString(), "42")
	}
}

// TestRecordImportState_InvalidFormat rejects an ID without a colon.
func TestRecordImportState_InvalidFormat(t *testing.T) {
	res := &recordResource{}
	resp := resource.ImportStateResponse{State: recordState(t, recordModel{})}

	res.ImportState(context.Background(), resource.ImportStateRequest{ID: "no-colon"}, &resp)

	if !resp.Diagnostics.HasError() {
		t.Error("expected an error for an import ID without a colon")
	}
}

// TestRecordRead_InvalidIDErrors covers the parseRecordID failure branch: a
// non-numeric id in state surfaces an error rather than panicking or silently
// proceeding.
func TestRecordRead_InvalidIDErrors(t *testing.T) {
	res := &recordResource{client: &namecom.NameCom{}}

	state := recordModel{
		ID:         types.StringValue("not-a-number"),
		DomainName: types.StringValue("example.com"),
	}

	resp := resource.ReadResponse{State: recordState(t, state)}

	res.Read(context.Background(), resource.ReadRequest{State: recordState(t, state)}, &resp)

	if !resp.Diagnostics.HasError() {
		t.Error("expected an error for a non-numeric record ID in state")
	}
}

// TestRequiresReplaceOnDNSChange pins the semantic-replace behaviour: a change
// that is only DNS canonicalization (case, trailing dot) must NOT force
// replacement — this is what keeps an upgrade from SDKv2 state (which stored
// the API's canonical form) from destroying and recreating resources — while a
// genuine value change still does.
func TestRequiresReplaceOnDNSChange(t *testing.T) {
	mod := requiresReplaceOnDNSChange()

	base := recordModel{ID: types.StringValue("42"), DomainName: types.StringValue("example.com")}

	cases := []struct {
		name        string
		state, plan string
		wantReplace bool
	}{
		{"case-only difference is not a replace", "CNAME", "cname", false},
		{"trailing-dot difference is not a replace", "bar.com.", "bar.com", false},
		{"a genuine change forces replacement", "cname", "mx", true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			req := planmodifier.StringRequest{
				State:      recordState(t, base),
				Plan:       recordPlan(t, base),
				StateValue: types.StringValue(testCase.state),
				PlanValue:  types.StringValue(testCase.plan),
			}
			resp := &planmodifier.StringResponse{}

			mod.PlanModifyString(context.Background(), req, resp)

			if resp.RequiresReplace != testCase.wantReplace {
				t.Errorf("RequiresReplace = %v, want %v", resp.RequiresReplace, testCase.wantReplace)
			}
		})
	}
}

// recordPlan builds a tfsdk.Plan carrying the record schema and the given model.
func recordPlan(t *testing.T, model recordModel) tfsdk.Plan {
	t.Helper()

	var schemaResp resource.SchemaResponse

	(&recordResource{}).Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)

	plan := tfsdk.Plan{Schema: schemaResp.Schema}

	diags := plan.Set(context.Background(), &model)
	if diags.HasError() {
		t.Fatalf("building record plan: %v", diags)
	}

	return plan
}

// recordState builds a tfsdk.State carrying the record schema and the given model.
func recordState(t *testing.T, model recordModel) tfsdk.State {
	t.Helper()

	var schemaResp resource.SchemaResponse

	(&recordResource{}).Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)

	state := tfsdk.State{Schema: schemaResp.Schema}

	diags := state.Set(context.Background(), &model)
	if diags.HasError() {
		t.Fatalf("building record state: %v", diags)
	}

	return state
}
