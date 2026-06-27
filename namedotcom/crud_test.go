//nolint:paralleltest // CRUD tests modify global rate limiter state
package namedotcom_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/hashicorp/terraform-plugin-framework/types"
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

func newErrorMock(t *testing.T, pattern string) *namecom.NameCom {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc(pattern, func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, `{"message":"server error"}`, http.StatusInternalServerError)
	})

	return newMockClient(t, mux)
}

// Record API helper tests.

func TestCreateRecordAPI_Success(t *testing.T) {
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

	client := newMockClient(t, mux)

	record, err := namedotcom.CreateRecordAPI(context.Background(), client, &namecom.Record{
		DomainName: testDomain, Host: "test", Type: "A", Answer: "1.2.3.4",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if record.ID != 42 {
		t.Errorf("ID = %d, want 42", record.ID)
	}

	if record.Answer != "1.2.3.4" {
		t.Errorf("Answer = %q, want %q", record.Answer, "1.2.3.4")
	}
}

// TestCreateRecordAPI_SendsPriority confirms an MX record's priority reaches the
// API request body and is read back from the response.
func TestCreateRecordAPI_SendsPriority(t *testing.T) {
	initLimiters(t)

	var gotPriority uint32

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com/records", func(writer http.ResponseWriter, request *http.Request) {
		var body namecom.Record

		err := json.NewDecoder(request.Body).Decode(&body)
		if err != nil {
			t.Fatalf("decoding request body: %v", err)
		}

		gotPriority = body.Priority

		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(writer, `{"id":42,"domainName":"example.com","host":"","type":"MX","answer":"mail.example.com","priority":10}`)
	})

	client := newMockClient(t, mux)

	record, err := namedotcom.CreateRecordAPI(context.Background(), client, &namecom.Record{
		DomainName: testDomain, Host: "", Type: "MX", Answer: "mail.example.com", Priority: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotPriority != 10 {
		t.Errorf("priority sent to API = %d, want 10", gotPriority)
	}

	if record.Priority != 10 {
		t.Errorf("record.Priority = %d, want 10", record.Priority)
	}
}

func TestCreateRecordAPI_APIError(t *testing.T) {
	initLimiters(t)

	client := newErrorMock(t, "/v4/domains/example.com/records")

	_, err := namedotcom.CreateRecordAPI(context.Background(), client, &namecom.Record{
		DomainName: testDomain, Host: "test", Type: "A", Answer: "1.2.3.4",
	})
	if err == nil {
		t.Fatal("expected error from API, got nil")
	}
}

func TestReadRecordAPI_Success(t *testing.T) {
	initLimiters(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com/records/42", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(writer, `{"id":42,"domainName":"example.com","host":"www","type":"CNAME","answer":"example.com."}`)
	})

	client := newMockClient(t, mux)

	record, err := namedotcom.ReadRecordAPI(context.Background(), client, testDomain, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if record.Host != "www" || record.Type != "CNAME" || record.Answer != "example.com." {
		t.Errorf("unexpected record: %+v", record)
	}
}

func TestReadRecordAPI_APIError(t *testing.T) {
	initLimiters(t)

	client := newErrorMock(t, "/v4/domains/example.com/records/42")

	_, err := namedotcom.ReadRecordAPI(context.Background(), client, testDomain, 42)
	if err == nil {
		t.Fatal("expected error from API, got nil")
	}
}

func TestUpdateRecordAPI_Success(t *testing.T) {
	initLimiters(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com/records/42", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(writer, `{"id":42,"domainName":"example.com","host":"test","type":"A","answer":"5.6.7.8"}`)
	})

	client := newMockClient(t, mux)

	record, err := namedotcom.UpdateRecordAPI(context.Background(), client, 42, &namecom.Record{
		DomainName: testDomain, Host: "test", Type: "A", Answer: "5.6.7.8",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if record.Answer != "5.6.7.8" {
		t.Errorf("Answer = %q, want %q", record.Answer, "5.6.7.8")
	}
}

func TestUpdateRecordAPI_APIError(t *testing.T) {
	initLimiters(t)

	client := newErrorMock(t, "/v4/domains/example.com/records/42")

	_, err := namedotcom.UpdateRecordAPI(context.Background(), client, 42, &namecom.Record{
		DomainName: testDomain, Host: "test", Type: "A", Answer: "1.2.3.4",
	})
	if err == nil {
		t.Fatal("expected error from API, got nil")
	}
}

func TestDeleteRecordAPI_Success(t *testing.T) {
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

	err := namedotcom.DeleteRecordAPI(context.Background(), client, testDomain, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteRecordAPI_APIError(t *testing.T) {
	initLimiters(t)

	client := newErrorMock(t, "/v4/domains/example.com/records/42")

	err := namedotcom.DeleteRecordAPI(context.Background(), client, testDomain, 42)
	if err == nil {
		t.Fatal("expected error from API, got nil")
	}
}

// DNSSEC API helper tests.

func testDNSSECResponse() *namecom.DNSSEC {
	return &namecom.DNSSEC{
		DomainName: testDomain,
		KeyTag:     12345,
		Algorithm:  8,
		DigestType: 2,
		Digest:     "AABBCCDD",
	}
}

func TestCreateDNSSECAPI_Success(t *testing.T) {
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

	client := newMockClient(t, mux)

	err := namedotcom.CreateDNSSECAPI(context.Background(), client, testDomain, 12345, 8, 2, "AABBCCDD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateDNSSECAPI_APIError(t *testing.T) {
	initLimiters(t)

	client := newErrorMock(t, "/v4/domains/example.com/dnssec")

	err := namedotcom.CreateDNSSECAPI(context.Background(), client, testDomain, 12345, 8, 2, "AABBCCDD")
	if err == nil {
		t.Fatal("expected error from API, got nil")
	}
}

func TestReadDNSSECAPI_Success(t *testing.T) {
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

	dnssec, err := namedotcom.ReadDNSSECAPI(context.Background(), client, testDomain, "AABBCCDD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if dnssec.KeyTag != 12345 || dnssec.Algorithm != 8 || dnssec.DigestType != 2 {
		t.Errorf("unexpected dnssec: %+v", dnssec)
	}
}

func TestReadDNSSECAPI_APIError(t *testing.T) {
	initLimiters(t)

	client := newErrorMock(t, "/v4/domains/example.com/dnssec/AABBCCDD")

	_, err := namedotcom.ReadDNSSECAPI(context.Background(), client, testDomain, "AABBCCDD")
	if err == nil {
		t.Fatal("expected error from API, got nil")
	}
}

func TestDeleteDNSSECAPI_Success(t *testing.T) {
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

	err := namedotcom.DeleteDNSSECAPI(context.Background(), client, testDomain, "AABBCCDD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteDNSSECAPI_APIError(t *testing.T) {
	initLimiters(t)

	client := newErrorMock(t, "/v4/domains/example.com/dnssec/AABBCCDD")

	err := namedotcom.DeleteDNSSECAPI(context.Background(), client, testDomain, "AABBCCDD")
	if err == nil {
		t.Fatal("expected error from API, got nil")
	}
}

// Nameservers API helper tests.

func TestSetNameserversAPI_Success(t *testing.T) {
	initLimiters(t)

	called := false

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com:setNameservers", func(writer http.ResponseWriter, request *http.Request) {
		called = true

		if request.Method != http.MethodPost {
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)

			return
		}

		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(writer, `{}`)
	})

	client := newMockClient(t, mux)

	err := namedotcom.SetNameserversAPI(context.Background(), client, testDomain, []string{"ns1.example.com", "ns2.example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !called {
		t.Fatal("SetNameservers API was not called")
	}
}

func TestSetNameserversAPI_APIError(t *testing.T) {
	initLimiters(t)

	client := newErrorMock(t, "/v4/domains/example.com:setNameservers")

	err := namedotcom.SetNameserversAPI(context.Background(), client, testDomain, []string{"ns1.example.com"})
	if err == nil {
		t.Fatal("expected error from API, got nil")
	}
}

func TestReadNameserversAPI_Success(t *testing.T) {
	initLimiters(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(writer, `{"domainName":"example.com","nameservers":["ns1.example.com","ns2.example.com"]}`)
	})

	client := newMockClient(t, mux)

	domain, found, err := namedotcom.ReadNameserversAPI(context.Background(), client, testDomain)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !found {
		t.Fatal("expected found=true")
	}

	if len(domain.Nameservers) != 2 {
		t.Fatalf("expected 2 nameservers, got %d", len(domain.Nameservers))
	}
}

func TestReadNameserversAPI_DomainNotFound(t *testing.T) {
	initLimiters(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/v4/domains/example.com", func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, `{"message":"Domain not found"}`, http.StatusNotFound)
	})

	client := newMockClient(t, mux)

	domain, found, err := namedotcom.ReadNameserversAPI(context.Background(), client, testDomain)
	if err != nil {
		t.Fatalf("expected nil error for not-found domain, got: %v", err)
	}

	if found {
		t.Fatal("expected found=false for a missing domain")
	}

	if domain != nil {
		t.Errorf("expected nil domain, got %+v", domain)
	}
}

func TestReadNameserversAPI_APIError(t *testing.T) {
	initLimiters(t)

	client := newErrorMock(t, "/v4/domains/example.com")

	_, _, err := namedotcom.ReadNameserversAPI(context.Background(), client, testDomain)
	if err == nil {
		t.Fatal("expected error from API, got nil")
	}
}

// extractNameservers tests.

func TestExtractNameservers_Set(t *testing.T) {
	t.Parallel()

	set, diags := types.SetValueFrom(context.Background(), types.StringType, []string{"ns1.example.com", "ns2.example.com"})
	if diags.HasError() {
		t.Fatalf("unexpected diags building set: %v", diags)
	}

	got, extractDiags := namedotcom.ExtractNameservers(context.Background(), set)
	if extractDiags.HasError() {
		t.Fatalf("unexpected diags: %v", extractDiags)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 nameservers, got %d", len(got))
	}
}

func TestExtractNameservers_Null(t *testing.T) {
	t.Parallel()

	got, diags := namedotcom.ExtractNameservers(context.Background(), types.SetNull(types.StringType))
	if diags.HasError() {
		t.Fatalf("unexpected diags: %v", diags)
	}

	if got != nil {
		t.Errorf("expected nil for null set, got %v", got)
	}
}

func TestExtractNameservers_Unknown(t *testing.T) {
	t.Parallel()

	got, diags := namedotcom.ExtractNameservers(context.Background(), types.SetUnknown(types.StringType))
	if diags.HasError() {
		t.Fatalf("unexpected diags: %v", diags)
	}

	if got != nil {
		t.Errorf("expected nil for unknown set, got %v", got)
	}
}

func TestIsNotFoundError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{name: "nil error", err: nil, expected: false},
		{name: "404 not found message", err: errors.New("404 not found"), expected: true},
		{name: "Not Found text", err: errors.New("Not Found"), expected: true},
		{name: "got error wrapping API message", err: errors.New("got error: Domain not found: "), expected: true},
		{name: "message without a not-found substring", err: errors.New("server returned 404 bytes"), expected: false},
		{name: "other error", err: errors.New("connection refused"), expected: false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got := namedotcom.IsNotFoundError(testCase.err)
			if got != testCase.expected {
				t.Errorf("IsNotFoundError(%v) = %v, want %v", testCase.err, got, testCase.expected)
			}
		})
	}
}
