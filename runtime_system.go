package witgo

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

const RuntimeSystemInterfaceID = "witgo:runtime/runtime@1.0.0"

var (
	ErrFuelRequestDisabled     = errors.New("additional fuel requests are disabled")
	ErrFuelRequestDenied       = errors.New("additional fuel request was denied")
	ErrFuelRequestTooLarge     = errors.New("additional fuel request exceeds a hard limit")
	ErrFuelRequestLimitReached = errors.New("additional fuel request limit reached")
	ErrRuntimeClosing          = errors.New("component runtime is closing")
	ErrArgumentTooLarge        = errors.New("component argument exceeds configured limit")
	ErrValueDepthExceeded      = errors.New("component value nesting exceeds configured limit")
	ErrProviderUnavailable     = errors.New("plugin provider is unavailable")
	ErrProviderClosing         = errors.New("plugin provider is closing")
	ErrCallCancelled           = errors.New("component call was cancelled")
)

type RuntimeCallInfo struct {
	CallID            string         `json:"call-id"`
	ParentCallID      Option[string] `json:"parent-call-id"`
	Depth             uint32         `json:"depth"`
	PluginID          string         `json:"plugin-id"`
	DeadlineUnixNanos Option[int64]  `json:"deadline-unix-nanos"`
}

type RuntimeFuelInfo struct {
	Enabled   bool           `json:"enabled"`
	Remaining Option[uint64] `json:"remaining"`
	Initial   Option[uint64] `json:"initial"`
	Consumed  Option[uint64] `json:"consumed"`
	PerCall   bool           `json:"per-call"`
}

type RuntimeLimits struct {
	MaxCallDepth       uint32         `json:"max-call-depth"`
	RemainingCallDepth uint32         `json:"remaining-call-depth"`
	MemoryLimitBytes   Option[uint64] `json:"memory-limit-bytes"`
	MaxMessageBytes    Option[uint64] `json:"max-message-bytes"`
	DeadlineUnixNanos  Option[int64]  `json:"deadline-unix-nanos"`
}

type FuelDenialReason string

const (
	FuelDeniedDisabled            FuelDenialReason = "disabled"
	FuelDeniedPolicy              FuelDenialReason = "policy-denied"
	FuelDeniedRequestTooLarge     FuelDenialReason = "request-too-large"
	FuelDeniedRequestLimitReached FuelDenialReason = "request-limit-reached"
	FuelDeniedCallFinishing       FuelDenialReason = "call-finishing"
	FuelDeniedRuntimeClosing      FuelDenialReason = "runtime-closing"
	FuelDeniedInvalidReason       FuelDenialReason = "invalid-reason"
)

type FuelGrant struct {
	Requested uint64 `json:"requested"`
	Granted   uint64 `json:"granted"`
	Remaining uint64 `json:"remaining"`
}

type FuelRequest struct {
	CallID       string
	ParentCallID string
	PluginID     string
	ProviderID   string
	Interface    string
	Function     string
	CallDepth    int
	Requested    uint64
	Reason       string
	CurrentFuel  uint64
	InitialFuel  uint64
	TotalGranted uint64
	RequestCount uint32
	Deadline     time.Time
}

type FuelDecision struct{ Grant uint64 }

type FuelRequestPolicy interface {
	DecideFuel(context.Context, FuelRequest) (FuelDecision, error)
}

type DenyFuelRequests struct{}

func (DenyFuelRequests) DecideFuel(context.Context, FuelRequest) (FuelDecision, error) {
	return FuelDecision{}, ErrFuelRequestDenied
}

type FixedFuelAllowance struct{ Grant uint64 }

func (p FixedFuelAllowance) DecideFuel(_ context.Context, request FuelRequest) (FuelDecision, error) {
	grant := p.Grant
	if grant > request.Requested {
		grant = request.Requested
	}
	return FuelDecision{Grant: grant}, nil
}

type CappedFuelAllowance struct {
	MaxGrantPerRequest uint64
	MaxTotalGrant      uint64
	MaxRequestsPerCall uint32
}

func (p CappedFuelAllowance) DecideFuel(_ context.Context, request FuelRequest) (FuelDecision, error) {
	if p.MaxRequestsPerCall > 0 && request.RequestCount >= p.MaxRequestsPerCall {
		return FuelDecision{}, ErrFuelRequestLimitReached
	}
	grant := request.Requested
	if p.MaxGrantPerRequest > 0 && grant > p.MaxGrantPerRequest {
		grant = p.MaxGrantPerRequest
	}
	if p.MaxTotalGrant > 0 {
		if request.TotalGranted >= p.MaxTotalGrant {
			return FuelDecision{}, ErrFuelRequestLimitReached
		}
		remaining := p.MaxTotalGrant - request.TotalGranted
		if grant > remaining {
			grant = remaining
		}
	}
	return FuelDecision{Grant: grant}, nil
}

type PerPluginFuelAllowance map[string]FuelRequestPolicy

func (p PerPluginFuelAllowance) DecideFuel(ctx context.Context, request FuelRequest) (FuelDecision, error) {
	policy := p[request.PluginID]
	if policy == nil {
		return FuelDecision{}, ErrFuelRequestDenied
	}
	return policy.DecideFuel(ctx, request)
}

type CallbackFuelPolicy func(context.Context, FuelRequest) (FuelDecision, error)

func (p CallbackFuelPolicy) DecideFuel(ctx context.Context, request FuelRequest) (FuelDecision, error) {
	if p == nil {
		return FuelDecision{}, ErrFuelRequestDenied
	}
	return p(ctx, request)
}

type FuelRequestLimits struct {
	Enabled              bool
	MaxGrantPerRequest   uint64
	MaxTotalGrantPerCall uint64
	MaxRequestsPerCall   uint32
	MinRemainingTime     time.Duration
	MaxReasonBytes       int
	PolicyTimeout        time.Duration
}

type FuelRequestError struct {
	PluginID  string
	CallID    string
	Requested uint64
	Reason    FuelDenialReason
	Cause     error
}

func (e *FuelRequestError) Error() string {
	if e == nil {
		return ErrFuelRequestDenied.Error()
	}
	return fmt.Sprintf("fuel request denied: plugin=%q call=%q requested=%d reason=%s", e.PluginID, e.CallID, e.Requested, e.Reason)
}
func (e *FuelRequestError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}
func (e *FuelRequestError) Is(target error) bool {
	if e == nil {
		return false
	}
	return target == ErrFuelRequestDenied || errors.Is(e.Cause, target)
}

type RuntimeLimitError struct {
	Limit   string
	Maximum uint64
	Actual  uint64
	Cause   error
}

func (e *RuntimeLimitError) Error() string {
	if e == nil {
		return "runtime limit exceeded"
	}
	return fmt.Sprintf("runtime limit %q exceeded: maximum=%d actual=%d", e.Limit, e.Maximum, e.Actual)
}
func (e *RuntimeLimitError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}
func (e *RuntimeLimitError) Is(target error) bool {
	return e != nil && errors.Is(e.Cause, target)
}

type FuelRequestEvent struct {
	Time            time.Time
	CallID          string
	PluginID        string
	CallPath        []PluginCallFrame
	Requested       uint64
	Granted         uint64
	DenialReason    string
	GuestReason     string
	RemainingBefore uint64
	RemainingAfter  uint64
}

type CycleEvent struct{ CallPath []PluginCallFrame }
type LimitEvent struct {
	PluginID, Limit string
	Maximum, Actual uint64
}
type ProviderEvent struct {
	PluginID, ProviderID, Interface string
	Reason                          string
}

type RuntimeSecurityObserver interface {
	OnFuelRequest(FuelRequestEvent)
	OnCycleRejected(CycleEvent)
	OnLimitExceeded(LimitEvent)
	OnProviderRejected(ProviderEvent)
}

type runtimeFuelCallState struct {
	info         RuntimeCallInfo
	path         []PluginCallFrame
	initial      uint64
	totalGranted uint64
	requestCount uint32
	finishing    bool
}

func decideFuelRequest(ctx context.Context, policy FuelRequestPolicy, limits FuelRequestLimits, request FuelRequest) (FuelDecision, FuelDenialReason, error) {
	if !limits.Enabled {
		return FuelDecision{}, FuelDeniedDisabled, ErrFuelRequestDisabled
	}
	if request.Requested == 0 {
		return FuelDecision{}, FuelDeniedRequestTooLarge, ErrFuelRequestTooLarge
	}
	if limits.MaxReasonBytes >= 0 && len(request.Reason) > limits.MaxReasonBytes {
		return FuelDecision{}, FuelDeniedInvalidReason, ErrFuelRequestTooLarge
	}
	if strings.IndexByte(request.Reason, 0) >= 0 {
		return FuelDecision{}, FuelDeniedInvalidReason, ErrFuelRequestDenied
	}
	if limits.MaxGrantPerRequest == 0 || request.Requested > limits.MaxGrantPerRequest {
		return FuelDecision{}, FuelDeniedRequestTooLarge, ErrFuelRequestTooLarge
	}
	if limits.MaxRequestsPerCall == 0 || request.RequestCount >= limits.MaxRequestsPerCall {
		return FuelDecision{}, FuelDeniedRequestLimitReached, ErrFuelRequestLimitReached
	}
	if limits.MaxTotalGrantPerCall == 0 || request.TotalGranted >= limits.MaxTotalGrantPerCall {
		return FuelDecision{}, FuelDeniedRequestLimitReached, ErrFuelRequestLimitReached
	}
	if request.Deadline.IsZero() == false && time.Until(request.Deadline) < limits.MinRemainingTime {
		return FuelDecision{}, FuelDeniedCallFinishing, ErrFuelRequestDenied
	}
	if err := contextError(ctx); err != nil {
		return FuelDecision{}, FuelDeniedCallFinishing, ErrCallCancelled
	}
	if policy == nil {
		policy = DenyFuelRequests{}
	}
	policyCtx := ctx
	cancel := func() {}
	if limits.PolicyTimeout > 0 {
		policyCtx, cancel = context.WithTimeout(ctx, limits.PolicyTimeout)
	}
	defer cancel()
	type answer struct {
		decision FuelDecision
		err      error
	}
	answers := make(chan answer, 1)
	go func() {
		var result answer
		defer func() {
			if recover() != nil {
				result.err = ErrFuelRequestDenied
			}
			answers <- result
		}()
		result.decision, result.err = policy.DecideFuel(policyCtx, request)
	}()
	var result answer
	select {
	case result = <-answers:
	case <-policyCtx.Done():
		return FuelDecision{}, FuelDeniedPolicy, ErrFuelRequestDenied
	}
	if result.err != nil || result.decision.Grant == 0 {
		return FuelDecision{}, FuelDeniedPolicy, ErrFuelRequestDenied
	}
	grant := result.decision.Grant
	if grant > request.Requested || grant > limits.MaxGrantPerRequest {
		grant = minUint64(request.Requested, limits.MaxGrantPerRequest)
	}
	remainingTotal := limits.MaxTotalGrantPerCall - request.TotalGranted
	if grant > remainingTotal {
		grant = remainingTotal
	}
	if grant == 0 || request.CurrentFuel > math.MaxUint64-grant {
		return FuelDecision{}, FuelDeniedRequestLimitReached, ErrFuelRequestLimitReached
	}
	return FuelDecision{Grant: grant}, "", nil
}

func minUint64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}
