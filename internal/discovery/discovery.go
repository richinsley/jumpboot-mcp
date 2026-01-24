package discovery

import "fmt"

// ServiceType is the mDNS service type for jumpboot-mcp servers
const ServiceType = "_jumpboot-mcp._tcp"

// ServiceInfo contains information about a discovered jumpboot-mcp service
type ServiceInfo struct {
	InstanceName string // Unique instance name (used as tool prefix)
	Host         string // Hostname or IP address
	Port         int    // Port number
	Note         string // Human-readable description
	Endpoint     string // HTTP endpoint path (e.g., "/mcp")
	TLS          bool   // Whether TLS is enabled
}

// URL returns the full URL for the service
func (s ServiceInfo) URL() string {
	scheme := "http"
	if s.TLS {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s:%d%s", scheme, s.Host, s.Port, s.Endpoint)
}

// ToolPrefix returns the prefix to use for tools from this service
func (s ServiceInfo) ToolPrefix() string {
	return s.InstanceName
}
