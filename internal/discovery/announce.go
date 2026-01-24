package discovery

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/hashicorp/mdns"
)

// Announcer announces a jumpboot-mcp service via mDNS
type Announcer struct {
	server *mdns.Server
	info   ServiceInfo
}

// NewAnnouncer creates a new mDNS announcer for the given service info
func NewAnnouncer(info ServiceInfo) *Announcer {
	return &Announcer{
		info: info,
	}
}

// Start begins announcing the service via mDNS
func (a *Announcer) Start() error {
	// Build TXT records
	txtRecords := []string{
		fmt.Sprintf("endpoint=%s", a.info.Endpoint),
		fmt.Sprintf("tls=%t", a.info.TLS),
	}
	if a.info.Note != "" {
		txtRecords = append(txtRecords, fmt.Sprintf("note=%s", a.info.Note))
	}

	// Get local IPs
	ips, err := getLocalIPs()
	if err != nil {
		return fmt.Errorf("failed to get local IPs: %w", err)
	}

	// Create the mDNS service
	service, err := mdns.NewMDNSService(
		a.info.InstanceName,
		ServiceType,
		"",              // domain (empty = .local)
		"",              // host (empty = auto)
		a.info.Port,
		ips,
		txtRecords,
	)
	if err != nil {
		return fmt.Errorf("failed to create mDNS service: %w", err)
	}

	// Create and start the server
	server, err := mdns.NewServer(&mdns.Config{Zone: service})
	if err != nil {
		return fmt.Errorf("failed to start mDNS server: %w", err)
	}

	a.server = server
	return nil
}

// Stop stops announcing the service
func (a *Announcer) Stop() error {
	if a.server != nil {
		return a.server.Shutdown()
	}
	return nil
}

// getLocalIPs returns the local IP addresses for mDNS announcement
func getLocalIPs() ([]net.IP, error) {
	var ips []net.IP

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		// Only include physical network interfaces (whitelist approach)
		if !isPhysicalInterface(iface.Name) {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// Only include routable IPv4 addresses
			if ip != nil && ip.To4() != nil && isRoutableIP(ip) {
				ips = append(ips, ip)
			}
		}
	}

	if len(ips) == 0 {
		// Fallback to localhost if no other IPs found
		ips = append(ips, net.ParseIP("127.0.0.1"))
	}

	return ips, nil
}

// isPhysicalInterface returns true if the interface looks like a physical network interface
func isPhysicalInterface(name string) bool {
	// Whitelist of physical interface prefixes
	physicalPrefixes := []string{
		// Linux
		"eth",  // Traditional ethernet
		"eno",  // Onboard ethernet (systemd naming)
		"ens",  // PCI slot ethernet (systemd naming)
		"enp",  // PCI bus ethernet (systemd naming)
		"enx",  // MAC-based ethernet (systemd naming)
		"wlan", // Traditional wifi
		"wlp",  // PCI wifi (systemd naming)
		"wls",  // PCI slot wifi (systemd naming)
		// macOS
		"en", // macOS ethernet/wifi (en0, en1, etc.)
		// BSD
		"bge", "em", "igb", "ix", "re", // Common BSD ethernet drivers
	}

	nameLower := strings.ToLower(name)
	for _, prefix := range physicalPrefixes {
		if strings.HasPrefix(nameLower, prefix) {
			return true
		}
	}
	return false
}

// isRoutableIP returns true if the IP is a normal routable address (not virtual/VPN)
func isRoutableIP(ip net.IP) bool {
	// Only check IPv4 addresses
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}

	// Reject loopback
	if ip4[0] == 127 {
		return false
	}

	// Reject link-local (169.254.x.x)
	if ip4[0] == 169 && ip4[1] == 254 {
		return false
	}

	// Reject CGNAT range (100.64.0.0/10) - used by Tailscale, carrier NAT
	if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
		return false
	}

	// Reject Docker ranges (172.17-31.x.x)
	if ip4[0] == 172 && ip4[1] >= 17 && ip4[1] <= 31 {
		return false
	}

	// Accept common private ranges: 10.x.x.x, 192.168.x.x, 172.16.x.x
	// and public IPs
	return true
}

// GetDefaultInstanceName returns the default instance name (hostname)
func GetDefaultInstanceName() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "jumpboot-mcp"
	}
	// Sanitize hostname for mDNS (remove dots, make lowercase)
	return sanitizeInstanceName(hostname)
}

// sanitizeInstanceName makes a name safe for use as an mDNS instance name
func sanitizeInstanceName(name string) string {
	// Replace dots and spaces with hyphens
	name = strings.ReplaceAll(name, ".", "-")
	name = strings.ReplaceAll(name, " ", "-")
	// Convert to lowercase
	name = strings.ToLower(name)
	// Remove any non-alphanumeric characters except hyphens
	var result strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// ParsePort extracts the port number from an address string like ":8080" or "0.0.0.0:8080"
func ParsePort(addr string) (int, error) {
	// Split on the last colon
	idx := strings.LastIndex(addr, ":")
	if idx == -1 {
		return 0, fmt.Errorf("no port in address: %s", addr)
	}
	portStr := addr[idx+1:]
	return strconv.Atoi(portStr)
}
