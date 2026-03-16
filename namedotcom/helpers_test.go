package namedotcom_test

import (
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/namedotcom/go/v4/namecom"

	namedotcom "github.com/lexfrei/terraform-provider-namedotcom/namedotcom"
)

type parseIDTestCase struct {
	name       string
	input      string
	wantFirst  string
	wantSecond string
	wantErr    bool
}

func runParseIDTests(
	t *testing.T,
	tests []parseIDTestCase,
	parseFn func(string) (string, string, error),
) {
	t.Helper()

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			first, second, err := parseFn(testCase.input)

			if testCase.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if first != testCase.wantFirst {
				t.Errorf("first = %q, want %q", first, testCase.wantFirst)
			}

			if second != testCase.wantSecond {
				t.Errorf("second = %q, want %q", second, testCase.wantSecond)
			}
		})
	}
}

func TestResourceRecordImporterParseID(t *testing.T) {
	t.Parallel()

	runParseIDTests(t, []parseIDTestCase{
		{name: "valid input", input: "example.com:12345", wantFirst: "example.com", wantSecond: "12345"},
		{
			name: "multiple colons keeps remainder in ID", input: "example.com:123:extra",
			wantFirst: "example.com", wantSecond: "123:extra",
		},
		{name: "empty domain", input: ":12345", wantErr: true},
		{name: "empty ID", input: "example.com:", wantErr: true},
		{name: "no separator", input: "example.com", wantErr: true},
		{name: "empty string", input: "", wantErr: true},
	}, namedotcom.ResourceRecordImporterParseID)
}

func TestResourceDNSSECImporterParseID(t *testing.T) {
	t.Parallel()

	runParseIDTests(t, []parseIDTestCase{
		{name: "valid input", input: "example.com_ABCDEF123", wantFirst: "example.com", wantSecond: "ABCDEF123"},
		{
			name: "multiple underscores splits at first", input: "my_domain.com_digest",
			wantFirst: "my", wantSecond: "domain.com_digest",
		},
		{name: "empty domain", input: "_ABCDEF", wantErr: true},
		{name: "empty digest", input: "example.com_", wantErr: true},
		{name: "no separator", input: "example.com", wantErr: true},
		{name: "empty string", input: "", wantErr: true},
	}, namedotcom.ResourceDNSSECImporterParseID)
}

func TestValidateIntForInt32(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   int
		wantErr bool
	}{
		{name: "zero", value: 0},
		{name: "positive", value: 1},
		{name: "negative", value: -1},
		{name: "max int32", value: 2147483647},
		{name: "min int32", value: -2147483648},
		{name: "overflow positive", value: 2147483648, wantErr: true},
		{name: "overflow negative", value: -2147483649, wantErr: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := namedotcom.ValidateIntForInt32(testCase.value, "test_field")

			if testCase.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				if !errors.Is(err, namedotcom.ErrValueOutsideInt32Range) {
					t.Errorf("expected ErrValueOutsideInt32Range, got: %v", err)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		meta    any
		wantErr bool
	}{
		{name: "valid namecom client", meta: &namecom.NameCom{}, wantErr: false},
		{name: "invalid type string", meta: "not a client", wantErr: true},
		{name: "nil value", meta: nil, wantErr: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			client, err := namedotcom.ValidateClient(testCase.meta)

			if testCase.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				if client != nil {
					t.Error("expected nil client on error")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if client == nil {
				t.Fatal("expected non-nil client")
			}
		})
	}
}
