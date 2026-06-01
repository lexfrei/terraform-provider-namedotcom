package namedotcom_test

import (
	"testing"

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
			name: "multiple underscores splits at last", input: "my_domain.com_digest",
			wantFirst: "my_domain.com", wantSecond: "digest",
		},
		{name: "empty domain", input: "_ABCDEF", wantErr: true},
		{name: "empty digest", input: "example.com_", wantErr: true},
		{name: "no separator", input: "example.com", wantErr: true},
		{name: "empty string", input: "", wantErr: true},
	}, namedotcom.ResourceDNSSECImporterParseID)
}

func TestParseRecordID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    int32
		wantErr bool
	}{
		{name: "valid", input: "42", want: 42},
		{name: "zero", input: "0", want: 0},
		{name: "not a number", input: "abc", wantErr: true},
		{name: "empty", input: "", wantErr: true},
		{name: "overflow int32", input: "2147483648", wantErr: true},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := namedotcom.ParseRecordID(testCase.input)

			if testCase.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != testCase.want {
				t.Errorf("ParseRecordID(%q) = %d, want %d", testCase.input, got, testCase.want)
			}
		})
	}
}
