package discovery

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

const mdnsAddr = "224.0.0.251:5353"

// Discover searches for jumpboot-mcp services on the local network
// It uses the packet source address as the service host, which is more reliable
// than relying on announced A records that might include virtual interfaces.
func Discover(ctx context.Context, timeout time.Duration) ([]ServiceInfo, error) {
	// Map to collect services by instance name (to deduplicate)
	services := make(map[string]*ServiceInfo)
	var mu sync.Mutex

	// Create UDP connection for multicast
	addr, err := net.ResolveUDPAddr("udp4", mdnsAddr)
	if err != nil {
		return nil, err
	}

	// Listen on all interfaces
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Build the mDNS query
	msg := new(dns.Msg)
	msg.SetQuestion(ServiceType+".local.", dns.TypePTR)
	msg.RecursionDesired = false

	// Pack the message
	buf, err := msg.Pack()
	if err != nil {
		return nil, err
	}

	// Send the query
	_, err = conn.WriteToUDP(buf, addr)
	if err != nil {
		return nil, err
	}

	// Set read deadline
	deadline := time.Now().Add(timeout)
	conn.SetReadDeadline(deadline)

	// Receive responses
	recvBuf := make([]byte, 65536)
	for {
		select {
		case <-ctx.Done():
			return mapToSlice(services), nil
		default:
		}

		n, src, err := conn.ReadFromUDP(recvBuf)
		if err != nil {
			// Timeout is expected
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break
			}
			continue
		}

		// Parse the response
		resp := new(dns.Msg)
		if err := resp.Unpack(recvBuf[:n]); err != nil {
			continue
		}

		// Extract service info from the response, using src.IP as the host
		info := parseResponse(resp, src.IP)
		if info != nil {
			mu.Lock()
			// Use instance name as key to deduplicate
			if existing, ok := services[info.InstanceName]; ok {
				// Merge: keep existing but update if we got more info
				if existing.Note == "" && info.Note != "" {
					existing.Note = info.Note
				}
			} else {
				services[info.InstanceName] = info
			}
			mu.Unlock()
		}
	}

	return mapToSlice(services), nil
}

// parseResponse extracts service info from an mDNS response
// The sourceIP is the actual IP address the packet came from
func parseResponse(msg *dns.Msg, sourceIP net.IP) *ServiceInfo {
	var info *ServiceInfo
	var instanceName string
	var port int
	var txtRecords []string

	// Look through all answers
	allRecords := append(msg.Answer, msg.Extra...)

	for _, rr := range allRecords {
		switch r := rr.(type) {
		case *dns.PTR:
			// PTR record gives us the instance name
			if strings.Contains(r.Hdr.Name, ServiceType) {
				instanceName = extractInstanceName(r.Ptr)
			}
		case *dns.SRV:
			// SRV record gives us the port
			port = int(r.Port)
			if instanceName == "" {
				instanceName = extractInstanceName(r.Hdr.Name)
			}
		case *dns.TXT:
			// TXT records give us metadata
			txtRecords = append(txtRecords, r.Txt...)
		}
	}

	// We need at least an instance name to be useful
	if instanceName == "" {
		return nil
	}

	info = &ServiceInfo{
		InstanceName: instanceName,
		Host:         sourceIP.String(), // Use the packet source IP!
		Port:         port,
		Endpoint:     "/mcp", // Default
	}

	// Parse TXT records
	for _, txt := range txtRecords {
		if val, ok := strings.CutPrefix(txt, "note="); ok {
			info.Note = val
		} else if val, ok := strings.CutPrefix(txt, "endpoint="); ok {
			info.Endpoint = val
		} else if val, ok := strings.CutPrefix(txt, "tls="); ok {
			info.TLS = val == "true"
		}
	}

	return info
}

// extractInstanceName pulls the instance name from a full service name
// e.g., "myserver._jumpboot-mcp._tcp.local." -> "myserver"
func extractInstanceName(fullName string) string {
	// Remove trailing dot
	name := strings.TrimSuffix(fullName, ".")

	// Split and take the first part (before the service type)
	parts := strings.Split(name, ".")
	if len(parts) > 0 {
		return sanitizeInstanceName(parts[0])
	}
	return ""
}

// mapToSlice converts the services map to a slice
func mapToSlice(m map[string]*ServiceInfo) []ServiceInfo {
	result := make([]ServiceInfo, 0, len(m))
	for _, info := range m {
		if info.Port > 0 && info.Host != "" {
			result = append(result, *info)
		}
	}
	return result
}
