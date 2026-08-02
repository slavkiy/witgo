package witgo

import (
	"testing"
	"time"
)

func TestHostPolicyPublicAndPluginGrants(t *testing.T) {
	policy := HostPolicy{
		Public: PluginGrant{
			Permissions: Permissions{System: true, Allow: []string{"app:host/api@1"}},
			Limits:      PluginLimits{FuelPerCall: 100, Timeout: time.Second},
		},
		Plugins: map[string]PluginGrant{
			"downloader": {Permissions: Permissions{Network: true, LoadPlugin: true}, Limits: PluginLimits{FuelPerCall: 200}},
		},
	}
	options, err := policy.Options("downloader")
	if err != nil {
		t.Fatal(err)
	}
	if options.PluginID != "downloader" || options.FuelPerCall != 200 || options.Timeout != time.Second {
		t.Fatalf("unexpected resolved options: %#v", options)
	}
	for _, capability := range []string{
		"wasi:clocks/monotonic-clock@0.2.0#now",
		"wasi:sockets/tcp@0.2.0#start-connect",
		"app:host/api@1#call",
	} {
		if !options.Capabilities.Allows(capability) {
			t.Errorf("expected %q to be allowed", capability)
		}
	}
	if options.Capabilities.Allows("wasi:filesystem/types@0.2.0#open-at") {
		t.Fatal("filesystem must remain denied")
	}
	if options.NestedPlugins.Disabled {
		t.Fatal("named plugin grant must permit nested plugin loading")
	}
}

func TestHostPolicySecureDefaults(t *testing.T) {
	options, err := (HostPolicy{}).Options("plain")
	if err != nil {
		t.Fatal(err)
	}
	if options.Capabilities.Allows("anything:anything/x@1#run") {
		t.Fatal("zero policy must deny ambient capabilities")
	}
	if !options.NestedPlugins.Disabled {
		t.Fatal("zero policy must disable nested plugin loading")
	}
}

func TestHostPolicyDenyWins(t *testing.T) {
	policy := HostPolicy{Public: PluginGrant{Permissions: Permissions{All: true, Deny: []string{"wasi:sockets/*"}}}}
	options, err := policy.Options("plugin")
	if err != nil {
		t.Fatal(err)
	}
	if options.Capabilities.Allows("wasi:sockets/tcp@0.2.0#connect") {
		t.Fatal("deny must override all")
	}
}
