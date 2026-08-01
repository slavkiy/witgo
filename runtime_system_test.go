//go:build !wasm

package witgo

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

func enabledFuelLimits() FuelRequestLimits {
	return FuelRequestLimits{
		Enabled:              true,
		MaxGrantPerRequest:   100,
		MaxTotalGrantPerCall: 150,
		MaxRequestsPerCall:   2,
		MaxReasonBytes:       32,
		PolicyTimeout:        100 * time.Millisecond,
	}
}

func TestFuelRequestsAreDefaultDeny(t *testing.T) {
	_, reason, err := decideFuelRequest(context.Background(), FixedFuelAllowance{Grant: 10}, FuelRequestLimits{}, FuelRequest{Requested: 10})
	if !errors.Is(err, ErrFuelRequestDisabled) || reason != FuelDeniedDisabled {
		t.Fatalf("result = %q, %v", reason, err)
	}
}

func TestFuelPolicyCannotBypassHardLimits(t *testing.T) {
	limits := enabledFuelLimits()
	request := FuelRequest{Requested: 80, CurrentFuel: 20, TotalGranted: 100}
	decision, reason, err := decideFuelRequest(context.Background(), CallbackFuelPolicy(func(context.Context, FuelRequest) (FuelDecision, error) {
		return FuelDecision{Grant: math.MaxUint64}, nil
	}), limits, request)
	if err != nil || reason != "" || decision.Grant != 50 {
		t.Fatalf("decision = %#v, %q, %v", decision, reason, err)
	}
	request.CurrentFuel = math.MaxUint64
	_, reason, err = decideFuelRequest(context.Background(), FixedFuelAllowance{Grant: 1}, limits, request)
	if !errors.Is(err, ErrFuelRequestLimitReached) || reason != FuelDeniedRequestLimitReached {
		t.Fatalf("overflow = %q, %v", reason, err)
	}
}

func TestFuelRequestRejectsInvalidInputsCancellationAndCount(t *testing.T) {
	limits := enabledFuelLimits()
	tests := []struct {
		name    string
		request FuelRequest
		ctx     context.Context
		want    error
	}{
		{"zero", FuelRequest{}, context.Background(), ErrFuelRequestTooLarge},
		{"too large", FuelRequest{Requested: 101}, context.Background(), ErrFuelRequestTooLarge},
		{"reason", FuelRequest{Requested: 1, Reason: "123456789012345678901234567890123"}, context.Background(), ErrFuelRequestTooLarge},
		{"count", FuelRequest{Requested: 1, RequestCount: 2}, context.Background(), ErrFuelRequestLimitReached},
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	tests = append(tests, struct {
		name    string
		request FuelRequest
		ctx     context.Context
		want    error
	}{"cancelled", FuelRequest{Requested: 1}, cancelled, ErrCallCancelled})
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := decideFuelRequest(test.ctx, FixedFuelAllowance{Grant: 1}, limits, test.request)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestFuelPolicyPanicAndTimeoutBecomeDenial(t *testing.T) {
	limits := enabledFuelLimits()
	_, _, err := decideFuelRequest(context.Background(), CallbackFuelPolicy(func(context.Context, FuelRequest) (FuelDecision, error) {
		panic("secret panic text")
	}), limits, FuelRequest{Requested: 1})
	if !errors.Is(err, ErrFuelRequestDenied) {
		t.Fatalf("panic error = %v", err)
	}
	limits.PolicyTimeout = time.Millisecond
	_, _, err = decideFuelRequest(context.Background(), CallbackFuelPolicy(func(ctx context.Context, _ FuelRequest) (FuelDecision, error) {
		<-ctx.Done()
		return FuelDecision{}, ctx.Err()
	}), limits, FuelRequest{Requested: 1})
	if !errors.Is(err, ErrFuelRequestDenied) {
		t.Fatalf("timeout error = %v", err)
	}
}

func TestRuntimeSystemFuelGrantAndClosing(t *testing.T) {
	b := &componentBridge{system: runtimeSystemConfig{
		enabled: true, pluginID: "plugin-a", initialFuel: 200,
		policy: FixedFuelAllowance{Grant: 25}, limits: enabledFuelLimits(),
	}}
	state := b.newRuntimeFuelCallState(context.Background(), "run")
	message := bridgeMessage{Function: "request-additional-fuel", Args: []any{uint64(50), "bounded work"}, FuelEnabled: true, FuelRemaining: "10"}
	response, event := b.handleRuntimeSystemCall(context.Background(), message, &state)
	if response["fuel_grant"] != uint64(25) || event == nil || event.Granted != 25 || event.PluginID != "plugin-a" {
		t.Fatalf("response=%#v event=%#v", response, event)
	}
	b.closing.Store(true)
	response, event = b.handleRuntimeSystemCall(context.Background(), message, &state)
	if _, exists := response["fuel_grant"]; exists || event.DenialReason != string(FuelDeniedRuntimeClosing) {
		t.Fatalf("closing response=%#v event=%#v", response, event)
	}
}

func TestValueLimitsAndCyclicValue(t *testing.T) {
	if err := validateArguments([]any{"12345"}, ValueLimits{MaxStringBytes: 4}); !errors.Is(err, ErrArgumentTooLarge) {
		t.Fatalf("string limit = %v", err)
	}
	if err := validateArguments([]any{[]int{1, 2}}, ValueLimits{MaxCollectionLen: 1}); !errors.Is(err, ErrArgumentTooLarge) {
		t.Fatalf("collection limit = %v", err)
	}
	if err := validateResults([]any{"12345"}, ValueLimits{MaxResultBytes: 3}); !errors.Is(err, ErrResultTooLarge) {
		t.Fatalf("result limit = %v", err)
	}
	cycle := map[string]any{}
	cycle["self"] = cycle
	if err := validateArguments([]any{cycle}, ValueLimits{}); !errors.Is(err, ErrValueDepthExceeded) {
		t.Fatalf("cycle = %v", err)
	}
}

func TestRuntimeSystemImportCannotBeSpoofed(t *testing.T) {
	_, _, _, err := prepareHostImports([]HostImport{{Interface: RuntimeSystemInterfaceID, Function: "fuel-info", Call: func([]any) (any, error) { return nil, nil }}})
	if err == nil {
		t.Fatal("reserved system import was accepted")
	}
}
