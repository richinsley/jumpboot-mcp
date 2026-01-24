package discovery

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/mdns"
)

// Discover searches for jumpboot-mcp services on the local network
func Discover(ctx context.Context, timeout time.Duration) ([]ServiceInfo, error) {
	// Channel to receive discovered services
	entriesCh := make(chan *mdns.ServiceEntry, 10)

	// Collect results
	var services []ServiceInfo
	var mu sync.Mutex

	// Use sync.Once to ensure channel is only closed once
	var closeOnce sync.Once
	safeClose := func() {
		closeOnce.Do(func() {
			close(entriesCh)
		})
	}

	// Start a goroutine to collect entries
	done := make(chan struct{})
	go func() {
		defer close(done)
		for entry := range entriesCh {
			info := parseServiceEntry(entry)
			if info != nil {
				mu.Lock()
				services = append(services, *info)
				mu.Unlock()
			}
		}
	}()

	// Create query parameters
	params := mdns.DefaultParams(ServiceType)
	params.Entries = entriesCh
	params.Timeout = timeout
	// Disable IPv6 to avoid issues on systems without proper IPv6 support
	params.DisableIPv6 = true

	// Handle context cancellation
	go func() {
		select {
		case <-ctx.Done():
			safeClose()
		case <-done:
			// Collection finished, nothing to do
		}
	}()

	// Perform the lookup - this closes the channel when done
	err := mdns.Query(params)

	// If query failed, close the channel so collector goroutine can exit
	if err != nil {
		safeClose()
	}

	// Wait for collection to complete
	<-done

	// Return services even if there was an error (we may have partial results)
	// Only return error if we got no services
	if err != nil && len(services) == 0 {
		return nil, err
	}

	return services, nil
}

// parseServiceEntry converts an mDNS entry to a ServiceInfo
func parseServiceEntry(entry *mdns.ServiceEntry) *ServiceInfo {
	if entry == nil {
		return nil
	}

	info := &ServiceInfo{
		InstanceName: sanitizeInstanceName(entry.Name),
		Port:         entry.Port,
		Endpoint:     "/mcp", // Default endpoint
	}

	// Determine host - prefer IPv4
	if entry.AddrV4 != nil {
		info.Host = entry.AddrV4.String()
	} else if entry.AddrV6 != nil {
		info.Host = entry.AddrV6.String()
	} else if entry.Host != "" {
		info.Host = strings.TrimSuffix(entry.Host, ".")
	} else {
		return nil // No host available
	}

	// Parse TXT records
	for _, txt := range entry.InfoFields {
		if val, ok := strings.CutPrefix(txt, "note="); ok {
			info.Note = val
		} else if val, ok := strings.CutPrefix(txt, "endpoint="); ok {
			info.Endpoint = val
		} else if val, ok := strings.CutPrefix(txt, "tls="); ok {
			info.TLS = val == "true"
		}
	}

	// Extract instance name from the full service name
	// Format is typically "instance._service._proto.local."
	if entry.Name != "" {
		parts := strings.Split(entry.Name, ".")
		if len(parts) > 0 && parts[0] != "" {
			info.InstanceName = sanitizeInstanceName(parts[0])
		}
	}

	return info
}
