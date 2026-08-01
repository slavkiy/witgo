package witgo

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestContractConvenienceMethods(t *testing.T) {
	contract := Contract{
		Imports:    []string{"host#write", "host#read"},
		Exports:    []string{"api#run"},
		Signatures: map[string]string{"api#run": "()->()"},
	}
	if got := contract.ImportNames(); !reflect.DeepEqual(got, []string{"host#read", "host#write"}) {
		t.Fatalf("ImportNames = %v", got)
	}
	if got := contract.FunctionNames(); !reflect.DeepEqual(got, []string{"api#run", "host#read", "host#write"}) {
		t.Fatalf("FunctionNames = %v", got)
	}
	if !contract.Requires("host#read") || !contract.Provides("api#run") {
		t.Fatal("Requires or Provides did not find a function")
	}
	if signature, ok := contract.Signature("api#run"); !ok || signature != "()->()" {
		t.Fatalf("Signature = %q, %v", signature, ok)
	}
}

func TestValidationReportError(t *testing.T) {
	report := ValidationReport{
		Imports: ContractDifference{Missing: []string{"host#read"}},
		Signatures: []SignatureMismatch{{
			Function: "api#run",
			Expected: "(string)->()",
			Actual:   "(u64)->()",
		}},
	}
	err := report.Err()
	if !errors.Is(err, ErrContractMismatch) {
		t.Fatalf("Err = %v, want ErrContractMismatch", err)
	}
	if report.ProblemCount() != 2 || !strings.Contains(report.Summary(), "host#read") || !strings.Contains(report.Summary(), "api#run") {
		t.Fatalf("unexpected report summary: %q", report.Summary())
	}
	var typed *ContractValidationError
	if !errors.As(err, &typed) || typed.Report.ProblemCount() != 2 {
		t.Fatalf("error does not preserve report: %#v", err)
	}

	compatible := ValidationReport{Compatible: true}
	if err := RequireCompatible(compatible); err != nil {
		t.Fatalf("compatible report returned %v", err)
	}
}

func TestInspectComponentBytesRejectsInvalidData(t *testing.T) {
	if _, err := InspectComponentBytes([]byte("not wasm")); err == nil {
		t.Fatal("invalid component bytes were accepted")
	}
}

func TestCompareContracts(t *testing.T) {
	report, err := CompareContracts(Contract{Exports: []string{"api#run"}}, Contract{Exports: []string{"api#run"}})
	if err != nil || !report.Compatible {
		t.Fatalf("CompareContracts = %#v, %v", report, err)
	}
}

func TestCompareContractsMismatchDetails(t *testing.T) {
	report, err := CompareContracts(
		Contract{
			Imports:    []string{"host#read"},
			Exports:    []string{"api#run"},
			Signatures: map[string]string{"api#run": "(string)->()"},
		},
		Contract{
			Imports:    []string{"host#write"},
			Exports:    []string{"api#run", "api#debug"},
			Signatures: map[string]string{"api#run": "(u32)->()"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if report.Compatible {
		t.Fatal("incompatible contracts were reported compatible")
	}
	if !reflect.DeepEqual(report.Imports.Missing, []string{"host#read"}) {
		t.Fatalf("missing imports = %v", report.Imports.Missing)
	}
	if !reflect.DeepEqual(report.Imports.Unexpected, []string{"host#write"}) {
		t.Fatalf("unexpected imports = %v", report.Imports.Unexpected)
	}
	if !reflect.DeepEqual(report.Exports.Unexpected, []string{"api#debug"}) {
		t.Fatalf("unexpected exports = %v", report.Exports.Unexpected)
	}
	if len(report.Signatures) != 1 || report.Signatures[0].Function != "api#run" {
		t.Fatalf("signature mismatches = %#v", report.Signatures)
	}
	summary := report.Summary()
	if !strings.Contains(summary, "host#read") || !strings.Contains(summary, "api#debug") || !strings.Contains(summary, `expected="(string)->()" actual="(u32)->()"`) {
		t.Fatalf("unexpected summary = %q", summary)
	}
}

func TestCapabilityPolicy(t *testing.T) {
	policy := CapabilityPolicy{
		Allow: []string{"example:plugins/host@1.0.0", "wasi:logging/logger@0.2.0#write*"},
		Deny:  []string{"example:plugins/host@1.0.0#delete-all"},
	}
	for _, allowed := range []string{
		"example:plugins/host@1.0.0#read",
		"wasi:logging/logger@0.2.0#write-info",
	} {
		if !policy.Allows(allowed) {
			t.Fatalf("policy rejected %q", allowed)
		}
	}
	for _, denied := range []string{
		"example:plugins/host@1.0.0#delete-all",
		"example:plugins/admin@1.0.0#read",
	} {
		if policy.Allows(denied) {
			t.Fatalf("policy allowed %q", denied)
		}
	}
	err := policy.ValidateImports([]string{
		"example:plugins/host@1.0.0#read",
		"example:plugins/host@1.0.0#delete-all",
		"example:plugins/admin@1.0.0#read",
	})
	if !errors.Is(err, ErrCapabilityDenied) {
		t.Fatalf("ValidateImports error = %v", err)
	}
	var typed *CapabilityPolicyError
	if !errors.As(err, &typed) || !reflect.DeepEqual(typed.Denied, []string{
		"example:plugins/admin@1.0.0#read",
		"example:plugins/host@1.0.0#delete-all",
	}) {
		t.Fatalf("capability error = %#v", err)
	}
}

func TestRuntimeIsClosed(t *testing.T) {
	var runtime *Runtime
	if !runtime.IsClosed() {
		t.Fatal("nil runtime is not closed")
	}
	runtime = &Runtime{bridge: &componentBridge{}}
	if runtime.IsClosed() {
		t.Fatal("open runtime is closed")
	}
	runtime.bridge.closed = true
	if !runtime.IsClosed() {
		t.Fatal("closed runtime is open")
	}
}
