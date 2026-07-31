package witgo

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// ImportNames returns a sorted copy of the required host function names.
func (c Contract) ImportNames() []string {
	names, _ := normalizedFunctions(c.Imports)
	return append([]string(nil), names...)
}

// ExportNames returns a sorted copy of the provided plugin function names.
func (c Contract) ExportNames() []string {
	names, _ := normalizedFunctions(c.Exports)
	return append([]string(nil), names...)
}

// FunctionNames returns all imported and exported function names in sorted
// order without duplicates.
func (c Contract) FunctionNames() []string {
	names, _ := normalizedFunctions(append(append([]string(nil), c.Imports...), c.Exports...))
	return append([]string(nil), names...)
}

// Signature returns the structural Component Model signature for a function.
func (c Contract) Signature(function string) (string, bool) {
	signature, ok := c.Signatures[function]
	return signature, ok
}

// Requires reports whether the component imports the named host function.
func (c Contract) Requires(function string) bool {
	return containsString(c.Imports, function)
}

// Provides reports whether the component exports the named plugin function.
func (c Contract) Provides(function string) bool {
	return containsString(c.Exports, function)
}

// ContractDifference describes missing and unexpected function names on one
// side of a component contract.
type ContractDifference struct {
	Missing    []string
	Unexpected []string
}

// Empty reports whether there are no missing or unexpected functions.
func (d ContractDifference) Empty() bool {
	return len(d.Missing) == 0 && len(d.Unexpected) == 0
}

// Count returns the total number of missing and unexpected functions.
func (d ContractDifference) Count() int {
	return len(d.Missing) + len(d.Unexpected)
}

// SignatureMismatch describes a function whose Component Model type differs
// from the generated WIT contract.
type SignatureMismatch struct {
	Function string
	Expected string
	Actual   string
}

// ValidationReport is the result of inspecting a component without
// instantiating it or running guest code.
type ValidationReport struct {
	Compatible bool
	Expected   Contract
	Actual     Contract
	Imports    ContractDifference
	Exports    ContractDifference
	Signatures []SignatureMismatch
}

// ContractValidationError wraps an incompatible report and supports
// errors.Is(err, ErrContractMismatch).
type ContractValidationError struct {
	Report ValidationReport
}

func (e *ContractValidationError) Error() string {
	if e == nil {
		return ErrContractMismatch.Error()
	}
	return e.Report.Summary()
}

func (e *ContractValidationError) Unwrap() error { return ErrContractMismatch }

// Err converts an incompatible report into an error. It returns nil for a
// compatible report.
func (r ValidationReport) Err() error {
	if r.Compatible {
		return nil
	}
	return &ContractValidationError{Report: r}
}

// ProblemCount returns the number of missing, unexpected, and type-mismatched
// functions in the report.
func (r ValidationReport) ProblemCount() int {
	return r.Imports.Count() + r.Exports.Count() + len(r.Signatures)
}

// Summary returns a concise deterministic description suitable for logs.
func (r ValidationReport) Summary() string {
	if r.Compatible {
		return "component contract is compatible"
	}
	var problems []string
	if len(r.Imports.Missing) > 0 {
		problems = append(problems, fmt.Sprintf("missing imports=%v", r.Imports.Missing))
	}
	if len(r.Imports.Unexpected) > 0 {
		problems = append(problems, fmt.Sprintf("unexpected imports=%v", r.Imports.Unexpected))
	}
	if len(r.Exports.Missing) > 0 {
		problems = append(problems, fmt.Sprintf("missing exports=%v", r.Exports.Missing))
	}
	if len(r.Exports.Unexpected) > 0 {
		problems = append(problems, fmt.Sprintf("unexpected exports=%v", r.Exports.Unexpected))
	}
	for _, mismatch := range r.Signatures {
		problems = append(problems, fmt.Sprintf("signature %s expected=%q actual=%q", mismatch.Function, mismatch.Expected, mismatch.Actual))
	}
	if len(problems) == 0 {
		return ErrContractMismatch.Error()
	}
	return ErrContractMismatch.Error() + ": " + strings.Join(problems, "; ")
}

// InspectComponent returns the imported and exported function names exposed by
// a WebAssembly component without instantiating it.
func InspectComponent(filename string) (Contract, error) {
	return InspectComponentWithOptions(filename, RuntimeOptions{})
}

// InspectComponentWithOptions is InspectComponent with bridge selection and
// resource options. Execution limits are not consumed because inspection does
// not instantiate or call the component.
func InspectComponentWithOptions(filename string, options RuntimeOptions) (Contract, error) {
	if err := validateRuntimeOptions(options); err != nil {
		return Contract{}, err
	}
	path, err := componentPath(filename)
	if err != nil {
		return Contract{}, err
	}
	return inspectComponentBridge(path, options)
}

// InspectComponentBytes inspects a component held in memory.
func InspectComponentBytes(data []byte) (Contract, error) {
	return InspectComponentBytesWithOptions(data, RuntimeOptions{})
}

// InspectComponentBytesWithOptions is InspectComponentBytes with explicit
// bridge options.
func InspectComponentBytesWithOptions(data []byte, options RuntimeOptions) (Contract, error) {
	name, err := writeTemporaryComponent(data)
	if err != nil {
		return Contract{}, err
	}
	defer os.Remove(name)
	return InspectComponentWithOptions(name, options)
}

// ValidateComponent compares a component with a generated contract without
// instantiating it or running guest code. Incompatibility is returned in the
// report; err is reserved for inspection and bridge failures.
func ValidateComponent(filename string, expected Contract) (ValidationReport, error) {
	return ValidateComponentWithOptions(filename, RuntimeOptions{}, expected)
}

// ValidateComponentWithOptions is ValidateComponent with explicit runtime
// options controlling bridge selection and inspection limits.
func ValidateComponentWithOptions(filename string, options RuntimeOptions, expected Contract) (ValidationReport, error) {
	actual, err := InspectComponentWithOptions(filename, options)
	if err != nil {
		return ValidationReport{}, err
	}
	return CompareContracts(expected, actual)
}

// ValidateComponentBytes compares an in-memory component with a contract.
func ValidateComponentBytes(data []byte, expected Contract) (ValidationReport, error) {
	return ValidateComponentBytesWithOptions(data, RuntimeOptions{}, expected)
}

// ValidateComponentBytesWithOptions is ValidateComponentBytes with explicit
// bridge options.
func ValidateComponentBytesWithOptions(data []byte, options RuntimeOptions, expected Contract) (ValidationReport, error) {
	actual, err := InspectComponentBytesWithOptions(data, options)
	if err != nil {
		return ValidationReport{}, err
	}
	return CompareContracts(expected, actual)
}

// CompareContracts compares two already-inspected manifests without loading a
// bridge or touching the filesystem.
func CompareContracts(expected, actual Contract) (ValidationReport, error) {
	return buildValidationReport(expected, actual)
}

// RequireCompatible returns nil when report is compatible and otherwise
// returns an error wrapping ErrContractMismatch.
func RequireCompatible(report ValidationReport) error {
	return report.Err()
}

func buildValidationReport(expected, actual Contract) (ValidationReport, error) {
	wantImports, duplicateWantImports := normalizedFunctions(expected.Imports)
	wantExports, duplicateWantExports := normalizedFunctions(expected.Exports)
	gotImports, duplicateGotImports := normalizedFunctions(actual.Imports)
	gotExports, duplicateGotExports := normalizedFunctions(actual.Exports)
	if len(duplicateWantImports)+len(duplicateWantExports)+len(duplicateGotImports)+len(duplicateGotExports) != 0 {
		return ValidationReport{}, fmt.Errorf("%w: duplicate function names: expected imports=%v exports=%v, actual imports=%v exports=%v",
			ErrContractMismatch, duplicateWantImports, duplicateWantExports, duplicateGotImports, duplicateGotExports)
	}
	report := ValidationReport{
		Expected: Contract{Imports: append([]string(nil), wantImports...), Exports: append([]string(nil), wantExports...), Signatures: cloneSignatures(expected.Signatures)},
		Actual:   Contract{Imports: append([]string(nil), gotImports...), Exports: append([]string(nil), gotExports...), Signatures: cloneSignatures(actual.Signatures)},
		Imports: ContractDifference{
			Missing:    difference(wantImports, gotImports),
			Unexpected: difference(gotImports, wantImports),
		},
		Exports: ContractDifference{
			Missing:    difference(wantExports, gotExports),
			Unexpected: difference(gotExports, wantExports),
		},
	}
	for name, want := range expected.Signatures {
		if got := actual.Signatures[name]; got != want {
			report.Signatures = append(report.Signatures, SignatureMismatch{Function: name, Expected: want, Actual: got})
		}
	}
	sort.Slice(report.Signatures, func(i, j int) bool { return report.Signatures[i].Function < report.Signatures[j].Function })
	report.Compatible = len(report.Imports.Missing) == 0 && len(report.Imports.Unexpected) == 0 &&
		len(report.Exports.Missing) == 0 && len(report.Exports.Unexpected) == 0 && len(report.Signatures) == 0
	return report, nil
}
