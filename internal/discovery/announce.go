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

		// Skip virtual/container network interfaces
		if isVirtualInterface(iface.Name) {
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

			// Skip loopback, link-local, and virtual network addresses
			if ip != nil && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !isVirtualIP(ip) {
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

// isVirtualInterface returns true if the interface name indicates a virtual/container network
func isVirtualInterface(name string) bool {
	// Common virtual interface prefixes
	virtualPrefixes := []string{
		"docker",    // Docker bridge
		"br-",       // Docker/Linux bridges
		"veth",      // Virtual ethernet (containers)
		"virbr",     // libvirt/KVM bridges
		"vboxnet",   // VirtualBox
		"vmnet",     // VMware
		"vnic",      // Virtual NIC
		"tap",       // TAP devices
		"tun",       // TUN devices
		"flannel",   // Kubernetes flannel
		"cni",       // Container Network Interface
		"calico",    // Kubernetes calico
		"weave",     // Kubernetes weave
		"podman",    // Podman
		"lxc",       // LXC containers
		"lxd",       // LXD containers
	}

	nameLower := strings.ToLower(name)
	for _, prefix := range virtualPrefixes {
		if strings.HasPrefix(nameLower, prefix) {
			return true
		}
	}
	return false
}

// isVirtualIP returns true if the IP is in a common virtual/container network range
func isVirtualIP(ip net.IP) bool {
	// Only check IPv4 addresses
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}

	// Docker default bridge: 172.17.0.0/16
	if ip4[0] == 172 && ip4[1] == 17 {
		return true
	}

	// Docker user-defined bridges: 172.18.0.0/16 - 172.31.0.0/16
	if ip4[0] == 172 && ip4[1] >= 18 && ip4[1] <= 31 {
		return true
	}

	return false
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
