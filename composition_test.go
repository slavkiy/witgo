//go:build !wasm

package witgo

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var testCodecDescriptor = InterfaceDescriptor{
	ID: "example:pipeline/image-codec@1.0.0",
	Functions: map[string]string{
		"decode": "(list<u8>)->(result<record{width:u32},string>)",
	},
}

func TestHostRegisterResolveAndAutoBind(t *testing.T) {
	host, err := NewHost()
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	handle, err := host.RegisterProvider("png", testCodecDescriptor, func(_ context.Context, function string, args []any) (any, error) {
		if function != "decode" || len(args) != 1 {
			t.Fatalf("unexpected call %q %#v", function, args)
		}
		return "decoded", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := host.AutoResolveProvider(testCodecDescriptor)
	if err != nil || resolved != handle {
		t.Fatalf("auto resolve = %p, %v; want %p", resolved, err, handle)
	}
	value, err := resolved.CallContext(context.Background(), "processor", "decode", []byte{1, 2})
	if err != nil || value != "decoded" {
		t.Fatalf("call = %#v, %v", value, err)
	}
	if _, err := host.RegisterProvider("jpeg", testCodecDescriptor, func(context.Context, string, []any) (any, error) { return nil, nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := host.AutoResolveProvider(testCodecDescriptor); !errors.Is(err, ErrPluginAlreadyRegistered) {
		t.Fatalf("ambiguous auto-bind = %v", err)
	}
}

func TestProviderPanicIsSanitized(t *testing.T) {
	host, _ := NewHost()
	defer host.Close()
	handle, err := host.RegisterProvider("panic", testCodecDescriptor, func(context.Context, string, []any) (any, error) {
		panic("secret implementation detail")
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = handle.CallContext(context.Background(), "consumer", "decode")
	if err == nil || strings.Contains(err.Error(), "secret implementation detail") {
		t.Fatalf("panic was not sanitized: %v", err)
	}
}

func TestHostRejectsDependencyMismatch(t *testing.T) {
	host, _ := NewHost()
	defer host.Close()
	_, err := host.RegisterProvider("codec", testCodecDescriptor, func(context.Context, string, []any) (any, error) { return nil, nil })
	if err != nil {
		t.Fatal(err)
	}
	expected := cloneDescriptor(testCodecDescriptor)
	expected.Functions["decode"] = "(string)->(string)"
	_, err = host.ResolveProvider("codec", expected)
	var dependencyErr *PluginDependencyError
	if !errors.Is(err, ErrPluginDependencyMismatch) || !errors.As(err, &dependencyErr) {
		t.Fatalf("mismatch error = %T %v", err, err)
	}
}

func TestHostDetectsProviderCycleBeforeCallback(t *testing.T) {
	host, _ := NewHost(HostOptions{MaxCallDepth: 8, RejectCycles: true})
	defer host.Close()
	var handle *ProviderHandle
	handle, _ = host.RegisterProvider("loop", testCodecDescriptor, func(ctx context.Context, _ string, _ []any) (any, error) {
		return handle.CallContext(ctx, "loop", "decode")
	})
	_, err := handle.CallContext(context.Background(), "root", "decode")
	if !errors.Is(err, ErrPluginCallCycle) {
		t.Fatalf("cycle error = %v", err)
	}
}

func TestHostEnforcesCallDepthAndDeadline(t *testing.T) {
	host, _ := NewHost(HostOptions{MaxCallDepth: 1, RejectCycles: true, CallTimeout: 20 * time.Millisecond})
	defer host.Close()
	second, _ := host.RegisterProvider("second", testCodecDescriptor, func(ctx context.Context, _ string, _ []any) (any, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 25*time.Millisecond {
			t.Fatalf("host deadline was not inherited: %v %v", deadline, ok)
		}
		return nil, nil
	})
	firstDescriptor := InterfaceDescriptor{ID: "example:pipeline/first@1.0.0", Functions: map[string]string{"run": "()->()"}}
	first, _ := host.RegisterProvider("first", firstDescriptor, func(ctx context.Context, _ string, _ []any) (any, error) {
		return second.CallContext(ctx, "first", "decode")
	})
	_, err := first.CallContext(context.Background(), "root", "run")
	if !errors.Is(err, ErrPluginCallDepthExceeded) {
		t.Fatalf("depth error = %v", err)
	}
}

func TestUnregisterWaitsAndClosesOnlyOwnedProvider(t *testing.T) {
	host, _ := NewHost()
	entered, release := make(chan struct{}), make(chan struct{})
	var closes atomic.Int32
	handle, err := host.RegisterProvider("codec", testCodecDescriptor, func(context.Context, string, []any) (any, error) {
		close(entered)
		<-release
		return nil, nil
	}, OwnedProvider(func() error { closes.Add(1); return nil }))
	if err != nil {
		t.Fatal(err)
	}
	callDone := make(chan struct{})
	go func() { _, _ = handle.CallContext(context.Background(), "root", "decode"); close(callDone) }()
	<-entered
	unregistered := make(chan error, 1)
	go func() { unregistered <- host.UnregisterProvider(testCodecDescriptor.ID, "codec") }()
	select {
	case <-unregistered:
		t.Fatal("unregister returned before active call completed")
	case <-time.After(10 * time.Millisecond):
	}
	if _, err := handle.CallContext(context.Background(), "root", "decode"); !errors.Is(err, ErrPluginProviderClosed) {
		t.Fatalf("draining provider accepted a new call: %v", err)
	}
	close(release)
	<-callDone
	if err := <-unregistered; err != nil {
		t.Fatal(err)
	}
	if closes.Load() != 1 {
		t.Fatalf("owned close count = %d", closes.Load())
	}
}

func TestProviderRejectsCrossRuntimeHandle(t *testing.T) {
	host, _ := NewHost()
	defer host.Close()
	handle, _ := host.RegisterProvider("codec", testCodecDescriptor, func(context.Context, string, []any) (any, error) {
		return Handle{}, nil
	})
	if _, err := handle.CallContext(context.Background(), "root", "decode"); !errors.Is(err, ErrCrossRuntimeHandle) {
		t.Fatalf("handle result error = %v", err)
	}
}

func TestProviderExposesSameStoreCompositionEdge(t *testing.T) {
	host, _ := NewHost()
	defer host.Close()
	composition := ComponentComposition{
		Component: "provider.wasm",
		Dependencies: []CompositionPlug{{
			Interface: "example:storage/api@1.0.0",
			Component: "storage.wasm",
		}},
	}
	handle, err := host.RegisterProvider("codec", testCodecDescriptor, func(context.Context, string, []any) (any, error) {
		return nil, nil
	}, ComponentProvider(composition))
	if err != nil {
		t.Fatal(err)
	}
	plug, ok := handle.CompositionPlug()
	if !ok || plug.Interface != testCodecDescriptor.ID || plug.Component != "provider.wasm" || len(plug.Dependencies) != 1 {
		t.Fatalf("wrong composition edge: %#v, %v", plug, ok)
	}
	composition.Dependencies[0].Component = "changed.wasm"
	if plug.Dependencies[0].Component != "storage.wasm" {
		t.Fatal("provider composition metadata aliases caller-owned memory")
	}
}

func TestDescriptorDetectsRuntimeHandles(t *testing.T) {
	for _, signature := range []string{"()->(own)", "(borrow)->()", "()->(future<string>)", "(stream<u8>)->()", "()->(error-context)"} {
		if !DescriptorUsesRuntimeHandles(InterfaceDescriptor{ID: "test:handles/api@1.0.0", Functions: map[string]string{"run": signature}}) {
			t.Fatalf("runtime handle signature was not detected: %s", signature)
		}
	}
	if DescriptorUsesRuntimeHandles(testCodecDescriptor) {
		t.Fatal("ordinary value descriptor was classified as runtime-bound")
	}
}

func TestPluginCallErrorPreservesCauseAndPath(t *testing.T) {
	host, _ := NewHost()
	defer host.Close()
	cause := errors.New("provider trap")
	handle, _ := host.RegisterProvider("codec", testCodecDescriptor, func(context.Context, string, []any) (any, error) {
		return nil, cause
	})
	_, err := handle.CallContext(context.Background(), "processor", "decode")
	var callErr *PluginCallError
	if !errors.Is(err, cause) || !errors.As(err, &callErr) {
		t.Fatalf("call error = %T %v", err, err)
	}
	if len(callErr.Path) != 1 || callErr.Path[0].Plugin != "codec" {
		t.Fatalf("call path = %#v", callErr.Path)
	}
}

type panicObserver struct{}

func (panicObserver) OnCallStart(PluginCallEvent)  { panic("start") }
func (panicObserver) OnCallFinish(PluginCallEvent) { panic("finish") }

func TestObserverPanicDoesNotBreakProvider(t *testing.T) {
	host, _ := NewHost(HostOptions{MaxCallDepth: 8, RejectCycles: true, Observer: panicObserver{}})
	defer host.Close()
	handle, _ := host.RegisterProvider("codec", testCodecDescriptor, func(context.Context, string, []any) (any, error) {
		return "ok", nil
	})
	value, err := handle.CallContext(context.Background(), "processor", "decode")
	if err != nil || value != "ok" {
		t.Fatalf("call after observer panic = %#v, %v", value, err)
	}
}

func TestRuntimeCallStackDetectsReentryBeforeLock(t *testing.T) {
	host, _ := NewHost(HostOptions{MaxCallDepth: 8, RejectCycles: true})
	ctx, cancel, err := host.enterRuntimeCall(context.Background(), "plugin-a", "process")
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	_, _, err = host.enterRuntimeCall(ctx, "plugin-a", "callback")
	if !errors.Is(err, ErrPluginCallCycle) {
		t.Fatalf("runtime reentry error = %v", err)
	}
}
