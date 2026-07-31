package witgo

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestValidateBridgeHandshake(t *testing.T) {
	valid := bridgeMessage{Type: "pong", ProtocolVersion: bridgeProtocolVersion, BridgeVersion: bridgeVersion, WasmtimeVersion: "47.0.2", WitgoVersion: "test", Features: append([]string(nil), bridgeRequiredFeatures...)}
	if err := validateBridgeHandshake(valid, "test"); err != nil {
		t.Fatal(err)
	}
	wrongProtocol := valid
	wrongProtocol.ProtocolVersion++
	if err := validateBridgeHandshake(wrongProtocol, "test"); !errors.Is(err, ErrBridgeProtocolMismatch) || !strings.Contains(err.Error(), "protocol") {
		t.Fatalf("expected protocol error, got %v", err)
	}
	wrongBridge := valid
	wrongBridge.BridgeVersion = "99.0.0"
	if err := validateBridgeHandshake(wrongBridge, "test"); !errors.Is(err, ErrBridgeVersionMismatch) || !strings.Contains(err.Error(), "bridge version") {
		t.Fatalf("expected bridge error, got %v", err)
	}
	missingFeature := valid
	missingFeature.Features = nil
	if err := validateBridgeHandshake(missingFeature, "test"); !errors.Is(err, ErrBridgeProtocolMismatch) {
		t.Fatalf("expected feature protocol error, got %v", err)
	}
}

func TestValidateContract(t *testing.T) {
	expected := Contract{
		Imports: []string{"test:plugin/host@1.0.0#log", "test:plugin/host@1.0.0#read"},
		Exports: []string{"test:plugin/api@1.0.0#run"},
	}
	if err := validateContract(expected,
		[]string{"test:plugin/host@1.0.0#read", "test:plugin/host@1.0.0#log"},
		[]string{"test:plugin/api@1.0.0#run"}); err != nil {
		t.Fatal(err)
	}
	err := validateContract(expected,
		[]string{"test:plugin/host@1.0.0#log"},
		[]string{"test:plugin/api@1.0.0#other"})
	if !errors.Is(err, ErrContractMismatch) || !strings.Contains(err.Error(), "read") {
		t.Fatalf("expected useful contract mismatch, got %v", err)
	}
}

func TestCompareSignatures(t *testing.T) {
	err := compareSignatures(
		map[string]string{"api#run": "(string)->(u64)"},
		map[string]string{"api#run": "(string)->(u32)"},
	)
	if !errors.Is(err, ErrContractMismatch) || !strings.Contains(err.Error(), "u64") || !strings.Contains(err.Error(), "u32") {
		t.Fatalf("expected typed contract mismatch, got %v", err)
	}
}

func TestBuildValidationReport(t *testing.T) {
	report, err := buildValidationReport(
		Contract{Imports: []string{"host#read", "host#write"}, Exports: []string{"api#run"}, Signatures: map[string]string{"host#read": "()->(string)"}},
		Contract{Imports: []string{"host#read", "host#delete"}, Exports: []string{"api#other"}, Signatures: map[string]string{"host#read": "()->(u64)"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if report.Compatible {
		t.Fatal("incompatible component was reported compatible")
	}
	if !reflect.DeepEqual(report.Imports.Missing, []string{"host#write"}) ||
		!reflect.DeepEqual(report.Imports.Unexpected, []string{"host#delete"}) ||
		!reflect.DeepEqual(report.Exports.Missing, []string{"api#run"}) ||
		!reflect.DeepEqual(report.Exports.Unexpected, []string{"api#other"}) ||
		len(report.Signatures) != 1 || report.Signatures[0].Function != "host#read" {
		t.Fatalf("unexpected report: %#v", report)
	}

	compatible, err := buildValidationReport(
		Contract{Imports: []string{"b", "a"}},
		Contract{Imports: []string{"a", "b"}},
	)
	if err != nil || !compatible.Compatible {
		t.Fatalf("compatible report = %#v, %v", compatible, err)
	}
}

func TestCallAfterCloseReturnsSentinel(t *testing.T) {
	runtime := &Runtime{bridge: &componentBridge{closed: true}}
	if _, err := runtime.Call("test:api/run"); !errors.Is(err, ErrRuntimeClosed) {
		t.Fatalf("Call error = %v, want ErrRuntimeClosed", err)
	}
}

func TestVerifyBridgeFileSHA256(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bridge")
	if err := os.WriteFile(path, []byte("abc"), 0o755); err != nil {
		t.Fatal(err)
	}
	const digest = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if err := verifyBridgeFile(path, digest); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("tampered"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := verifyBridgeFile(path, digest); err == nil {
		t.Fatal("tampered library passed verification")
	}
}
