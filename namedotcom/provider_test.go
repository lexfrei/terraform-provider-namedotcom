package namedotcom_test

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/lexfrei/terraform-provider-namedotcom/namedotcom"
)

func TestProviderSchema(t *testing.T) {
	t.Parallel()

	provider := namedotcom.Provider()

	if provider == nil {
		t.Fatal("Provider() returned nil")
	}

	// Test provider schema
	if provider.Schema == nil {
		t.Fatal("Provider schema is nil")
	}

	// Test required fields exist in schema
	requiredFields := []string{"username", "token"}
	for _, field := range requiredFields {
		if _, exists := provider.Schema[field]; !exists {
			t.Errorf("Required field '%s' not found in provider schema", field)
		}

		if !provider.Schema[field].Required {
			t.Errorf("Field '%s' should be required", field)
		}
	}

	// Test optional fields exist in schema
	optionalFields := []string{"rate_limit_per_second", "rate_limit_per_hour", "timeout"}
	for _, field := range optionalFields {
		if _, exists := provider.Schema[field]; !exists {
			t.Errorf("Optional field '%s' not found in provider schema", field)
		}

		if provider.Schema[field].Required {
			t.Errorf("Field '%s' should be optional", field)
		}
	}
}

func TestProviderDefaults(t *testing.T) {
	t.Parallel()

	provider := namedotcom.Provider()

	// Test default values for optional fields
	if provider.Schema["rate_limit_per_second"].Default != 20 {
		t.Errorf("rate_limit_per_second default should be %d, got %v",
			20, provider.Schema["rate_limit_per_second"].Default)
	}

	if provider.Schema["rate_limit_per_hour"].Default != 3000 {
		t.Errorf("rate_limit_per_hour default should be %d, got %v",
			3000, provider.Schema["rate_limit_per_hour"].Default)
	}

	if provider.Schema["timeout"].Default != 120 {
		t.Errorf("timeout default should be %d, got %v",
			120, provider.Schema["timeout"].Default)
	}
}

func TestProviderResources(t *testing.T) {
	t.Parallel()

	provider := namedotcom.Provider()

	// Test resources map
	expectedResources := []string{
		"namedotcom_record",
		"namedotcom_domain_nameservers",
		"namedotcom_dnssec",
	}

	for _, resourceName := range expectedResources {
		if _, exists := provider.ResourcesMap[resourceName]; !exists {
			t.Errorf("Expected resource '%s' not found in ResourcesMap", resourceName)
		}
	}

	// Test that ConfigureContextFunc is set
	if provider.ConfigureContextFunc == nil {
		t.Error("Provider ConfigureContextFunc is nil")
	}
}

func TestProviderConfigure_Success(t *testing.T) {
	t.Parallel()

	// Create test data
	resourceData := schema.TestResourceDataRaw(t, namedotcom.Provider().Schema, map[string]interface{}{
		"username": "testuser",
		"token":    "testtoken",
	})

	// Test provider configuration
	client, diags := namedotcom.ProviderConfigure(context.TODO(), resourceData)
	if diags.HasError() {
		t.Fatalf("ProviderConfigure failed: %v", diags)
	}

	if client == nil {
		t.Fatal("ProviderConfigure returned nil client")
	}
}

func TestProviderConfigure_MissingToken(t *testing.T) {
	t.Parallel()

	// Test with missing token
	resourceData := schema.TestResourceDataRaw(t, namedotcom.Provider().Schema, map[string]interface{}{
		"username": "testuser",
	})

	_, diags := namedotcom.ProviderConfigure(context.TODO(), resourceData)
	if !diags.HasError() {
		t.Error("Expected error when token is missing")
	}
}

func TestProviderConfigure_MissingUsername(t *testing.T) {
	t.Parallel()

	// Test with missing username
	resourceData := schema.TestResourceDataRaw(t, namedotcom.Provider().Schema, map[string]interface{}{
		"token": "testtoken",
	})

	_, diags := namedotcom.ProviderConfigure(context.TODO(), resourceData)
	if !diags.HasError() {
		t.Error("Expected error when username is missing")
	}
}

func TestProviderConfigure_EmptyCredentials(t *testing.T) {
	t.Parallel()

	// Test with empty credentials
	resourceData := schema.TestResourceDataRaw(t, namedotcom.Provider().Schema, map[string]interface{}{
		"username": "",
		"token":    "",
	})

	_, diags := namedotcom.ProviderConfigure(context.TODO(), resourceData)
	if !diags.HasError() {
		t.Error("Expected error when credentials are empty")
	}
}

func TestProviderConfigure_CustomRateLimits(t *testing.T) {
	t.Parallel()

	// Test with custom rate limits
	resourceData := schema.TestResourceDataRaw(t, namedotcom.Provider().Schema, map[string]interface{}{
		"username":              "testuser",
		"token":                 "testtoken",
		"rate_limit_per_second": 10,
		"rate_limit_per_hour":   1000,
		"timeout":               60,
	})

	client, diags := namedotcom.ProviderConfigure(context.TODO(), resourceData)
	if diags.HasError() {
		t.Fatalf("ProviderConfigure failed: %v", diags)
	}

	if client == nil {
		t.Fatal("ProviderConfigure returned nil client")
	}
}

func TestProviderConfigure_DefaultValues(t *testing.T) {
	t.Parallel()

	// Test that default values are used when not specified
	resourceData := schema.TestResourceDataRaw(t, namedotcom.Provider().Schema, map[string]interface{}{
		"username": "testuser",
		"token":    "testtoken",
	})

	client, diags := namedotcom.ProviderConfigure(context.TODO(), resourceData)
	if diags.HasError() {
		t.Fatalf("ProviderConfigure failed: %v", diags)
	}

	if client == nil {
		t.Fatal("ProviderConfigure returned nil client")
	}

	// Verify default timeout is applied by checking the configured value
	timeoutValue := resourceData.Get("timeout")
	if timeoutValue != 120 {
		t.Errorf("Expected default timeout %d, got %v", 120, timeoutValue)
	}
}