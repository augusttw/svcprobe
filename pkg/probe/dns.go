package probe

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

// ProbeDNSExec performs a DNS lookup probe for a host.
func ProbeDNSExec(ctx context.Context, target ParsedTarget, opts Options) Sample {
	sample := Sample{
		Timestamp: time.Now(),
		ProbeType: ProbeDNS,
	}

	resolver := net.DefaultResolver
	if opts.DNSServer != "" {
		dnsAddr := opts.DNSServer
		if !strings.Contains(dnsAddr, ":") {
			dnsAddr = net.JoinHostPort(dnsAddr, "53")
		}
		resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{Timeout: opts.Timeout}
				return d.DialContext(ctx, "udp", dnsAddr)
			},
		}
	}

	start := time.Now()
	ips, err := resolver.LookupIPAddr(ctx, target.Host)
	duration := time.Since(start)
	sample.Duration = duration

	if err != nil {
		sample.Success = false
		sample.Error = fmt.Sprintf("DNS resolution failed for %s: %v", target.Host, err)
		return sample
	}

	if len(ips) == 0 {
		sample.Success = false
		sample.Error = fmt.Sprintf("DNS resolution returned 0 IP addresses for %s", target.Host)
		return sample
	}

	ipStrs := make([]string, 0, len(ips))
	for _, ip := range ips {
		ipStrs = append(ipStrs, ip.String())
	}

	// Optional CNAME lookup
	cname, _ := resolver.LookupCNAME(ctx, target.Host)

	sample.Success = true
	sample.DNS = &DNSResult{
		Host:         target.Host,
		IPs:          ipStrs,
		LookupTime:   duration,
		ResolverUsed: opts.DNSServer,
		CNAME:        strings.TrimSuffix(cname, "."),
	}

	return sample
}
