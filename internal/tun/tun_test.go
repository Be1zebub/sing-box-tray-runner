//go:build windows

package tun

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/zelgray/sing-box-tray/internal/config"
)

// writeFixture writes a minimal sing-box config whose only inbound is not a
// tun, so InjectTUN has to append the default tun inbound.
func writeFixture(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.json")
	src := `{
  "inbounds": [{"type": "mixed", "tag": "mixed-in", "listen": "127.0.0.1", "listen_port": 2080}],
  "outbounds": [{"type": "direct", "tag": "direct"}]
}`
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return p
}

func isIPv6(cidr string) bool { return strings.Contains(cidr, ":") }

// TestBuildTUNInbound pins the defaults and the tray-config.json overrides.
func TestBuildTUNInbound(t *testing.T) {
	cases := []struct {
		name             string
		cfg              config.TUNConfig
		wantInterface    string
		wantMTU          int
		wantAddress      []string
		wantRouteAddr    []string
		wantRouteExclude []string
	}{
		{
			name:             "defaults",
			cfg:              config.TUNConfig{},
			wantInterface:    "singbox-tun",
			wantMTU:          9000,
			wantAddress:      defaultTUNAddress,
			wantRouteAddr:    defaultRouteAddress,
			wantRouteExclude: defaultRouteExcludeAddress,
		},
		{
			name: "overrides",
			cfg: config.TUNConfig{
				InterfaceName:       "my-tun",
				MTU:                 1400,
				Address:             []string{"10.9.0.1/30"},
				RouteAddress:        []string{"0.0.0.0/2", "64.0.0.0/2", "128.0.0.0/2", "192.0.0.0/2"},
				RouteExcludeAddress: []string{"127.0.0.0/8"},
			},
			wantInterface:    "my-tun",
			wantMTU:          1400,
			wantAddress:      []string{"10.9.0.1/30"},
			wantRouteAddr:    []string{"0.0.0.0/2", "64.0.0.0/2", "128.0.0.0/2", "192.0.0.0/2"},
			wantRouteExclude: []string{"127.0.0.0/8"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildTUNInbound(tc.cfg)

			if got["interface_name"] != tc.wantInterface {
				t.Errorf("interface_name = %v, want %q", got["interface_name"], tc.wantInterface)
			}
			if got["mtu"] != tc.wantMTU {
				t.Errorf("mtu = %v, want %d", got["mtu"], tc.wantMTU)
			}
			assertStrings(t, "address", got["address"], tc.wantAddress)
			assertStrings(t, "route_address", got["route_address"], tc.wantRouteAddr)
			assertStrings(t, "route_exclude_address", got["route_exclude_address"], tc.wantRouteExclude)

			// strict_route must stay on for its DNS-leak protection, and
			// auto_route for the routing table itself.
			if got["strict_route"] != true || got["auto_route"] != true {
				t.Errorf("strict_route/auto_route must both be true, got %v/%v",
					got["strict_route"], got["auto_route"])
			}
		})
	}
}

// TestInjectTUNDefaults runs the full config rewrite and checks the JSON that
// actually lands on disk.
func TestInjectTUNDefaults(t *testing.T) {
	out, err := InjectTUN(writeFixture(t), config.TUNConfig{}, "sing-box.exe")
	if err != nil {
		t.Fatalf("InjectTUN: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(out) })

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read injected config: %v", err)
	}

	var got struct {
		Inbounds []struct {
			Type                string   `json:"type"`
			Address             []string `json:"address"`
			RouteAddress        []string `json:"route_address"`
			RouteExcludeAddress []string `json:"route_exclude_address"`
		} `json:"inbounds"`
		Route struct {
			AutoDetectInterface bool             `json:"auto_detect_interface"`
			Rules               []map[string]any `json:"rules"`
		} `json:"route"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("parse injected config: %v", err)
	}

	if len(got.Inbounds) != 1 || got.Inbounds[0].Type != "tun" {
		t.Fatalf("want exactly one tun inbound, got %+v", got.Inbounds)
	}
	in := got.Inbounds[0]

	// An IPv6 address on the interface is what stops strict_route from
	// installing an unconditional IPv6 block filter that kills ::1.
	if !slices.ContainsFunc(in.Address, isIPv6) {
		t.Errorf("address must include an IPv6 CIDR, got %v", in.Address)
	}
	if len(in.RouteAddress) == 0 || slices.ContainsFunc(in.RouteAddress, isIPv6) {
		t.Errorf("route_address must be non-empty and IPv4-only, got %v", in.RouteAddress)
	}
	for _, want := range []string{"::1/128", "10.0.0.0/8", "192.168.0.0/16"} {
		if !slices.Contains(in.RouteExcludeAddress, want) {
			t.Errorf("route_exclude_address missing %q: %v", want, in.RouteExcludeAddress)
		}
	}

	if !got.Route.AutoDetectInterface {
		t.Error("route.auto_detect_interface must be injected for auto_route to work")
	}

	// Rule order: private/loopback first, then the process bypass. Both must
	// use the explicit action: "route" form.
	if len(got.Route.Rules) < 2 {
		t.Fatalf("want at least the two injected rules, got %v", got.Route.Rules)
	}
	private := got.Route.Rules[0]
	if private["ip_is_private"] != true {
		t.Errorf(`rules[0] must be the ip_is_private rule, got %v`, private)
	}
	assertRouteDirect(t, "rules[0]", private)

	process := got.Route.Rules[1]
	names, ok := process["process_name"].([]any)
	if !ok || len(names) != 1 || names[0] != "sing-box.exe" {
		t.Errorf(`rules[1] must target process_name ["sing-box.exe"], got %v`, process["process_name"])
	}
	assertRouteDirect(t, "rules[1]", process)
}

// assertRouteDirect checks a rule is the modern action/outbound direct form.
func assertRouteDirect(t *testing.T, where string, rule map[string]any) {
	t.Helper()
	if rule["action"] != "route" {
		t.Errorf(`%s action = %v, want "route"`, where, rule["action"])
	}
	if rule["outbound"] != "direct" {
		t.Errorf(`%s outbound = %v, want "direct"`, where, rule["outbound"])
	}
}

func assertStrings(t *testing.T, field string, got any, want []string) {
	t.Helper()
	gotSlice, ok := got.([]string)
	if !ok {
		t.Fatalf("%s is %T, want []string", field, got)
	}
	if !slices.Equal(gotSlice, want) {
		t.Errorf("%s = %v, want %v", field, gotSlice, want)
	}
}
