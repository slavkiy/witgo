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
