package contract

import (
	"context"
	"errors"
	"testing"

	"github.com/slavkiy/witgo"
)

type localHostProvider struct{ prefix string }

func (provider localHostProvider) ProcessString(_ context.Context, value string) (string, error) {
	return provider.prefix + value, nil
}

var _ Host = localHostProvider{}

func TestGeneratedProviderRegistrationAndAutoBind(t *testing.T) {
	router, err := witgo.NewHost()
	if err != nil {
		t.Fatal(err)
	}
	defer router.Close()
	if err := RegisterHost(router, "local", localHostProvider{prefix: "HOST:"}); err != nil {
		t.Fatal(err)
	}
	provider, err := ResolveHost(router, "local")
	if err != nil {
		t.Fatal(err)
	}
	value, err := provider.ProcessString(context.Background(), "value")
	if err != nil || value != "HOST:value" {
		t.Fatalf("provider returned %q, %v", value, err)
	}
	bindings, err := AutoBindPlugin(router)
	if err != nil {
		t.Fatal(err)
	}
	if bindings.Host == nil {
		t.Fatal("AutoBindPlugin returned nil Host provider")
	}
	if err := router.UnregisterProvider(HostDescriptor.ID, "local"); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.ProcessString(context.Background(), "again"); !errors.Is(err, witgo.ErrPluginProviderClosed) {
		t.Fatalf("stale generated provider error = %v", err)
	}
}
