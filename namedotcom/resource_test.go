package namedotcom_test

import (
	"testing"

	"github.com/lexfrei/terraform-provider-namedotcom/namedotcom"
)

func TestResourceRecord_Schema(t *testing.T) {
	t.Parallel()

	provider := namedotcom.Provider()
	resource := provider.ResourcesMap["namedotcom_record"]

	if resource == nil {
		t.Fatal("namedotcom_record resource not found")
	}

	// Test that required CRUD operations are defined
	if resource.Create == nil {
		t.Error("Create function is nil")
	}
	if resource.Read == nil {
		t.Error("Read function is nil")
	}
	if resource.Update == nil {
		t.Error("Update function is nil")
	}
	if resource.Delete == nil {
		t.Error("Delete function is nil")
	}

	// Test that importer is defined
	if resource.Importer == nil {
		t.Error("Importer is nil")
	}

	// Test schema fields
	expectedFields := []string{
		"record_id",
		"domain_name",
		"host",
		"record_type",
		"answer",
	}

	for _, field := range expectedFields {
		if _, exists := resource.Schema[field]; !exists {
			t.Errorf("Expected field '%s' not found in schema", field)
		}
	}

	// Test that all fields are optional (as defined in the resource)
	for fieldName, fieldSchema := range resource.Schema {
		if fieldSchema.Required {
			t.Errorf("Field '%s' should be optional, not required", fieldName)
		}
	}
}

func TestResourceDomainNameServers_Schema(t *testing.T) {
	t.Parallel()

	provider := namedotcom.Provider()
	resource := provider.ResourcesMap["namedotcom_domain_nameservers"]

	if resource == nil {
		t.Fatal("namedotcom_domain_nameservers resource not found")
	}

	// Test that required CRUD operations are defined
	if resource.Create == nil {
		t.Error("Create function is nil")
	}
	if resource.Read == nil {
		t.Error("Read function is nil")
	}
	if resource.Update == nil {
		t.Error("Update function is nil")
	}
	if resource.Delete == nil {
		t.Error("Delete function is nil")
	}

	// Check that schema exists
	if resource.Schema == nil {
		t.Fatal("Resource schema is nil")
	}
}

func TestResourceDNSSEC_Schema(t *testing.T) {
	t.Parallel()

	provider := namedotcom.Provider()
	resource := provider.ResourcesMap["namedotcom_dnssec"]

	if resource == nil {
		t.Fatal("namedotcom_dnssec resource not found")
	}

	// Test that required CRUD operations are defined
	if resource.Create == nil {
		t.Error("Create function is nil")
	}
	if resource.Read == nil {
		t.Error("Read function is nil")
	}
	if resource.Delete == nil {
		t.Error("Delete function is nil")
	}

	// Test that importer is defined
	if resource.Importer == nil {
		t.Error("Importer is nil")
	}

	// Check that schema exists
	if resource.Schema == nil {
		t.Fatal("Resource schema is nil")
	}
}

func TestResourceRecord_SchemaConsistency(t *testing.T) {
	t.Parallel()

	provider := namedotcom.Provider()
	resource := provider.ResourcesMap["namedotcom_record"]

	// Test that all schema fields have descriptions
	for fieldName, fieldSchema := range resource.Schema {
		if fieldSchema.Description == "" {
			t.Errorf("Field '%s' is missing description", fieldName)
		}
	}
}

func TestResourceDomainNameServers_SchemaConsistency(t *testing.T) {
	t.Parallel()

	provider := namedotcom.Provider()
	resource := provider.ResourcesMap["namedotcom_domain_nameservers"]

	// Test that schema is not empty
	if len(resource.Schema) == 0 {
		t.Error("Resource schema should not be empty")
	}
}

func TestResourceDNSSEC_SchemaConsistency(t *testing.T) {
	t.Parallel()

	provider := namedotcom.Provider()
	resource := provider.ResourcesMap["namedotcom_dnssec"]

	// Test that schema is not empty
	if len(resource.Schema) == 0 {
		t.Error("Resource schema should not be empty")
	}
}

func TestProvider_SchemaConsistency(t *testing.T) {
	t.Parallel()

	provider := namedotcom.Provider()

	// Test that provider schema has required fields
	requiredFields := []string{"username", "token"}
	for _, field := range requiredFields {
		if _, exists := provider.Schema[field]; !exists {
			t.Errorf("Provider missing required field: %s", field)
		}
	}

	// Test all resources have schemas
	for resourceName, resource := range provider.ResourcesMap {
		if resource.Schema == nil {
			t.Errorf("Resource '%s' has nil schema", resourceName)
		}
		if len(resource.Schema) == 0 {
			t.Errorf("Resource '%s' has empty schema", resourceName)
		}
	}
}