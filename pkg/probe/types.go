package probe

import (
	"time"

	"svcprobe/pkg/stats"
)

// ProbeType specifies the network diagnostic probe kind.
type ProbeType string

const (
	ProbeCheck ProbeType = "check"
	ProbeDNS   ProbeType = "dns"
	ProbeTCP   ProbeType = "tcp"
	ProbeTLS   ProbeType = "tls"
	ProbeHTTP  ProbeType = "http"
)

// IssueSeverity indicates diagnostic severity level.
type IssueSeverity string

const (
	SeverityOK      IssueSeverity = "OK"
	SeverityWarning IssueSeverity = "WARN"
	SeverityError   IssueSeverity = "ERROR"
)

// DiagnosticIssue describes a specific problem found during probing.
type DiagnosticIssue struct {
	Code     string        `json:"code"`
	Severity IssueSeverity `json:"severity"`
	Message  string        `json:"message"`
	Details  string        `json:"details,omitempty"`
}

// LatencyBreakdown decomposes execution timing per phase.
type LatencyBreakdown struct {
	DNSLookup    time.Duration `json:"dns_lookup_ns"`
	TCPConnect   time.Duration `json:"tcp_connect_ns"`
	TLSHandshake time.Duration `json:"tls_handshake_ns"`
	TTFB         time.Duration `json:"ttfb_ns"`
	Transfer     time.Duration `json:"transfer_ns"`
	Total        time.Duration `json:"total_ns"`
}

// TLSCertInfo contains certificate and TLS protocol details.
type TLSCertInfo struct {
	Subject           string    `json:"subject"`
	Issuer            string    `json:"issuer"`
	NotBefore         time.Time `json:"not_before"`
	NotAfter          time.Time `json:"not_after"`
	DaysRemaining     int       `json:"days_remaining"`
	IsExpired         bool      `json:"is_expired"`
	DNSNames          []string  `json:"dns_names,omitempty"`
	CipherSuite       string    `json:"cipher_suite"`
	Version           string    `json:"version"`
	VerificationError string    `json:"verification_error,omitempty"`
}

// DNSResult contains hostname resolution details.
type DNSResult struct {
	Host         string        `json:"host"`
	IPs          []string      `json:"ips"`
	LookupTime   time.Duration `json:"lookup_time_ns"`
	ResolverUsed string        `json:"resolver_used,omitempty"`
	CNAME        string        `json:"cname,omitempty"`
}

// TCPResult contains TCP connection establishment details.
type TCPResult struct {
	Address     string        `json:"address"`
	ConnectTime time.Duration `json:"connect_time_ns"`
}

// HTTPResult contains HTTP response details.
type HTTPResult struct {
	URL           string            `json:"url"`
	StatusCode    int               `json:"status_code"`
	StatusText    string            `json:"status_text"`
	Proto         string            `json:"proto"`
	ContentLength int64             `json:"content_length"`
	Breakdown     LatencyBreakdown  `json:"breakdown"`
	Headers       map[string]string `json:"headers,omitempty"`
}

// Sample represents a single probe run result.
type Sample struct {
	Timestamp time.Time        `json:"timestamp"`
	ProbeType ProbeType        `json:"probe_type"`
	Success   bool             `json:"success"`
	Duration  time.Duration    `json:"duration_ns"`
	Error     string           `json:"error,omitempty"`
	Breakdown LatencyBreakdown `json:"breakdown,omitempty"`
	DNS       *DNSResult       `json:"dns,omitempty"`
	TCP       *TCPResult       `json:"tcp,omitempty"`
	TLS       *TLSCertInfo     `json:"tls,omitempty"`
	HTTP      *HTTPResult      `json:"http,omitempty"`
}

// TargetResult holds the aggregated results for a specific endpoint target.
type TargetResult struct {
	Target         string             `json:"target"`
	ParsedTarget   ParsedTarget       `json:"parsed_target"`
	ProbeType      ProbeType          `json:"probe_type"`
	TotalSamples   int                `json:"total_samples"`
	SuccessSamples int                `json:"success_samples"`
	FailedSamples  int                `json:"failed_samples"`
	PacketLossPct  float64            `json:"packet_loss_pct"`
	Stats          stats.LatencyStats `json:"stats"`
	Issues         []DiagnosticIssue  `json:"issues,omitempty"`
	LastSample     *Sample            `json:"last_sample,omitempty"`
	Samples        []Sample           `json:"samples,omitempty"`
}

// Options configures probe behavior.
type Options struct {
	Samples             int               `json:"samples"`
	Timeout             time.Duration     `json:"timeout"`
	Interval            time.Duration     `json:"interval"`
	Concurrency         int               `json:"concurrency"`
	DNSServer           string            `json:"dns_server,omitempty"`
	SlowThreshold       time.Duration     `json:"slow_threshold"`
	HTTPMethod          string            `json:"http_method"`
	FollowRedirects     bool              `json:"follow_redirects"`
	ExpectedStatusCodes []int             `json:"expected_status_codes,omitempty"`
	Headers             map[string]string `json:"headers,omitempty"`
	InsecureSkipVerify  bool              `json:"insecure_skip_verify"`
}

// DefaultOptions returns sensible defaults for probe options.
func DefaultOptions() Options {
	return Options{
		Samples:             5,
		Timeout:             5 * time.Second,
		Interval:            1 * time.Second,
		Concurrency:         10,
		SlowThreshold:       500 * time.Millisecond,
		HTTPMethod:          "GET",
		FollowRedirects:     true,
		ExpectedStatusCodes: []int{200, 201, 202, 204, 301, 302, 307, 308},
		InsecureSkipVerify:  false,
	}
}
