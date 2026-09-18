//go:build windows

package tun

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/zelgray/sing-box-tray/internal/config"
)

// InjectTUN reads the sing-box config at sbConfigPath and strips any inbound
// that is not a tun inbound (so an http/mixed inbound left over from the base
// config isn't run alongside TUN). If the base config already has a tun
// inbound, it is kept as-is; otherwise a default one built from cfg is
// appended. Prepends a process-exclusion route rule so sing-box's own traffic
// is not looped back through TUN, writes the result to a temp file, and
// returns its path.
//
// The injected tun inbound always carries an IPv6 address in addition to the
// IPv4 one: with strict_route enabled, sing-tun installs an unconditional WFP
// block filter on the IPv6 connect layer when the interface has no IPv6
// address, which blackholes ::1 and breaks anything that resolves localhost
// to IPv6 (Node/Vite, Next, etc.). route_address stays IPv4-only so no IPv6
// default route is added, and route_exclude_address keeps loopback, private
// and link-local traffic out of the tunnel.
func InjectTUN(sbConfigPath string, cfg config.TUNConfig, singBoxPath string) (string, error) {
	root, err := config.LoadRawSingBoxConfig(sbConfigPath)
	if err != nil {
		return "", err
	}

	inbounds, err := config.FilterInbounds(root["inbounds"], "tun")
	if err != nil {
		return "", err
	}

	if len(inbounds) == 0 {
		tunInbound := buildTUNInbound(cfg)
		tunRaw, err := json.Marshal(tunInbound)
		if err != nil {
			return "", fmt.Errorf("marshal tun inbound: %w", err)
		}
		inbounds = append(inbounds, json.RawMessage(tunRaw))
	}

	inboundsRaw, err := json.Marshal(inbounds)
	if err != nil {
		return "", fmt.Errorf("marshal inbounds: %w", err)
	}
	root["inbounds"] = json.RawMessage(inboundsRaw)

	// Prepend a route rule that sends sing-box's own process traffic directly,
	// preventing it from looping back through the TUN interface.
	if err := injectSelfBypassRule(root, filepath.Base(singBoxPath)); err != nil {
		return "", err
	}

	return config.WriteRawSingBoxConfig(root)
}

// EnsureWintunDll copies wintun.dll from src to dstDir/wintun.dll if the
// destination does not already exist.
func EnsureWintunDll(src, dstDir string) error {
	if src == "" {
		return nil
	}
	dst := filepath.Join(dstDir, "wintun.dll")
	if _, err := os.Stat(dst); err == nil {
		return nil // already present
	}
	return copyFile(src, dst)
}

// Defaults used when tray-config.json leaves the corresponding tun.* field
// empty. tun.address MUST include an IPv6 address — see InjectTUN for why.
var (
	defaultTUNAddress = []string{"172.19.0.1/30", "fdfe:dcba:9876::1/126"}

	// IPv4-only on purpose: an IPv6 entry here would make auto_route install
	// an IPv6 default route, and direct IPv6 traffic could re-enter the TUN.
	defaultRouteAddress = []string{"0.0.0.0/1", "128.0.0.0/1"}

	// Loopback, RFC1918 and link-local, kept out of the tunnel entirely.
	defaultRouteExcludeAddress = []string{
		"127.0.0.0/8", "::1/128",
		"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "169.254.0.0/16",
		"fc00::/7", "fe80::/10", "ff00::/8",
	}
)

func buildTUNInbound(cfg config.TUNConfig) map[string]any {
	addr := cfg.Address
	if len(addr) == 0 {
		addr = defaultTUNAddress
	}
	routeAddr := cfg.RouteAddress
	if len(routeAddr) == 0 {
		routeAddr = defaultRouteAddress
	}
	routeExclude := cfg.RouteExcludeAddress
	if len(routeExclude) == 0 {
		routeExclude = defaultRouteExcludeAddress
	}
	mtu := cfg.MTU
	if mtu == 0 {
		mtu = 9000
	}
	name := cfg.InterfaceName
	if name == "" {
		name = "singbox-tun"
	}
	return map[string]any{
		"type":                  "tun",
		"tag":                   "tun-in",
		"interface_name":        name,
		"address":               addr,
		"mtu":                   mtu,
		"auto_route":            true,
		"strict_route":          true,
		"route_address":         routeAddr,
		"route_exclude_address": routeExclude,
	}
}

// injectSelfBypassRule patches the route section for TUN mode:
//   - sets auto_detect_interface: true so sing-box knows which physical NIC to
//     use when building the auto_route routing table (without this the default
//     routes that redirect browser traffic into TUN are not set up correctly on
//     Windows)
//   - prepends a process_name rule that sends sing-box's own connections via
//     "direct", breaking the TUN loop for the proxy process itself
func injectSelfBypassRule(root map[string]json.RawMessage, processName string) error {
	var route map[string]json.RawMessage
	if raw, ok := root["route"]; ok {
		if err := json.Unmarshal(raw, &route); err != nil {
			return fmt.Errorf("parse route: %w", err)
		}
	} else {
		route = make(map[string]json.RawMessage)
	}

	// auto_detect_interface is required for auto_route to work on Windows: it
	// tells sing-box which physical interface carries the real default gateway,
	// so it can add exclusion routes and properly redirect all other traffic
	// through TUN. Only inject if the user hasn't set it explicitly.
	if _, ok := route["auto_detect_interface"]; !ok {
		trueVal, _ := json.Marshal(true)
		route["auto_detect_interface"] = trueVal
	}

	var rules []json.RawMessage
	if raw, ok := route["rules"]; ok {
		if err := json.Unmarshal(raw, &rules); err != nil {
			return fmt.Errorf("parse route.rules: %w", err)
		}
	}

	bypassRule := map[string]any{
		"process_name": []string{processName},
		"outbound":     "direct",
	}
	bypassRaw, err := json.Marshal(bypassRule)
	if err != nil {
		return fmt.Errorf("marshal bypass rule: %w", err)
	}
	rules = append([]json.RawMessage{json.RawMessage(bypassRaw)}, rules...)

	rulesRaw, err := json.Marshal(rules)
	if err != nil {
		return fmt.Errorf("marshal route.rules: %w", err)
	}
	route["rules"] = rulesRaw

	routeRaw, err := json.Marshal(route)
	if err != nil {
		return fmt.Errorf("marshal route: %w", err)
	}
	root["route"] = routeRaw
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open wintun.dll source: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create wintun.dll destination: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy wintun.dll: %w", err)
	}
	return nil
}
