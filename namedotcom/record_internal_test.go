//nolint:paralleltest // The Read test exercises the global rate limiter.
package namedotcom

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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

// TestReconcilePriority pins the drift-avoidance rules for the priority
// attribute: an unset priority must not flap against the API's zero default,
// a configured value matching the API is preserved, and a genuine difference
// (including populating an MX record's priority on import) is adopted.
func TestReconcilePriority(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		prior    types.Int32
		apiValue uint32
		want     types.Int32
	}{
		{"unset stays null when API reports the zero default", types.Int32Null(), 0, types.Int32Null()},
		{"configured value matching the API is preserved", types.Int32Value(10), 10, types.Int32Value(10)},
		{"unset adopts a non-zero API value (MX import)", types.Int32Null(), 10, types.Int32Value(10)},
		{"a genuine difference is adopted from the API", types.Int32Value(10), 20, types.Int32Value(20)},
		{"an explicit zero is preserved", types.Int32Value(0), 0, types.Int32Value(0)},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := reconcilePriority(testCase.prior, testCase.apiValue)

			if !got.Equal(testCase.want) {
				t.Errorf("reconcilePriority(%v, %d) = %v, want %v", testCase.prior, testCase.apiValue, got, testCase.want)
			}
		})
	}
}

// TestRecordCreate_MXSetsPriority drives Create end to end for an MX record and
// confirms the configured priority is sent to the API and echoed into state.
func TestRecordCreate_MXSetsPriority(t *testing.T) {
	InitRateLimiters(defaultPerSecondLimit, defaultPerHourLimit)

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com/records", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{"id":42,"domainName":"example.com","host":"","type":"MX","answer":"mail.example.com","priority":10}`)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	res := &recordResource{client: namecom.Mock("u", "t", server.URL)}

	req := resource.CreateRequest{
		Plan: recordPlan(t, recordModel{
			ID:         types.StringNull(),
			RecordID:   types.Int32Null(),
			DomainName: types.StringValue("example.com"),
			Host:       types.StringValue(""),
			RecordType: types.StringValue("MX"),
			Answer:     types.StringValue("mail.example.com"),
			Priority:   types.Int32Value(10),
		}),
	}
	resp := resource.CreateResponse{State: recordState(t, recordModel{ID: types.StringNull()})}

	res.Create(context.Background(), req, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	var got recordModel

	resp.State.Get(context.Background(), &got)

	if got.Priority.ValueInt32() != 10 {
		t.Errorf("priority = %d, want 10", got.Priority.ValueInt32())
	}
}

// TestRecordValidateConfig_PriorityRecordType pins that priority is accepted
// only on MX and SRV records and rejected on every other type, so a config that
// would silently drift (the API drops priority for other types) fails fast.
func TestRecordValidateConfig_PriorityRecordType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		recordType string
		priority   types.Int32
		wantErr    bool
	}{
		{"priority on MX is allowed", "MX", types.Int32Value(10), false},
		{"priority on SRV is allowed", "SRV", types.Int32Value(10), false},
		{"priority on lowercase mx is allowed", "mx", types.Int32Value(10), false},
		{"priority on A is rejected", "A", types.Int32Value(10), true},
		{"priority on CNAME is rejected", "CNAME", types.Int32Value(10), true},
		{"no priority on A is allowed", "A", types.Int32Null(), false},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			res := &recordResource{}
			req := resource.ValidateConfigRequest{
				Config: recordConfig(t, recordModel{
					DomainName: types.StringValue("example.com"),
					Host:       types.StringValue(""),
					RecordType: types.StringValue(testCase.recordType),
					Answer:     types.StringValue("mail.example.com"),
					Priority:   testCase.priority,
				}),
			}
			resp := &resource.ValidateConfigResponse{}

			res.ValidateConfig(context.Background(), req, resp)

			if resp.Diagnostics.HasError() != testCase.wantErr {
				t.Errorf("ValidateConfig error = %v, want %v (diags: %v)", resp.Diagnostics.HasError(), testCase.wantErr, resp.Diagnostics)
			}
		})
	}
}

// TestRecordSchema_PriorityRangeEnforced confirms the documented 0-65535 range
// is actually enforced by a validator on the attribute, rejecting out-of-range
// values and accepting the boundaries.
func TestRecordSchema_PriorityRangeEnforced(t *testing.T) {
	t.Parallel()

	var schemaResp resource.SchemaResponse

	(&recordResource{}).Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)

	attr, ok := schemaResp.Schema.Attributes[keyPriority].(schema.Int32Attribute)
	if !ok {
		t.Fatalf("priority attribute is %T, want schema.Int32Attribute", schemaResp.Schema.Attributes[keyPriority])
	}

	if len(attr.Int32Validators()) == 0 {
		t.Fatal("priority attribute has no validators; the documented 0-65535 range is not enforced")
	}

	rejected := func(value int32) bool {
		req := validator.Int32Request{Path: path.Root(keyPriority), ConfigValue: types.Int32Value(value)}
		resp := &validator.Int32Response{}

		for _, val := range attr.Int32Validators() {
			val.ValidateInt32(context.Background(), req, resp)
		}

		return resp.Diagnostics.HasError()
	}

	cases := []struct {
		value   int32
		wantErr bool
	}{
		{-1, true},
		{65536, true},
		{0, false},
		{65535, false},
	}

	for _, testCase := range cases {
		if got := rejected(testCase.value); got != testCase.wantErr {
			t.Errorf("priority=%d: validator rejected = %v, want %v", testCase.value, got, testCase.wantErr)
		}
	}
}

// TestPriorityToUint32 covers the unset/unknown branches that map to the API's
// zero default; concrete values pass through unchanged.
func TestPriorityToUint32(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input types.Int32
		want  uint32
	}{
		{"null maps to zero", types.Int32Null(), 0},
		{"unknown maps to zero", types.Int32Unknown(), 0},
		{"concrete value passes through", types.Int32Value(10), 10},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := priorityToUint32(testCase.input); got != testCase.want {
				t.Errorf("priorityToUint32(%v) = %d, want %d", testCase.input, got, testCase.want)
			}
		})
	}
}

// TestRecordUpdate_MXPriorityReset pins the priority 10 -> 0 path. Because the
// API field is `omitempty`, a zero priority is dropped from the request body;
// UpdateRecord is a full-replace PUT, so the server resets priority to 0 and
// state converges. A switch to PATCH semantics would break this and the test.
func TestRecordUpdate_MXPriorityReset(t *testing.T) {
	InitRateLimiters(defaultPerSecondLimit, defaultPerHourLimit)

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com/records/42", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprint(writer, `{"id":42,"domainName":"example.com","host":"","type":"MX","answer":"mail.example.com","priority":0}`)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	res := &recordResource{client: namecom.Mock("u", "t", server.URL)}

	model := func(priority types.Int32) recordModel {
		return recordModel{
			ID:         types.StringValue("42"),
			RecordID:   types.Int32Value(42),
			DomainName: types.StringValue("example.com"),
			Host:       types.StringValue(""),
			RecordType: types.StringValue("MX"),
			Answer:     types.StringValue("mail.example.com"),
			Priority:   priority,
		}
	}

	req := resource.UpdateRequest{
		Plan:  recordPlan(t, model(types.Int32Value(0))),
		State: recordState(t, model(types.Int32Value(10))),
	}
	resp := resource.UpdateResponse{State: recordState(t, model(types.Int32Value(10)))}

	res.Update(context.Background(), req, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	var got recordModel

	resp.State.Get(context.Background(), &got)

	if got.Priority.ValueInt32() != 0 {
		t.Errorf("priority = %d, want 0 (reset converges)", got.Priority.ValueInt32())
	}
}

// TestPriorityToInt32 covers the API-to-schema conversion directly.
func TestPriorityToInt32(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input uint32
		want  int32
	}{
		{0, 0},
		{10, 10},
		{65535, 65535},
	}

	for _, testCase := range cases {
		if got := priorityToInt32(testCase.input); got != testCase.want {
			t.Errorf("priorityToInt32(%d) = %d, want %d", testCase.input, got, testCase.want)
		}
	}
}

// TestRecordValidateConfig_DefersOnUnknown confirms the two defer branches: when
// either record_type or priority is unknown (computed from another resource),
// validation is postponed rather than erroring.
func TestRecordValidateConfig_DefersOnUnknown(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		recordType types.String
		priority   types.Int32
	}{
		{"unknown record_type defers", types.StringUnknown(), types.Int32Value(10)},
		{"null record_type defers", types.StringNull(), types.Int32Value(10)},
		{"unknown priority defers", types.StringValue("A"), types.Int32Unknown()},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			res := &recordResource{}
			req := resource.ValidateConfigRequest{
				Config: recordConfig(t, recordModel{
					DomainName: types.StringValue("example.com"),
					Host:       types.StringValue(""),
					RecordType: testCase.recordType,
					Answer:     types.StringValue("mail.example.com"),
					Priority:   testCase.priority,
				}),
			}
			resp := &resource.ValidateConfigResponse{}

			res.ValidateConfig(context.Background(), req, resp)

			if resp.Diagnostics.HasError() {
				t.Errorf("expected validation to defer (no error), got %v", resp.Diagnostics)
			}
		})
	}
}

// recordConfig builds a tfsdk.Config carrying the record schema and the given
// model. tfsdk.Config has no Set, so the model is marshalled via State.Set and
// the resulting Raw value is wrapped in a Config (they share a representation).
func recordConfig(t *testing.T, model recordModel) tfsdk.Config {
	t.Helper()

	return tfsdk.Config(recordState(t, model))
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
