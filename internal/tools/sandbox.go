package tools

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// NetworkInterface represents a simulated macOS network interface (e.g., en0, en1).
type NetworkInterface struct {
	DeviceName   string   `json:"device_name"`   // en0, en1
	HardwarePort string   `json:"hardware_port"` // Wi-Fi, Ethernet
	MACAddress   string   `json:"mac_address"`
	IPv4Address  string   `json:"ipv4_address"`
	SubnetMask   string   `json:"subnet_mask"`
	Router       string   `json:"router"` // Default Gateway
	DHCP         bool     `json:"dhcp"`
	DNSServers   []string `json:"dns_servers"`
	Active       bool     `json:"active"`
	LinkUp       bool     `json:"link_up"`
}

// LogEntry records every sandbox command and state modification.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Command   string    `json:"command"`
	Target    string    `json:"target"`
	Output    string    `json:"output"`
	Success   bool      `json:"success"`
}

// SandboxState contains the simulated macOS network environment state.
type SandboxState struct {
	Interfaces       map[string]*NetworkInterface `json:"interfaces"`
	DefaultInterface string                       `json:"default_interface"`
	DNSCacheFlushed  int                          `json:"dns_cache_flushed"`
	LastCacheFlush   time.Time                    `json:"last_cache_flush"`
	InternetUplink   bool                         `json:"internet_uplink"`
	Logs             []LogEntry                   `json:"logs"`
}

// NetworkSandbox provides a safe, isolated simulation of macOS network commands.
type NetworkSandbox struct {
	mu        sync.RWMutex
	statePath string
	state     *SandboxState
}

// NewNetworkSandbox initializes or loads the simulated network environment.
func NewNetworkSandbox(statePath string) (*NetworkSandbox, error) {
	sb := &NetworkSandbox{
		statePath: statePath,
	}

	if err := sb.loadOrCreate(); err != nil {
		return nil, fmt.Errorf("failed to init sandbox: %w", err)
	}

	return sb, nil
}

func (sb *NetworkSandbox) loadOrCreate() error {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.loadOrCreateLocked()
}

func (sb *NetworkSandbox) loadOrCreateLocked() error {
	if _, err := os.Stat(sb.statePath); err == nil {
		data, err := os.ReadFile(sb.statePath)
		if err == nil {
			var state SandboxState
			if json.Unmarshal(data, &state) == nil {
				sb.state = &state
				return nil
			}
		}
	}

	// Default macOS Sandbox Environment
	sb.state = &SandboxState{
		DefaultInterface: "Wi-Fi",
		InternetUplink:   true,
		DNSCacheFlushed:  0,
		Interfaces: map[string]*NetworkInterface{
			"Wi-Fi": {
				DeviceName:   "en0",
				HardwarePort: "Wi-Fi",
				MACAddress:   "a4:83:e7:22:91:0c",
				IPv4Address:  "192.168.1.105",
				SubnetMask:   "255.255.255.0",
				Router:       "192.168.1.1",
				DHCP:         true,
				DNSServers:   []string{"10.255.255.1"}, // Intentionally flawed DNS for troubleshooting scenario
				Active:       true,
				LinkUp:       true,
			},
			"Ethernet": {
				DeviceName:   "en1",
				HardwarePort: "Ethernet",
				MACAddress:   "3c:22:fb:a1:05:44",
				IPv4Address:  "",
				SubnetMask:   "",
				Router:       "",
				DHCP:         true,
				DNSServers:   []string{},
				Active:       false,
				LinkUp:       false,
			},
		},
		Logs: []LogEntry{},
	}

	return sb.saveLocked()
}

func (sb *NetworkSandbox) saveLocked() error {
	if sb.statePath == "" {
		return nil
	}
	dir := filepath.Dir(sb.statePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(sb.state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(sb.statePath, data, 0644)
}

func (sb *NetworkSandbox) logAction(cmd, target, output string, success bool) {
	sb.state.Logs = append(sb.state.Logs, LogEntry{
		Timestamp: time.Now(),
		Command:   cmd,
		Target:    target,
		Output:    output,
		Success:   success,
	})
	_ = sb.saveLocked()
}

// GetDNSServers simulates `networksetup -getdnsservers <service>`
func (sb *NetworkSandbox) GetDNSServers(service string) (string, []string, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if service == "" {
		service = sb.state.DefaultInterface
	}

	iface, ok := sb.state.Interfaces[service]
	if !ok {
		// Try matching by device name (e.g. en0)
		for _, ifc := range sb.state.Interfaces {
			if ifc.DeviceName == service {
				iface = ifc
				ok = true
				break
			}
		}
	}

	if !ok {
		out := fmt.Sprintf("Error: '%s' is not a recognized network service.", service)
		sb.logAction("networksetup -getdnsservers", service, out, false)
		return out, nil, fmt.Errorf("%s", out)
	}

	if len(iface.DNSServers) == 0 {
		out := fmt.Sprintf("There aren't any DNS Servers set on %s.", service)
		sb.logAction("networksetup -getdnsservers", service, out, true)
		return out, []string{}, nil
	}

	out := strings.Join(iface.DNSServers, "\n")
	sb.logAction("networksetup -getdnsservers", service, out, true)
	return out, iface.DNSServers, nil
}

// SetDNSServers simulates `networksetup -setdnsservers <service> <dns1> [dns2...]`
func (sb *NetworkSandbox) SetDNSServers(service string, servers []string) (string, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if service == "" {
		service = sb.state.DefaultInterface
	}

	iface, ok := sb.state.Interfaces[service]
	if !ok {
		for _, ifc := range sb.state.Interfaces {
			if ifc.DeviceName == service {
				iface = ifc
				service = ifc.HardwarePort
				ok = true
				break
			}
		}
	}

	if !ok {
		out := fmt.Sprintf("Error: Service '%s' does not exist in sandbox.", service)
		sb.logAction("networksetup -setdnsservers", service, out, false)
		return out, fmt.Errorf("%s", out)
	}

	// Validate IP syntax
	cleaned := []string{}
	for _, s := range servers {
		s = strings.TrimSpace(s)
		if strings.EqualFold(s, "empty") || strings.EqualFold(s, "clear") {
			continue
		}
		if s != "" {
			if net.ParseIP(s) == nil {
				out := fmt.Sprintf("Invalid IP address format: '%s'", s)
				sb.logAction("networksetup -setdnsservers", service, out, false)
				return out, fmt.Errorf("%s", out)
			}
			cleaned = append(cleaned, s)
		}
	}

	iface.DNSServers = cleaned
	var out string
	if len(cleaned) == 0 {
		out = fmt.Sprintf("[Sandbox] DNS servers cleared on %s (%s). Falling back to DHCP DNS.", service, iface.DeviceName)
	} else {
		out = fmt.Sprintf("[Sandbox] Successfully updated DNS servers for %s (%s) to: %s", service, iface.DeviceName, strings.Join(cleaned, ", "))
	}

	sb.logAction("networksetup -setdnsservers", service, out, true)
	return out, nil
}

// GetInfo simulates `networksetup -getinfo <service>`
func (sb *NetworkSandbox) GetInfo(service string) (string, *NetworkInterface, error) {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	if service == "" {
		service = sb.state.DefaultInterface
	}

	iface, ok := sb.state.Interfaces[service]
	if !ok {
		for _, ifc := range sb.state.Interfaces {
			if ifc.DeviceName == service {
				iface = ifc
				ok = true
				break
			}
		}
	}

	if !ok {
		return "", nil, fmt.Errorf("service '%s' not found", service)
	}

	var sbText strings.Builder
	sbText.WriteString(fmt.Sprintf("Hardware Port: %s\n", iface.HardwarePort))
	sbText.WriteString(fmt.Sprintf("Device: %s\n", iface.DeviceName))
	sbText.WriteString(fmt.Sprintf("Ethernet Address: %s\n", iface.MACAddress))
	if iface.DHCP {
		sbText.WriteString("IP address configuration: DHCP\n")
	} else {
		sbText.WriteString("IP address configuration: Manual\n")
	}
	sbText.WriteString(fmt.Sprintf("IP address: %s\n", iface.IPv4Address))
	sbText.WriteString(fmt.Sprintf("Subnet mask: %s\n", iface.SubnetMask))
	sbText.WriteString(fmt.Sprintf("Router: %s\n", iface.Router))
	if len(iface.DNSServers) > 0 {
		sbText.WriteString(fmt.Sprintf("DNS Servers: %s\n", strings.Join(iface.DNSServers, ", ")))
	} else {
		sbText.WriteString("DNS Servers: (none)\n")
	}

	return sbText.String(), iface, nil
}

// FlushDNSCache simulates `dscacheutil -flushcache` and `killall -HUP mDNSResponder`
func (sb *NetworkSandbox) FlushDNSCache() (string, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	sb.state.DNSCacheFlushed++
	sb.state.LastCacheFlush = time.Now()
	out := fmt.Sprintf("[Sandbox] macOS DNS cache flushed successfully via dscacheutil & mDNSResponder (Total flushes: %d).", sb.state.DNSCacheFlushed)
	sb.logAction("dscacheutil -flushcache", "system", out, true)
	return out, nil
}

// PingResult contains simulated ping metrics.
type PingResult struct {
	Target       string  `json:"target"`
	PacketsSent  int     `json:"packets_sent"`
	PacketsRecv  int     `json:"packets_recv"`
	PacketLossPct float64 `json:"packet_loss_pct"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	Success      bool    `json:"success"`
	Output       string  `json:"output"`
}

// Ping simulates `ping -c <count> <target>` in macOS environment.
func (sb *NetworkSandbox) Ping(target string, count int) (*PingResult, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if count <= 0 {
		count = 3
	}

	iface := sb.state.Interfaces[sb.state.DefaultInterface]
	if iface == nil || !iface.Active || !iface.LinkUp {
		out := fmt.Sprintf("PING %s (%s): 56 data bytes\nping: sendto: Network is down\n--- %s ping statistics ---\n%d packets transmitted, 0 packets received, 100.0%% packet loss", target, target, target, count)
		res := &PingResult{
			Target:        target,
			PacketsSent:   count,
			PacketsRecv:   0,
			PacketLossPct: 100.0,
			AvgLatencyMs:  0,
			Success:       false,
			Output:        out,
		}
		sb.logAction(fmt.Sprintf("ping -c %d", count), target, out, false)
		return res, nil
	}

	// 1. Gateway Ping
	if target == iface.Router || target == "gateway" || target == "192.168.1.1" {
		out := fmt.Sprintf("PING %s (%s): 56 data bytes\n64 bytes from %s: icmp_seq=0 ttl=64 time=1.428 ms\n64 bytes from %s: icmp_seq=1 ttl=64 time=1.215 ms\n64 bytes from %s: icmp_seq=2 ttl=64 time=1.350 ms\n--- %s ping statistics ---\n%d packets transmitted, %d packets received, 0.0%% packet loss\nround-trip min/avg/max/stddev = 1.215/1.331/1.428/0.088 ms", iface.Router, iface.Router, iface.Router, iface.Router, iface.Router, iface.Router, count, count)
		res := &PingResult{
			Target:        iface.Router,
			PacketsSent:   count,
			PacketsRecv:   count,
			PacketLossPct: 0.0,
			AvgLatencyMs:  1.33,
			Success:       true,
			Output:        out,
		}
		sb.logAction(fmt.Sprintf("ping -c %d", count), target, out, true)
		return res, nil
	}

	// 2. Direct External IP Ping (e.g. 8.8.8.8 or 1.1.1.1)
	isIP := net.ParseIP(target) != nil
	if isIP {
		if !sb.state.InternetUplink {
			out := fmt.Sprintf("PING %s (%s): 56 data bytes\nRequest timeout for icmp_seq 0\nRequest timeout for icmp_seq 1\n--- %s ping statistics ---\n%d packets transmitted, 0 packets received, 100.0%% packet loss", target, target, target, count)
			res := &PingResult{
				Target:        target,
				PacketsSent:   count,
				PacketsRecv:   0,
				PacketLossPct: 100.0,
				AvgLatencyMs:  0,
				Success:       false,
				Output:        out,
			}
			sb.logAction(fmt.Sprintf("ping -c %d", count), target, out, false)
			return res, nil
		}

		out := fmt.Sprintf("PING %s (%s): 56 data bytes\n64 bytes from %s: icmp_seq=0 ttl=117 time=14.215 ms\n64 bytes from %s: icmp_seq=1 ttl=117 time=13.890 ms\n64 bytes from %s: icmp_seq=2 ttl=117 time=14.050 ms\n--- %s ping statistics ---\n%d packets transmitted, %d packets received, 0.0%% packet loss\nround-trip min/avg/max/stddev = 13.890/14.051/14.215/0.132 ms", target, target, target, target, target, target, count, count)
		res := &PingResult{
			Target:        target,
			PacketsSent:   count,
			PacketsRecv:   count,
			PacketLossPct: 0.0,
			AvgLatencyMs:  14.05,
			Success:       true,
			Output:        out,
		}
		sb.logAction(fmt.Sprintf("ping -c %d", count), target, out, true)
		return res, nil
	}

	// 3. Domain Name Ping (e.g., google.com) -> Requires working DNS!
	dnsWorking := false
	for _, dns := range iface.DNSServers {
		if dns == "8.8.8.8" || dns == "8.8.4.4" || dns == "1.1.1.1" || dns == "1.0.0.1" || dns == "9.9.9.9" || dns == "192.168.1.1" {
			dnsWorking = true
			break
		}
	}

	if !dnsWorking {
		out := fmt.Sprintf("ping: cannot resolve %s: Unknown host", target)
		res := &PingResult{
			Target:        target,
			PacketsSent:   0,
			PacketsRecv:   0,
			PacketLossPct: 100.0,
			AvgLatencyMs:  0,
			Success:       false,
			Output:        out,
		}
		sb.logAction(fmt.Sprintf("ping -c %d", count), target, out, false)
		return res, nil
	}

	resolvedIP := "142.250.185.206"
	out := fmt.Sprintf("PING %s (%s): 56 data bytes\n64 bytes from %s: icmp_seq=0 ttl=116 time=18.412 ms\n64 bytes from %s: icmp_seq=1 ttl=116 time=17.920 ms\n64 bytes from %s: icmp_seq=2 ttl=116 time=18.110 ms\n--- %s ping statistics ---\n%d packets transmitted, %d packets received, 0.0%% packet loss\nround-trip min/avg/max/stddev = 17.920/18.147/18.412/0.203 ms", target, resolvedIP, resolvedIP, resolvedIP, resolvedIP, target, count, count)
	res := &PingResult{
		Target:        target,
		PacketsSent:   count,
		PacketsRecv:   count,
		PacketLossPct: 0.0,
		AvgLatencyMs:  18.14,
		Success:       true,
		Output:        out,
	}
	sb.logAction(fmt.Sprintf("ping -c %d", count), target, out, true)
	return res, nil
}

// ResetToDefault resets the sandbox environment to default state.
func (sb *NetworkSandbox) ResetToDefault() {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	_ = os.Remove(sb.statePath)
	_ = sb.loadOrCreateLocked()
}
