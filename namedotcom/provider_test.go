//nolint:paralleltest // Provider tests touch the global rate limiter state via BuildClient
package namedotcom_test

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/namedotcom/go/v4/namecom"

	namedotcom "github.com/lexfrei/terraform-provider-namedotcom/namedotcom"
)

func TestProviderSchema(t *testing.T) {
	prov := namedotcom.New("test")()

	var resp provider.SchemaResponse

	prov.Schema(context.Background(), provider.SchemaRequest{}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("provider schema returned diagnostics: %v", resp.Diagnostics)
	}

	for _, field := range []string{"username", "token", "rate_limit_per_second", "rate_limit_per_hour", "timeout"} {
		if _, ok := resp.Schema.Attributes[field]; !ok {
			t.Errorf("provider schema missing attribute %q", field)
		}
	}

	// Credentials are optional (they can be supplied via environment variables).
	if resp.Schema.Attributes["username"].IsRequired() {
		t.Error("username should be optional so it can fall back to NAMEDOTCOM_USERNAME")
	}

	if !resp.Schema.Attributes["token"].IsSensitive() {
		t.Error("token should be marked sensitive")
	}
}

func TestProviderMetadata(t *testing.T) {
	prov := namedotcom.New("1.2.3")()

	var resp provider.MetadataResponse

	prov.Metadata(context.Background(), provider.MetadataRequest{}, &resp)

	if resp.TypeName != "namedotcom" {
		t.Errorf("TypeName = %q, want %q", resp.TypeName, "namedotcom")
	}

	if resp.Version != "1.2.3" {
		t.Errorf("Version = %q, want %q", resp.Version, "1.2.3")
	}
}

func TestProviderResources(t *testing.T) {
	prov := namedotcom.New("test")()

	resources := prov.Resources(context.Background())
	if len(resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(resources))
	}
}

func TestBuildClient_Defaults(t *testing.T) {
	client := namedotcom.BuildClient("u", "t", types.Int64Null(), types.Int64Null(), types.Int64Null())

	if client == nil {
		t.Fatal("BuildClient returned nil")
	}

	if client.Client.Timeout != 120*time.Second {
		t.Errorf("default timeout = %v, want %v", client.Client.Timeout, 120*time.Second)
	}
}

func TestBuildClient_Custom(t *testing.T) {
	client := namedotcom.BuildClient("u", "t", types.Int64Value(10), types.Int64Value(1000), types.Int64Value(60))

	if client == nil {
		t.Fatal("BuildClient returned nil")
	}

	if client.Client.Timeout != 60*time.Second {
		t.Errorf("custom timeout = %v, want %v", client.Client.Timeout, 60*time.Second)
	}
}

func TestResolveCredentials_ConfigWins(t *testing.T) {
	t.Setenv("NAMEDOTCOM_USERNAME", "env-user")
	t.Setenv("NAMEDOTCOM_TOKEN", "env-token")

	username, token, missing := namedotcom.ResolveCredentials(types.StringValue("cfg-user"), types.StringValue("cfg-token"))

	if username != "cfg-user" || token != "cfg-token" {
		t.Errorf("config values should win, got %q/%q", username, token)
	}

	if len(missing) != 0 {
		t.Errorf("expected no missing credentials, got %v", missing)
	}
}

func TestResolveCredentials_EnvFallback(t *testing.T) {
	t.Setenv("NAMEDOTCOM_USERNAME", "env-user")
	t.Setenv("NAMEDOTCOM_TOKEN", "env-token")

	username, token, missing := namedotcom.ResolveCredentials(types.StringNull(), types.StringNull())

	if username != "env-user" || token != "env-token" {
		t.Errorf("expected env fallback, got %q/%q", username, token)
	}

	if len(missing) != 0 {
		t.Errorf("expected no missing credentials, got %v", missing)
	}
}

func TestResolveCredentials_Missing(t *testing.T) {
	t.Setenv("NAMEDOTCOM_USERNAME", "")
	t.Setenv("NAMEDOTCOM_TOKEN", "")

	_, _, missing := namedotcom.ResolveCredentials(types.StringNull(), types.StringNull())

	if len(missing) != 2 {
		t.Fatalf("expected both credentials reported missing, got %v", missing)
	}
}

func TestResolveCredentials_ExplicitEmptyDoesNotFallBack(t *testing.T) {
	t.Setenv("NAMEDOTCOM_USERNAME", "env-user")

	// An explicit empty string in config disables the env fallback and is
	// reported as missing, matching SDKv2's EnvDefaultFunc behaviour.
	_, _, missing := namedotcom.ResolveCredentials(types.StringValue(""), types.StringValue("tok"))

	if len(missing) != 1 || missing[0] != "username" {
		t.Errorf("expected only username reported missing, got %v", missing)
	}
}

func TestConfigureClient_Valid(t *testing.T) {
	var diags diag.Diagnostics

	client, ok := namedotcom.ConfigureClient(&namecom.NameCom{}, &diags)

	if !ok || client == nil {
		t.Fatal("expected a valid client")
	}

	if diags.HasError() {
		t.Errorf("unexpected diagnostics: %v", diags)
	}
}

func TestConfigureClient_WrongType(t *testing.T) {
	var diags diag.Diagnostics

	client, ok := namedotcom.ConfigureClient("not a client", &diags)

	if ok || client != nil {
		t.Error("expected failure for the wrong provider data type")
	}

	if !diags.HasError() {
		t.Error("expected an error diagnostic for the wrong provider data type")
	}
}

func TestConfigureClient_Nil(t *testing.T) {
	var diags diag.Diagnostics

	client, ok := namedotcom.ConfigureClient(nil, &diags)

	if ok || client != nil {
		t.Error("expected nil provider data to yield no client")
	}

	// nil ProviderData is the normal pre-Configure state and must not raise an error.
	if diags.HasError() {
		t.Errorf("nil provider data should not raise diagnostics, got: %v", diags)
	}
}
