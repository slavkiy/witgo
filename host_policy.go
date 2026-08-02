package witgo

import (
	"errors"
	"sort"
	"strings"
	"time"
)

// Permissions describes ambient capabilities granted by the host. Grants are
// additive: a plugin receives Public permissions plus its named grant. Deny
// rules always win. The zero value grants nothing.
type Permissions struct {
	All        bool
	System     bool
	Network    bool
	Files      bool
	LoadPlugin bool
	Allow      []string
	Deny       []string
}

// PluginLimits are host-owned execution limits for one plugin. A plugin
// manifest cannot increase them.
type PluginLimits struct {
	Fuel             uint64
	FuelPerCall      uint64
	Timeout          time.Duration
	MemoryLimitBytes int64
	MaxResultBytes   uint64
	InstanceLimit    int64
	ValueLimits      ValueLimits
	FuelRequests     FuelRequestLimits
	FuelPolicy       FuelRequestPolicy
}

// PluginGrant adds permissions and optionally replaces public limits for one
// logical plugin ID.
type PluginGrant struct {
	Permissions Permissions
	Limits      PluginLimits
	// AllowedPluginRoots bounds components loaded through a plugin manifest.
	AllowedPluginRoots []string
}

// HostPolicy is the convenient, secure entry point for configuring plugins.
// Public applies to every plugin; Plugins adds grants by PluginID.
type HostPolicy struct {
	Public       PluginGrant
	Plugins      map[string]PluginGrant
	RuntimeAPI   bool
	SearchPaths  []string
	Observer     RuntimeSecurityObserver
	BridgePath   string
	BridgeSHA256 string
}

// Options resolves immutable runtime options for pluginID. Public and named
// permissions are combined, while non-zero named limits replace public ones.
func (p HostPolicy) Options(pluginID string) (RuntimeOptions, error) {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return RuntimeOptions{}, errors.New("plugin ID is required")
	}
	grant := p.Public
	if named, ok := p.Plugins[pluginID]; ok {
		grant.Permissions = mergePermissions(grant.Permissions, named.Permissions)
		grant.Limits = mergePluginLimits(grant.Limits, named.Limits)
		if len(named.AllowedPluginRoots) != 0 {
			grant.AllowedPluginRoots = append([]string(nil), named.AllowedPluginRoots...)
		}
	}
	permissions := grant.Permissions
	if p.RuntimeAPI {
		permissions.Allow = append(append([]string(nil), permissions.Allow...), RuntimeSystemInterfaceID)
	}
	options := RuntimeOptions{
		PluginID: pluginID, EnableRuntimeAPI: p.RuntimeAPI,
		Fuel: grant.Limits.Fuel, FuelPerCall: grant.Limits.FuelPerCall,
		Timeout: grant.Limits.Timeout, MemoryLimitBytes: grant.Limits.MemoryLimitBytes,
		MaxResultBytes: grant.Limits.MaxResultBytes, InstanceLimit: grant.Limits.InstanceLimit,
		ValueLimits: grant.Limits.ValueLimits, FuelRequestLimits: grant.Limits.FuelRequests,
		FuelRequestPolicy: grant.Limits.FuelPolicy, SecurityObserver: p.Observer,
		BridgePath: p.BridgePath, BridgeSHA256: p.BridgeSHA256,
		Capabilities: permissions.capabilityPolicy(),
		NestedPlugins: NestedPluginOptions{
			Disabled: !permissions.LoadPlugin, SearchPaths: append([]string(nil), p.SearchPaths...),
			AllowedRoots: append([]string(nil), grant.AllowedPluginRoots...),
		},
	}
	if err := validateRuntimeOptions(options); err != nil {
		return RuntimeOptions{}, err
	}
	return options, nil
}

func (p Permissions) capabilityPolicy() CapabilityPolicy {
	allow := append([]string(nil), p.Allow...)
	if p.All {
		allow = append(allow, "*")
	}
	if p.System {
		allow = append(allow, "wasi:clocks/*", "wasi:random/*", "wasi:cli/*")
	}
	if p.Network {
		allow = append(allow, "wasi:sockets/*", "wasi:http/*")
	}
	if p.Files {
		allow = append(allow, "wasi:filesystem/*", "wasi:io/*")
	}
	// An explicit empty allow-list must deny everything. CapabilityPolicy's
	// legacy zero value allows everything, so use a pattern that matches none.
	if len(allow) == 0 {
		allow = []string{"witgo:deny-all/never@0"}
	}
	return CapabilityPolicy{Allow: uniqueStrings(allow), Deny: uniqueStrings(p.Deny)}
}

func mergePermissions(a, b Permissions) Permissions {
	return Permissions{All: a.All || b.All, System: a.System || b.System,
		Network: a.Network || b.Network, Files: a.Files || b.Files,
		LoadPlugin: a.LoadPlugin || b.LoadPlugin,
		Allow:      append(append([]string(nil), a.Allow...), b.Allow...),
		Deny:       append(append([]string(nil), a.Deny...), b.Deny...)}
}

func mergePluginLimits(base, override PluginLimits) PluginLimits {
	if override.Fuel != 0 {
		base.Fuel = override.Fuel
		base.FuelPerCall = 0
	}
	if override.FuelPerCall != 0 {
		base.FuelPerCall = override.FuelPerCall
		base.Fuel = 0
	}
	if override.Timeout != 0 {
		base.Timeout = override.Timeout
	}
	if override.MemoryLimitBytes != 0 {
		base.MemoryLimitBytes = override.MemoryLimitBytes
	}
	if override.MaxResultBytes != 0 {
		base.MaxResultBytes = override.MaxResultBytes
	}
	if override.InstanceLimit != 0 {
		base.InstanceLimit = override.InstanceLimit
	}
	if override.ValueLimits != (ValueLimits{}) {
		base.ValueLimits = override.ValueLimits
	}
	if override.FuelRequests != (FuelRequestLimits{}) {
		base.FuelRequests = override.FuelRequests
	}
	if override.FuelPolicy != nil {
		base.FuelPolicy = override.FuelPolicy
	}
	return base
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
