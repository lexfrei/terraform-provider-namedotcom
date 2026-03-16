//nolint:paralleltest // CRUD tests modify global rate limiter state
package namedotcom_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/namedotcom/go/v4/namecom"

	namedotcom "github.com/lexfrei/terraform-provider-namedotcom/namedotcom"
)

const testDomain = "example.com"

func newMockClient(t *testing.T, handler http.Handler) *namecom.NameCom {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return namecom.Mock("testuser", "testtoken", server.URL)
}

func mustJSON(t *testing.T, val any) []byte {
	t.Helper()

	data, err := json.Marshal(val)
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}

	return data
}

func initLimiters(t *testing.T) {
	t.Helper()

	namedotcom.InitRateLimiters(namedotcom.DefaultPerSecondLimit, namedotcom.DefaultPerHourLimit)

	t.Cleanup(func() {
		namedotcom.InitRateLimiters(namedotcom.DefaultPerSecondLimit, namedotcom.DefaultPerHourLimit)
	})
}

// Record CRUD tests.

func TestResourceRecordCreate_Success(t *testing.T) {
	initLimiters(t)

	mux := http.NewServeMux()

	mux.HandleFunc("/v4/domains/example.com/records", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)

			return
		}

		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(writer, `{"id":42,"domainName":"example.com","host":"test","type":"A","answer":"1.2.3.4"}`)
	})

	mux.HandleFunc("/v4/domains/example.com/records/42", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(writer, `{"id":42,"domainName":"example.com","host":"test","type":"A","answer":"1.2.3.4"}`)
	})

	client := newMockClient(t, mux)

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceRecord().Schema, map[string]any{
		"domain_name": testDomain,
		"host":        "test",
		"record_type": "A",
		"answer":      "1.2.3.4",
	})

	err := namedotcom.ResourceRecordCreate(data, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data.Id() != "42" {
		t.Errorf("ID = %q, want %q", data.Id(), "42")
	}
}

func TestResourceRecordRead_Success(t *testing.T) {
	initLimiters(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com/records/42", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(writer, `{"id":42,"domainName":"example.com","host":"www","type":"CNAME","answer":"example.com."}`)
	})

	client := newMockClient(t, mux)

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceRecord().Schema, map[string]any{
		"domain_name": testDomain,
		"host":        "",
		"record_type": "",
		"answer":      "",
	})
	data.SetId("42")

	err := namedotcom.ResourceRecordRead(data, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertResourceData(t, data, "domain_name", testDomain)
	assertResourceData(t, data, "host", "www")
	assertResourceData(t, data, "record_type", "CNAME")
	assertResourceData(t, data, "answer", "example.com.")
}

func TestResourceRecordUpdate_Success(t *testing.T) {
	initLimiters(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com/records/42", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(writer, `{"id":42,"domainName":"example.com","host":"test","type":"A","answer":"5.6.7.8"}`)
	})

	client := newMockClient(t, mux)

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceRecord().Schema, map[string]any{
		"domain_name": testDomain,
		"host":        "test",
		"record_type": "A",
		"answer":      "5.6.7.8",
	})
	data.SetId("42")

	err := namedotcom.ResourceRecordUpdate(data, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertResourceData(t, data, "answer", "5.6.7.8")
}

func TestResourceRecordDelete_Success(t *testing.T) {
	initLimiters(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com/records/42", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodDelete {
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)

			return
		}

		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(writer, `{}`)
	})

	client := newMockClient(t, mux)

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceRecord().Schema, map[string]any{
		"domain_name": testDomain,
		"host":        "test",
		"record_type": "A",
		"answer":      "1.2.3.4",
	})
	data.SetId("42")

	err := namedotcom.ResourceRecordDelete(data, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data.Id() != "" {
		t.Errorf("ID should be empty after delete, got %q", data.Id())
	}
}

func TestResourceRecordCreate_APIError(t *testing.T) {
	initLimiters(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com/records", func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, `{"message":"internal server error"}`, http.StatusInternalServerError)
	})

	client := newMockClient(t, mux)

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceRecord().Schema, map[string]any{
		"domain_name": testDomain,
		"host":        "test",
		"record_type": "A",
		"answer":      "1.2.3.4",
	})

	err := namedotcom.ResourceRecordCreate(data, client)
	if err == nil {
		t.Fatal("expected error from API, got nil")
	}
}

func TestResourceRecordCreate_InvalidMeta(t *testing.T) {
	t.Parallel()

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceRecord().Schema, map[string]any{
		"domain_name": testDomain,
		"host":        "test",
		"record_type": "A",
		"answer":      "1.2.3.4",
	})

	err := namedotcom.ResourceRecordCreate(data, "not a client")
	if err == nil {
		t.Fatal("expected error for invalid meta, got nil")
	}
}

// DNSSEC CRUD tests.

func testDNSSECResponse() *namecom.DNSSEC {
	return &namecom.DNSSEC{
		DomainName: testDomain,
		KeyTag:     12345,
		Algorithm:  8,
		DigestType: 2,
		Digest:     "AABBCCDD",
	}
}

func TestResourceDNSSECCreate_Success(t *testing.T) {
	initLimiters(t)

	mux := http.NewServeMux()

	mux.HandleFunc("/v4/domains/example.com/dnssec", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)

			return
		}

		writer.Header().Set("Content-Type", "application/json")

		_, writeErr := writer.Write(mustJSON(t, testDNSSECResponse()))
		if writeErr != nil {
			t.Errorf("failed to write response: %v", writeErr)
		}
	})

	mux.HandleFunc("/v4/domains/example.com/dnssec/AABBCCDD", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")

		_, writeErr := writer.Write(mustJSON(t, testDNSSECResponse()))
		if writeErr != nil {
			t.Errorf("failed to write response: %v", writeErr)
		}
	})

	client := newMockClient(t, mux)

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceDNSSEC().Schema, map[string]any{
		"domain_name": testDomain,
		"key_tag":     12345,
		"algorithm":   8,
		"digest_type": 2,
		"digest":      "AABBCCDD",
	})

	err := namedotcom.ResourceDNSSECCreate(data, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data.Id() != testDomain {
		t.Errorf("ID = %q, want %q", data.Id(), testDomain)
	}
}

func TestResourceDNSSECRead_Success(t *testing.T) {
	initLimiters(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com/dnssec/AABBCCDD", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")

		_, writeErr := writer.Write(mustJSON(t, testDNSSECResponse()))
		if writeErr != nil {
			t.Errorf("failed to write response: %v", writeErr)
		}
	})

	client := newMockClient(t, mux)

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceDNSSEC().Schema, map[string]any{
		"domain_name": testDomain,
		"key_tag":     0,
		"algorithm":   0,
		"digest_type": 0,
		"digest":      "AABBCCDD",
	})
	data.SetId(testDomain)

	err := namedotcom.ResourceDNSSECRead(data, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertResourceData(t, data, "domain_name", testDomain)
	assertResourceData(t, data, "digest", "AABBCCDD")
	assertResourceDataInt(t, data, "key_tag", 12345)
	assertResourceDataInt(t, data, "algorithm", 8)
	assertResourceDataInt(t, data, "digest_type", 2)
}

func TestResourceDNSSECDelete_Success(t *testing.T) {
	initLimiters(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com/dnssec/AABBCCDD", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodDelete {
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)

			return
		}

		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(writer, `{}`)
	})

	client := newMockClient(t, mux)

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceDNSSEC().Schema, map[string]any{
		"domain_name": testDomain,
		"key_tag":     12345,
		"algorithm":   8,
		"digest_type": 2,
		"digest":      "AABBCCDD",
	})
	data.SetId(testDomain)

	err := namedotcom.ResourceDNSSECDelete(data, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data.Id() != "" {
		t.Errorf("ID should be empty after delete, got %q", data.Id())
	}
}

// Domain Nameservers CRUD tests.

func TestResourceDomainNameServersCreate_Success(t *testing.T) {
	initLimiters(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com:setNameservers", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)

			return
		}

		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(writer, `{}`)
	})

	// GET /v4/domains/example.com → Read (called after Create)
	mux.HandleFunc("/v4/domains/example.com", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(writer, `{"domainName":"example.com","nameservers":["ns1.example.com","ns2.example.com"]}`)
	})

	client := newMockClient(t, mux)

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceDomainNameServers().Schema, map[string]any{
		"domain_name": testDomain,
		"nameservers": []any{"ns1.example.com", "ns2.example.com"},
	})

	err := namedotcom.ResourceDomainNameServersCreate(data, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data.Id() != testDomain {
		t.Errorf("ID = %q, want %q", data.Id(), testDomain)
	}

	// Verify Read was called after Create and populated nameservers from API
	nameservers, ok := data.Get("nameservers").([]any)
	if !ok {
		t.Fatal("nameservers is not []any")
	}

	if len(nameservers) != 2 {
		t.Fatalf("expected 2 nameservers after Create+Read, got %d", len(nameservers))
	}
}

func TestResourceDomainNameServersDelete_Success(t *testing.T) {
	initLimiters(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com:setNameservers", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)

			return
		}

		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(writer, `{}`)
	})

	client := newMockClient(t, mux)

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceDomainNameServers().Schema, map[string]any{
		"domain_name": testDomain,
		"nameservers": []any{},
	})
	data.SetId(testDomain)

	err := namedotcom.ResourceDomainNameServersDelete(data, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data.Id() != "" {
		t.Errorf("ID should be empty after delete, got %q", data.Id())
	}
}

func TestResourceDomainNameServersCreate_InvalidMeta(t *testing.T) {
	t.Parallel()

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceDomainNameServers().Schema, map[string]any{
		"domain_name": testDomain,
		"nameservers": []any{"ns1.example.com"},
	})

	err := namedotcom.ResourceDomainNameServersCreate(data, "not a client")
	if err == nil {
		t.Fatal("expected error for invalid meta, got nil")
	}
}

// API error tests for Read, Update, Delete operations.

func newErrorMock(t *testing.T, pattern string) *namecom.NameCom {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc(pattern, func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, `{"message":"server error"}`, http.StatusInternalServerError)
	})

	return newMockClient(t, mux)
}

func TestResourceRecordRead_APIError(t *testing.T) {
	initLimiters(t)

	client := newErrorMock(t, "/v4/domains/example.com/records/42")

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceRecord().Schema, map[string]any{
		"domain_name": testDomain,
		"host":        "",
		"record_type": "",
		"answer":      "",
	})
	data.SetId("42")

	err := namedotcom.ResourceRecordRead(data, client)
	if err == nil {
		t.Fatal("expected error from API, got nil")
	}
}

func TestResourceRecordRead_InvalidMeta(t *testing.T) {
	t.Parallel()

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceRecord().Schema, map[string]any{
		"domain_name": testDomain,
		"host":        "",
		"record_type": "",
		"answer":      "",
	})
	data.SetId("42")

	err := namedotcom.ResourceRecordRead(data, "not a client")
	if err == nil {
		t.Fatal("expected error for invalid meta, got nil")
	}
}

func TestResourceRecordUpdate_APIError(t *testing.T) {
	initLimiters(t)

	client := newErrorMock(t, "/v4/domains/example.com/records/42")

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceRecord().Schema, map[string]any{
		"domain_name": testDomain,
		"host":        "test",
		"record_type": "A",
		"answer":      "1.2.3.4",
	})
	data.SetId("42")

	err := namedotcom.ResourceRecordUpdate(data, client)
	if err == nil {
		t.Fatal("expected error from API, got nil")
	}
}

func TestResourceRecordUpdate_InvalidMeta(t *testing.T) {
	t.Parallel()

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceRecord().Schema, map[string]any{
		"domain_name": testDomain,
		"host":        "test",
		"record_type": "A",
		"answer":      "1.2.3.4",
	})
	data.SetId("42")

	err := namedotcom.ResourceRecordUpdate(data, "not a client")
	if err == nil {
		t.Fatal("expected error for invalid meta, got nil")
	}
}

func TestResourceRecordDelete_APIError(t *testing.T) {
	initLimiters(t)

	client := newErrorMock(t, "/v4/domains/example.com/records/42")

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceRecord().Schema, map[string]any{
		"domain_name": testDomain,
		"host":        "test",
		"record_type": "A",
		"answer":      "1.2.3.4",
	})
	data.SetId("42")

	err := namedotcom.ResourceRecordDelete(data, client)
	if err == nil {
		t.Fatal("expected error from API, got nil")
	}
}

func TestResourceRecordDelete_InvalidMeta(t *testing.T) {
	t.Parallel()

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceRecord().Schema, map[string]any{
		"domain_name": testDomain,
		"host":        "test",
		"record_type": "A",
		"answer":      "1.2.3.4",
	})
	data.SetId("42")

	err := namedotcom.ResourceRecordDelete(data, "not a client")
	if err == nil {
		t.Fatal("expected error for invalid meta, got nil")
	}
}

// DNSSEC API error tests.

func TestResourceDNSSECCreate_APIError(t *testing.T) {
	initLimiters(t)

	client := newErrorMock(t, "/v4/domains/example.com/dnssec")

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceDNSSEC().Schema, map[string]any{
		"domain_name": testDomain,
		"key_tag":     12345,
		"algorithm":   8,
		"digest_type": 2,
		"digest":      "AABBCCDD",
	})

	err := namedotcom.ResourceDNSSECCreate(data, client)
	if err == nil {
		t.Fatal("expected error from API, got nil")
	}
}

func TestResourceDNSSECCreate_InvalidMeta(t *testing.T) {
	t.Parallel()

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceDNSSEC().Schema, map[string]any{
		"domain_name": testDomain,
		"key_tag":     12345,
		"algorithm":   8,
		"digest_type": 2,
		"digest":      "AABBCCDD",
	})

	err := namedotcom.ResourceDNSSECCreate(data, "not a client")
	if err == nil {
		t.Fatal("expected error for invalid meta, got nil")
	}
}

func TestResourceDNSSECRead_APIError(t *testing.T) {
	initLimiters(t)

	client := newErrorMock(t, "/v4/domains/example.com/dnssec/AABBCCDD")

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceDNSSEC().Schema, map[string]any{
		"domain_name": testDomain,
		"key_tag":     0,
		"algorithm":   0,
		"digest_type": 0,
		"digest":      "AABBCCDD",
	})
	data.SetId(testDomain)

	err := namedotcom.ResourceDNSSECRead(data, client)
	if err == nil {
		t.Fatal("expected error from API, got nil")
	}
}

func TestResourceDNSSECRead_InvalidMeta(t *testing.T) {
	t.Parallel()

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceDNSSEC().Schema, map[string]any{
		"domain_name": testDomain,
		"key_tag":     0,
		"algorithm":   0,
		"digest_type": 0,
		"digest":      "AABBCCDD",
	})
	data.SetId(testDomain)

	err := namedotcom.ResourceDNSSECRead(data, "not a client")
	if err == nil {
		t.Fatal("expected error for invalid meta, got nil")
	}
}

func TestResourceDNSSECDelete_APIError(t *testing.T) {
	initLimiters(t)

	client := newErrorMock(t, "/v4/domains/example.com/dnssec/AABBCCDD")

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceDNSSEC().Schema, map[string]any{
		"domain_name": testDomain,
		"key_tag":     12345,
		"algorithm":   8,
		"digest_type": 2,
		"digest":      "AABBCCDD",
	})
	data.SetId(testDomain)

	err := namedotcom.ResourceDNSSECDelete(data, client)
	if err == nil {
		t.Fatal("expected error from API, got nil")
	}
}

func TestResourceDNSSECDelete_InvalidMeta(t *testing.T) {
	t.Parallel()

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceDNSSEC().Schema, map[string]any{
		"domain_name": testDomain,
		"key_tag":     12345,
		"algorithm":   8,
		"digest_type": 2,
		"digest":      "AABBCCDD",
	})
	data.SetId(testDomain)

	err := namedotcom.ResourceDNSSECDelete(data, "not a client")
	if err == nil {
		t.Fatal("expected error for invalid meta, got nil")
	}
}

// Nameservers API error tests.

func TestResourceDomainNameServersCreate_APIError(t *testing.T) {
	initLimiters(t)

	client := newErrorMock(t, "/v4/domains/example.com:setNameservers")

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceDomainNameServers().Schema, map[string]any{
		"domain_name": testDomain,
		"nameservers": []any{"ns1.example.com"},
	})

	err := namedotcom.ResourceDomainNameServersCreate(data, client)
	if err == nil {
		t.Fatal("expected error from API, got nil")
	}
}

func TestResourceDomainNameServersDelete_APIError(t *testing.T) {
	initLimiters(t)

	client := newErrorMock(t, "/v4/domains/example.com:setNameservers")

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceDomainNameServers().Schema, map[string]any{
		"domain_name": testDomain,
		"nameservers": []any{},
	})
	data.SetId(testDomain)

	err := namedotcom.ResourceDomainNameServersDelete(data, client)
	if err == nil {
		t.Fatal("expected error from API, got nil")
	}
}

func TestResourceDomainNameServersDelete_InvalidMeta(t *testing.T) {
	t.Parallel()

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceDomainNameServers().Schema, map[string]any{
		"domain_name": testDomain,
		"nameservers": []any{},
	})
	data.SetId(testDomain)

	err := namedotcom.ResourceDomainNameServersDelete(data, "not a client")
	if err == nil {
		t.Fatal("expected error for invalid meta, got nil")
	}
}

// Nameservers Read and Update tests.

func TestResourceDomainNameServersRead_Success(t *testing.T) {
	initLimiters(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(writer, `{"domainName":"example.com","nameservers":["ns1.example.com","ns2.example.com"]}`)
	})

	client := newMockClient(t, mux)

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceDomainNameServers().Schema, map[string]any{
		"domain_name": testDomain,
		"nameservers": []any{},
	})
	data.SetId(testDomain)

	err := namedotcom.ResourceDomainNameServersRead(data, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nameservers, ok := data.Get("nameservers").([]any)
	if !ok {
		t.Fatal("nameservers is not []any")
	}

	if len(nameservers) != 2 {
		t.Fatalf("expected 2 nameservers, got %d", len(nameservers))
	}

	if nameservers[0] != "ns1.example.com" {
		t.Errorf("nameservers[0] = %q, want %q", nameservers[0], "ns1.example.com")
	}

	if nameservers[1] != "ns2.example.com" {
		t.Errorf("nameservers[1] = %q, want %q", nameservers[1], "ns2.example.com")
	}
}

func TestResourceDomainNameServersRead_APIError(t *testing.T) {
	initLimiters(t)

	client := newErrorMock(t, "/v4/domains/example.com")

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceDomainNameServers().Schema, map[string]any{
		"domain_name": testDomain,
		"nameservers": []any{},
	})
	data.SetId(testDomain)

	err := namedotcom.ResourceDomainNameServersRead(data, client)
	if err == nil {
		t.Fatal("expected error from API, got nil")
	}
}

func TestResourceDomainNameServersRead_InvalidMeta(t *testing.T) {
	t.Parallel()

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceDomainNameServers().Schema, map[string]any{
		"domain_name": testDomain,
		"nameservers": []any{},
	})
	data.SetId(testDomain)

	err := namedotcom.ResourceDomainNameServersRead(data, "not a client")
	if err == nil {
		t.Fatal("expected error for invalid meta, got nil")
	}
}

func TestResourceDomainNameServersRead_DomainNotFound(t *testing.T) {
	initLimiters(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com", func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, `{"message":"Domain not found"}`, http.StatusNotFound)
	})

	client := newMockClient(t, mux)

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceDomainNameServers().Schema, map[string]any{
		"domain_name": testDomain,
		"nameservers": []any{},
	})
	data.SetId(testDomain)

	err := namedotcom.ResourceDomainNameServersRead(data, client)
	if err != nil {
		t.Fatalf("expected nil error for not-found domain, got: %v", err)
	}

	if data.Id() != "" {
		t.Errorf("ID should be empty after domain not found, got %q", data.Id())
	}
}

func TestIsDomainNotFound(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{name: "nil error", err: nil, expected: false},
		{name: "404 in message", err: errors.New("404 not found"), expected: true},
		{name: "Not Found text", err: errors.New("Not Found"), expected: true},
		{name: "other error", err: errors.New("connection refused"), expected: false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := namedotcom.IsDomainNotFound(testCase.err)
			if got != testCase.expected {
				t.Errorf("isDomainNotFound(%v) = %v, want %v", testCase.err, got, testCase.expected)
			}
		})
	}
}

func TestResourceDomainNameServersCreate_VerifiesAPICall(t *testing.T) {
	initLimiters(t)

	setCalled := false

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com:setNameservers", func(writer http.ResponseWriter, request *http.Request) {
		setCalled = true

		if request.Method != http.MethodPost {
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)

			return
		}

		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(writer, `{}`)
	})

	mux.HandleFunc("/v4/domains/example.com", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(writer, `{"domainName":"example.com","nameservers":["ns1.example.com"]}`)
	})

	client := newMockClient(t, mux)

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceDomainNameServers().Schema, map[string]any{
		"domain_name": testDomain,
		"nameservers": []any{"ns1.example.com"},
	})

	err := namedotcom.ResourceDomainNameServersCreate(data, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !setCalled {
		t.Fatal("SetNameservers API was not called during Create")
	}
}

func TestResourceDomainNameServersUpdate_VerifiesAPICall(t *testing.T) {
	initLimiters(t)

	setCalled := false

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com:setNameservers", func(writer http.ResponseWriter, request *http.Request) {
		setCalled = true

		if request.Method != http.MethodPost {
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)

			return
		}

		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(writer, `{}`)
	})

	mux.HandleFunc("/v4/domains/example.com", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(writer, `{"domainName":"example.com","nameservers":["ns3.example.com"]}`)
	})

	client := newMockClient(t, mux)

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceDomainNameServers().Schema, map[string]any{
		"domain_name": testDomain,
		"nameservers": []any{"ns3.example.com"},
	})
	data.SetId(testDomain)

	err := namedotcom.ResourceDomainNameServersUpdate(data, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !setCalled {
		t.Fatal("SetNameservers API was not called during Update")
	}
}

func TestResourceDomainNameServersUpdate_Success(t *testing.T) {
	initLimiters(t)

	mux := http.NewServeMux()

	mux.HandleFunc("/v4/domains/example.com:setNameservers", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)

			return
		}

		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(writer, `{}`)
	})

	mux.HandleFunc("/v4/domains/example.com", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(writer, `{"domainName":"example.com","nameservers":["ns3.example.com","ns4.example.com"]}`)
	})

	client := newMockClient(t, mux)

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceDomainNameServers().Schema, map[string]any{
		"domain_name": testDomain,
		"nameservers": []any{"ns3.example.com", "ns4.example.com"},
	})
	data.SetId(testDomain)

	err := namedotcom.ResourceDomainNameServersUpdate(data, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nameservers, ok := data.Get("nameservers").([]any)
	if !ok {
		t.Fatal("nameservers is not []any")
	}

	if len(nameservers) != 2 {
		t.Fatalf("expected 2 nameservers, got %d", len(nameservers))
	}

	if nameservers[0] != "ns3.example.com" {
		t.Errorf("nameservers[0] = %q, want %q", nameservers[0], "ns3.example.com")
	}
}

func TestResourceDomainNameServersUpdate_APIError(t *testing.T) {
	initLimiters(t)

	client := newErrorMock(t, "/v4/domains/example.com:setNameservers")

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceDomainNameServers().Schema, map[string]any{
		"domain_name": testDomain,
		"nameservers": []any{"ns1.example.com"},
	})
	data.SetId(testDomain)

	err := namedotcom.ResourceDomainNameServersUpdate(data, client)
	if err == nil {
		t.Fatal("expected error from API, got nil")
	}
}

func TestResourceDomainNameServersUpdate_InvalidMeta(t *testing.T) {
	t.Parallel()

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceDomainNameServers().Schema, map[string]any{
		"domain_name": testDomain,
		"nameservers": []any{"ns1.example.com"},
	})
	data.SetId(testDomain)

	err := namedotcom.ResourceDomainNameServersUpdate(data, "not a client")
	if err == nil {
		t.Fatal("expected error for invalid meta, got nil")
	}
}

// Record Importer tests.

func TestResourceRecordImporter_Success(t *testing.T) {
	t.Parallel()

	res := namedotcom.ResourceRecord()
	data := schema.TestResourceDataRaw(t, res.Schema, map[string]any{
		"domain_name": "",
		"host":        "",
		"record_type": "",
		"answer":      "",
	})
	data.SetId("example.com:42")

	results, err := namedotcom.ResourceRecordImporter(data, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	assertResourceData(t, results[0], "domain_name", testDomain)

	if results[0].Id() != "42" {
		t.Errorf("ID = %q, want %q", results[0].Id(), "42")
	}
}

func TestResourceRecordImporter_InvalidFormat(t *testing.T) {
	t.Parallel()

	res := namedotcom.ResourceRecord()
	data := schema.TestResourceDataRaw(t, res.Schema, map[string]any{
		"domain_name": "",
		"host":        "",
		"record_type": "",
		"answer":      "",
	})
	data.SetId("invalid-no-colon")

	_, err := namedotcom.ResourceRecordImporter(data, nil)
	if err == nil {
		t.Fatal("expected error for invalid import ID, got nil")
	}
}

// DNSSEC Importer tests.

func TestResourceDNSSECImporter_Success(t *testing.T) {
	initLimiters(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com/dnssec/AABBCCDD", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")

		_, writeErr := writer.Write(mustJSON(t, testDNSSECResponse()))
		if writeErr != nil {
			t.Errorf("failed to write response: %v", writeErr)
		}
	})

	client := newMockClient(t, mux)

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceDNSSEC().Schema, map[string]any{
		"domain_name": "",
		"key_tag":     0,
		"algorithm":   0,
		"digest_type": 0,
		"digest":      "",
	})
	data.SetId("example.com_AABBCCDD")

	results, err := namedotcom.ResourceDNSSECImporter(data, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	assertResourceData(t, results[0], "domain_name", testDomain)
	assertResourceData(t, results[0], "digest", "AABBCCDD")
	assertResourceDataInt(t, results[0], "key_tag", 12345)
	assertResourceDataInt(t, results[0], "algorithm", 8)
	assertResourceDataInt(t, results[0], "digest_type", 2)

	if results[0].Id() != testDomain {
		t.Errorf("ID = %q, want %q", results[0].Id(), testDomain)
	}
}

func TestResourceDNSSECImporter_InvalidFormat(t *testing.T) {
	initLimiters(t)

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceDNSSEC().Schema, map[string]any{
		"domain_name": "",
		"key_tag":     0,
		"algorithm":   0,
		"digest_type": 0,
		"digest":      "",
	})
	data.SetId("invalid-no-underscore")

	client := newMockClient(t, http.NewServeMux())

	_, err := namedotcom.ResourceDNSSECImporter(data, client)
	if err == nil {
		t.Fatal("expected error for invalid import ID, got nil")
	}
}

func TestResourceDNSSECImporter_InvalidMeta(t *testing.T) {
	t.Parallel()

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceDNSSEC().Schema, map[string]any{
		"domain_name": "",
		"key_tag":     0,
		"algorithm":   0,
		"digest_type": 0,
		"digest":      "",
	})
	data.SetId("example.com_AABBCCDD")

	_, err := namedotcom.ResourceDNSSECImporter(data, "not a client")
	if err == nil {
		t.Fatal("expected error for invalid meta, got nil")
	}
}

func TestResourceDNSSECImporter_APIError(t *testing.T) {
	initLimiters(t)

	client := newErrorMock(t, "/v4/domains/example.com/dnssec/AABBCCDD")

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceDNSSEC().Schema, map[string]any{
		"domain_name": "",
		"key_tag":     0,
		"algorithm":   0,
		"digest_type": 0,
		"digest":      "",
	})
	data.SetId("example.com_AABBCCDD")

	_, err := namedotcom.ResourceDNSSECImporter(data, client)
	if err == nil {
		t.Fatal("expected error from API, got nil")
	}
}

// getDNSSECFromResourceData tests.

func TestGetDNSSECFromResourceData_Success(t *testing.T) {
	t.Parallel()

	data := schema.TestResourceDataRaw(t, namedotcom.ResourceDNSSEC().Schema, map[string]any{
		"domain_name": testDomain,
		"key_tag":     12345,
		"algorithm":   8,
		"digest_type": 2,
		"digest":      "AABBCCDD",
	})

	dnssec, err := namedotcom.GetDNSSECFromResourceData(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if dnssec.DomainName != testDomain {
		t.Errorf("DomainName = %q, want %q", dnssec.DomainName, testDomain)
	}

	if dnssec.KeyTag != 12345 {
		t.Errorf("KeyTag = %d, want %d", dnssec.KeyTag, 12345)
	}

	if dnssec.Algorithm != 8 {
		t.Errorf("Algorithm = %d, want %d", dnssec.Algorithm, 8)
	}

	if dnssec.DigestType != 2 {
		t.Errorf("DigestType = %d, want %d", dnssec.DigestType, 2)
	}

	if dnssec.Digest != "AABBCCDD" {
		t.Errorf("Digest = %q, want %q", dnssec.Digest, "AABBCCDD")
	}
}

// Test helpers.

func assertResourceData(t *testing.T, data *schema.ResourceData, key, expected string) {
	t.Helper()

	got, ok := data.Get(key).(string)
	if !ok {
		t.Errorf("field %q is not a string", key)

		return
	}

	if got != expected {
		t.Errorf("field %q = %q, want %q", key, got, expected)
	}
}

func assertResourceDataInt(t *testing.T, data *schema.ResourceData, key string, expected int) {
	t.Helper()

	got, ok := data.Get(key).(int)
	if !ok {
		t.Errorf("field %q is not an int", key)

		return
	}

	if got != expected {
		t.Errorf("field %q = %d, want %d", key, got, expected)
	}
}
