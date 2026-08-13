package probe

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// ParsedTarget represents a structured representation of an endpoint input.
type ParsedTarget struct {
	Original string    `json:"original"`
	Host     string    `json:"host"`
	Port     int       `json:"port"`
	Address  string    `json:"address"` // host:port
	URL      string    `json:"url,omitempty"`
	Scheme   string    `json:"scheme,omitempty"`
	Path     string    `json:"path,omitempty"`
	Type     ProbeType `json:"type"`
}

// ParseTarget parses a raw string into a ParsedTarget.
func ParseTarget(raw string, forcedType ProbeType) (ParsedTarget, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ParsedTarget{}, fmt.Errorf("target string cannot be empty")
	}

	pt := ParsedTarget{
		Original: raw,
		Type:     forcedType,
	}

	// Check if raw contains scheme (http://, https://, tcp://, tls://, dns://)
	var rawURL string
	if strings.Contains(raw, "://") {
		rawURL = raw
	} else {
		// Default scheme handling based on forcedType or heuristic
		switch forcedType {
		case ProbeHTTP:
			if strings.HasSuffix(raw, ":443") {
				rawURL = "https://" + raw
			} else {
				rawURL = "http://" + raw
			}
		case ProbeTLS:
			rawURL = "tls://" + raw
		case ProbeTCP:
			rawURL = "tcp://" + raw
		case ProbeDNS:
			rawURL = "dns://" + raw
		default:
			if strings.HasSuffix(raw, ":443") || strings.HasSuffix(raw, ":8443") {
				rawURL = "https://" + raw
			} else if strings.HasSuffix(raw, ":80") || strings.HasSuffix(raw, ":8080") {
				rawURL = "http://" + raw
			} else {
				// General fallback
				rawURL = "https://" + raw
			}
		}
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return ParsedTarget{}, fmt.Errorf("invalid endpoint URL format: %w", err)
	}

	scheme := strings.ToLower(u.Scheme)
	pt.Scheme = scheme
	pt.Path = u.Path
	if pt.Path == "" {
		pt.Path = "/"
	}

	// Extract host and port
	hostStr := u.Hostname()
	portStr := u.Port()

	if hostStr == "" {
		// Fallback for custom schemes like tcp://host:port or raw input without host in url.Parse
		hostStr = u.Host
		if strings.Contains(hostStr, ":") {
			h, p, err := net.SplitHostPort(hostStr)
			if err == nil {
				hostStr = h
				portStr = p
			}
		}
	}

	pt.Host = hostStr

	var port int
	if portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err == nil {
			port = p
		}
	}

	if port == 0 {
		switch scheme {
		case "http":
			port = 80
		case "https", "tls":
			port = 443
		case "dns":
			port = 53
		default:
			port = 80
		}
	}
	pt.Port = port
	pt.Address = net.JoinHostPort(pt.Host, strconv.Itoa(pt.Port))

	// Reconstruct URL for HTTP/HTTPS probes
	if scheme == "http" || scheme == "https" {
		pt.URL = u.String()
		if pt.Type == ProbeCheck || pt.Type == "" {
			pt.Type = ProbeHTTP
		}
	} else {
		// For tcp, tls, dns schemes
		switch scheme {
		case "tcp":
			if pt.Type == ProbeCheck || pt.Type == "" {
				pt.Type = ProbeTCP
			}
		case "tls":
			if pt.Type == ProbeCheck || pt.Type == "" {
				pt.Type = ProbeTLS
			}
		case "dns":
			if pt.Type == ProbeCheck || pt.Type == "" {
				pt.Type = ProbeDNS
			}
		}
		pt.URL = fmt.Sprintf("%s://%s", scheme, pt.Address)
	}

	if forcedType != "" && forcedType != ProbeCheck {
		pt.Type = forcedType
	}

	return pt, nil
}
