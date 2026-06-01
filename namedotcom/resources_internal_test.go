package namedotcom

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/namedotcom/go/v4/namecom"
)

func TestResourceMetadata(t *testing.T) {
	t.Parallel()

	cases := []struct {
		res  resource.Resource
		want string
	}{
		{&recordResource{}, "namedotcom_record"},
		{&dnssecResource{}, "namedotcom_dnssec"},
		{&domainNameServersResource{}, "namedotcom_domain_nameservers"},
	}

	for _, testCase := range cases {
		var resp resource.MetadataResponse

		testCase.res.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "namedotcom"}, &resp)

		if resp.TypeName != testCase.want {
			t.Errorf("TypeName = %q, want %q", resp.TypeName, testCase.want)
		}
	}
}

func TestResourceConfigure(t *testing.T) {
	t.Parallel()

	client := &namecom.NameCom{}

	for _, res := range []resource.ResourceWithConfigure{
		&recordResource{}, &dnssecResource{}, &domainNameServersResource{},
	} {
		var resp resource.ConfigureResponse

		res.Configure(context.Background(), resource.ConfigureRequest{ProviderData: client}, &resp)

		if resp.Diagnostics.HasError() {
			t.Errorf("unexpected diagnostics: %v", resp.Diagnostics)
		}
	}
}

func TestProviderDataSources(t *testing.T) {
	t.Parallel()

	if New("test")().DataSources(context.Background()) != nil {
		t.Error("expected no data sources")
	}
}

func TestProviderConfigure_EnvFallback(t *testing.T) {
	t.Setenv("NAMEDOTCOM_USERNAME", "env-user")
	t.Setenv("NAMEDOTCOM_TOKEN", "env-token")

	var resp provider.ConfigureResponse

	New("test")().Configure(context.Background(), provider.ConfigureRequest{Config: nullProviderConfig(t)}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	if resp.ResourceData == nil {
		t.Error("expected a configured client in ResourceData")
	}
}

func TestProviderConfigure_MissingCredentials(t *testing.T) {
	t.Setenv("NAMEDOTCOM_USERNAME", "")
	t.Setenv("NAMEDOTCOM_TOKEN", "")

	var resp provider.ConfigureResponse

	New("test")().Configure(context.Background(), provider.ConfigureRequest{Config: nullProviderConfig(t)}, &resp)

	if !resp.Diagnostics.HasError() {
		t.Error("expected missing-credential diagnostics when neither config nor env supplies them")
	}
}

func TestProviderConfigure_UnknownCredentialErrors(t *testing.T) {
	t.Parallel()

	// A credential that is unknown until apply must surface a clear error, not
	// be misreported as missing.
	cfg := providerConfig(t, providerModel{
		Username:           types.StringValue("known-user"),
		Token:              types.StringUnknown(),
		RateLimitPerSecond: types.Int64Null(),
		RateLimitPerHour:   types.Int64Null(),
		Timeout:            types.Int64Null(),
	})

	var resp provider.ConfigureResponse

	New("test")().Configure(context.Background(), provider.ConfigureRequest{Config: cfg}, &resp)

	if !resp.Diagnostics.HasError() {
		t.Error("expected an error when a credential is unknown at plan time")
	}
}

// nullProviderConfig builds a provider Config whose attributes are all null, so
// Configure falls back to the environment.
func nullProviderConfig(t *testing.T) tfsdk.Config {
	t.Helper()

	return providerConfig(t, providerModel{
		Username:           types.StringNull(),
		Token:              types.StringNull(),
		RateLimitPerSecond: types.Int64Null(),
		RateLimitPerHour:   types.Int64Null(),
		Timeout:            types.Int64Null(),
	})
}

// providerConfig encodes a providerModel into a provider Config. It reuses
// tfsdk.State.Set (Config has no Set) to build the raw value.
func providerConfig(t *testing.T, model providerModel) tfsdk.Config {
	t.Helper()

	var schemaResp provider.SchemaResponse

	New("test")().Schema(context.Background(), provider.SchemaRequest{}, &schemaResp)

	state := tfsdk.State{Schema: schemaResp.Schema}

	diags := state.Set(context.Background(), &model)
	if diags.HasError() {
		t.Fatalf("building provider config: %v", diags)
	}

	return tfsdk.Config{Schema: schemaResp.Schema, Raw: state.Raw}
}
