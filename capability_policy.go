package witgo

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// CapabilityPolicy controls which host import functions a component may
// require. Patterns support:
//   - exact function names: namespace:pkg/interface@1.0.0#func
//   - interface-level rules: namespace:pkg/interface@1.0.0
//   - prefix wildcards ending in *: namespace:pkg/interface@1.0.0#read*
//   - "*" to match everything
//
// Deny rules override allow rules. If Allow is empty, every capability is
// allowed except those matched by Deny.
type CapabilityPolicy struct {
	Allow []string
	Deny  []string
}

// CapabilityPolicyError reports which required capabilities were denied by a
// policy.
type CapabilityPolicyError struct {
	Denied []string
}

func (e *CapabilityPolicyError) Error() string {
	if e == nil || len(e.Denied) == 0 {
		return ErrCapabilityDenied.Error()
	}
	return fmt.Sprintf("%s: %v", ErrCapabilityDenied, e.Denied)
}

func (e *CapabilityPolicyError) Unwrap() error { return ErrCapabilityDenied }

// Allows reports whether function is allowed by policy.
func (p CapabilityPolicy) Allows(function string) bool {
	for _, pattern := range p.Deny {
		if capabilityMatch(pattern, function) {
			return false
		}
	}
	if len(p.Allow) == 0 {
		return true
	}
	for _, pattern := range p.Allow {
		if capabilityMatch(pattern, function) {
			return true
		}
	}
	return false
}

// ValidateImports rejects required imports denied by policy.
func (p CapabilityPolicy) ValidateImports(imports []string) error {
	if len(imports) == 0 {
		return nil
	}
	var denied []string
	for _, function := range imports {
		if !p.Allows(function) {
			denied = append(denied, function)
		}
	}
	if len(denied) == 0 {
		return nil
	}
	sort.Strings(denied)
	return &CapabilityPolicyError{Denied: denied}
}

// InspectRequiredCapabilities returns sorted component imports suitable for a
// capability decision before startup.
func InspectRequiredCapabilities(filename string) ([]string, error) {
	return InspectRequiredCapabilitiesContext(context.Background(), filename)
}

func InspectRequiredCapabilitiesContext(ctx context.Context, filename string) ([]string, error) {
	return InspectRequiredCapabilitiesWithOptionsContext(ctx, filename, RuntimeOptions{})
}

// InspectRequiredCapabilitiesWithOptions is InspectRequiredCapabilities with
// explicit bridge selection options.
func InspectRequiredCapabilitiesWithOptions(filename string, options RuntimeOptions) ([]string, error) {
	return InspectRequiredCapabilitiesWithOptionsContext(context.Background(), filename, options)
}

func InspectRequiredCapabilitiesWithOptionsContext(ctx context.Context, filename string, options RuntimeOptions) ([]string, error) {
	contract, err := InspectComponentWithOptionsContext(ctx, filename, options)
	if err != nil {
		return nil, err
	}
	return contract.ImportNames(), nil
}

func capabilityMatch(pattern, function string) bool {
	switch {
	case pattern == "", function == "":
		return false
	case pattern == "*":
		return true
	case strings.HasSuffix(pattern, "*"):
		return strings.HasPrefix(function, strings.TrimSuffix(pattern, "*"))
	case strings.Contains(pattern, "#"):
		return function == pattern
	default:
		return strings.HasPrefix(function, pattern+"#")
	}
}
