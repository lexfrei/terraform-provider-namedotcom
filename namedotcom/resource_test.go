package namedotcom_test

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"

	namedotcom "github.com/lexfrei/terraform-provider-namedotcom/namedotcom"
)

// forcesReplace reports whether an attribute documents that changing it forces
// replacement. Every attribute carrying a RequiresReplace plan modifier must
// say so in its description, so the registry docs stay accurate.
func forcesReplace(attr rschema.Attribute) bool {
	return strings.Contains(strings.ToLower(attr.GetDescription()), "forces a new resource")
}

// assertStringForcesReplace checks that each named string attribute carries a
// RequiresReplace plan modifier and documents it in its description.
func assertStringForcesReplace(t *testing.T, attrs map[string]rschema.Attribute, names ...string) {
	t.Helper()

	for _, name := range names {
		if !stringRequiresReplace(attrs[name]) {
			t.Errorf("%s should force replacement", name)
		}

		if !forcesReplace(attrs[name]) {
			t.Errorf("%s description should document that it forces a new resource", name)
		}
	}
}

// resourceSchema invokes a resource's Schema method and returns the result.
func resourceSchema(t *testing.T, res resource.Resource) rschema.Schema {
	t.Helper()

	var resp resource.SchemaResponse

	res.Schema(context.Background(), resource.SchemaRequest{}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("resource schema returned diagnostics: %v", resp.Diagnostics)
	}

	return resp.Schema
}

// stringRequiresReplace reports whether a string attribute carries a plan
// modifier (the RequiresReplace behaviour ported from SDKv2 ForceNew).
func stringRequiresReplace(attr rschema.Attribute) bool {
	stringAttr, ok := attr.(rschema.StringAttribute)

	return ok && len(stringAttr.PlanModifiers) > 0
}

func int32RequiresReplace(attr rschema.Attribute) bool {
	int32Attr, ok := attr.(rschema.Int32Attribute)

	return ok && len(int32Attr.PlanModifiers) > 0
}

func TestRecordResource_Schema(t *testing.T) {
	t.Parallel()

	res := namedotcom.NewRecordResource()
	attrs := resourceSchema(t, res).Attributes

	for _, field := range []string{"id", "record_id", "domain_name", "host", "record_type", "answer"} {
		if _, ok := attrs[field]; !ok {
			t.Errorf("record schema missing attribute %q", field)
		}
	}

	if !attrs["id"].IsComputed() {
		t.Error("id should be computed")
	}

	if !attrs["record_id"].IsComputed() {
		t.Error("record_id should be computed (populated from the API, not user-set)")
	}

	// record_id moved from Optional (SDKv2) to Computed-only in v4; pin that it
	// is no longer settable so the documented breaking change can't regress.
	if attrs["record_id"].IsOptional() {
		t.Error("record_id must not be settable")
	}

	if !attrs["domain_name"].IsOptional() {
		t.Error("domain_name should be optional")
	}

	// A record cannot move zones or change type in place, so both force replacement.
	assertStringForcesReplace(t, attrs, "domain_name", "record_type")

	if _, ok := res.(resource.ResourceWithImportState); !ok {
		t.Error("record resource should support import")
	}
}

func TestDNSSECResource_Schema(t *testing.T) {
	t.Parallel()

	res := namedotcom.NewDNSSECResource()
	attrs := resourceSchema(t, res).Attributes

	for _, field := range []string{"id", "domain_name", "key_tag", "algorithm", "digest_type", "digest"} {
		if _, ok := attrs[field]; !ok {
			t.Errorf("dnssec schema missing attribute %q", field)
		}
	}

	if !attrs["domain_name"].IsRequired() {
		t.Error("domain_name should be required")
	}

	if !attrs["key_tag"].IsRequired() {
		t.Error("key_tag should be required")
	}

	if !stringRequiresReplace(attrs["domain_name"]) {
		t.Error("domain_name should force replacement")
	}

	if !int32RequiresReplace(attrs["key_tag"]) {
		t.Error("key_tag should force replacement")
	}

	// Every DNSSEC attribute forces replacement, so each must document it.
	for _, name := range []string{"domain_name", "key_tag", "algorithm", "digest_type", "digest"} {
		if !forcesReplace(attrs[name]) {
			t.Errorf("%s description should document that it forces a new resource", name)
		}
	}

	if _, ok := res.(resource.ResourceWithImportState); !ok {
		t.Error("dnssec resource should support import")
	}
}

func TestDomainNameServersResource_Schema(t *testing.T) {
	t.Parallel()

	res := namedotcom.NewDomainNameServersResource()
	schema := resourceSchema(t, res)
	attrs := schema.Attributes

	for _, field := range []string{"id", "domain_name", "nameservers"} {
		if _, ok := attrs[field]; !ok {
			t.Errorf("nameservers schema missing attribute %q", field)
		}
	}

	if !attrs["domain_name"].IsRequired() {
		t.Error("domain_name should be required")
	}

	assertStringForcesReplace(t, attrs, "domain_name")

	if !attrs["nameservers"].IsOptional() || !attrs["nameservers"].IsComputed() {
		t.Error("nameservers should be optional and computed")
	}
}

func TestDomainNameServersResource_StateUpgrade(t *testing.T) {
	t.Parallel()

	res := namedotcom.NewDomainNameServersResource()

	// State written by the SDKv2 provider was at schema version 1; the framework
	// resource must match it so existing state is read without an upgrade.
	if resourceSchema(t, res).Version != 1 {
		t.Errorf("nameservers schema version = %d, want 1", resourceSchema(t, res).Version)
	}

	upgrader, ok := res.(resource.ResourceWithUpgradeState)
	if !ok {
		t.Fatal("nameservers resource should support state upgrades")
	}

	if _, exists := upgrader.UpgradeState(context.Background())[0]; !exists {
		t.Error("nameservers resource should register a v0 -> v1 state upgrader")
	}
}

func TestProviderResourcesInstantiate(t *testing.T) {
	t.Parallel()

	prov := namedotcom.New("test")()
	for index, factory := range prov.Resources(context.Background()) {
		if factory() == nil {
			t.Errorf("resource factory %d returned nil", index)
		}
	}
}
