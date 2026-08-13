package probe

import (
	"context"
	"fmt"
	"strings"
	"time"

	"svcprobe/pkg/stats"
)

// ProbeTarget executes N samples for a given parsed target according to options.
func ProbeTarget(ctx context.Context, target ParsedTarget, opts Options) TargetResult {
	if opts.Samples <= 0 {
		opts.Samples = 1
	}

	res := TargetResult{
		Target:       target.Original,
		ParsedTarget: target,
		ProbeType:    target.Type,
		Samples:      make([]Sample, 0, opts.Samples),
	}

	var durations []time.Duration
	var successCount int

	for i := 0; i < opts.Samples; i++ {
		select {
		case <-ctx.Done():
			break
		default:
		}

		var sample Sample

		if target.Type == ProbeCheck {
			sample = probeComprehensiveCheck(ctx, target, opts)
		} else {
			switch target.Type {
			case ProbeDNS:
				sample = ProbeDNSExec(ctx, target, opts)
			case ProbeTCP:
				sample = ProbeTCPExec(ctx, target, opts)
			case ProbeTLS:
				sample = ProbeTLSExec(ctx, target, opts)
			case ProbeHTTP:
				sample = ProbeHTTPExec(ctx, target, opts)
			default:
				sample = ProbeHTTPExec(ctx, target, opts)
			}
		}

		res.Samples = append(res.Samples, sample)
		if sample.Success {
			successCount++
			durations = append(durations, sample.Duration)
		}

		if i < opts.Samples-1 && opts.Interval > 0 {
			select {
			case <-ctx.Done():
				break
			case <-time.After(opts.Interval):
			}
		}
	}

	res.TotalSamples = len(res.Samples)
	res.SuccessSamples = successCount
	res.FailedSamples = res.TotalSamples - successCount

	if res.TotalSamples > 0 {
		res.PacketLossPct = (float64(res.FailedSamples) / float64(res.TotalSamples)) * 100.0
		res.LastSample = &res.Samples[len(res.Samples)-1]
	}

	if len(durations) > 0 {
		res.Stats = stats.Calculate(durations)
	}

	// Analyze diagnostic issues
	res.Issues = AnalyzeIssues(res, opts)

	return res
}

// probeComprehensiveCheck executes full diagnostic suite (DNS -> TCP -> TLS -> HTTP) for a single probe sample
func probeComprehensiveCheck(ctx context.Context, target ParsedTarget, opts Options) Sample {
	sample := Sample{
		Timestamp: time.Now(),
		ProbeType: ProbeCheck,
		Success:   true,
	}

	overallStart := time.Now()

	// 1. DNS Probe
	dnsSample := ProbeDNSExec(ctx, target, opts)
	sample.DNS = dnsSample.DNS
	if !dnsSample.Success {
		sample.Success = false
		sample.Error = dnsSample.Error
		sample.Duration = time.Since(overallStart)
		return sample
	}

	// 2. TCP Probe
	tcpSample := ProbeTCPExec(ctx, target, opts)
	sample.TCP = tcpSample.TCP
	if !tcpSample.Success {
		sample.Success = false
		sample.Error = tcpSample.Error
		sample.Duration = time.Since(overallStart)
		return sample
	}

	// 3. TLS Probe (if scheme is https or tls or port 443)
	if target.Scheme == "https" || target.Scheme == "tls" || target.Port == 443 {
		tlsSample := ProbeTLSExec(ctx, target, opts)
		sample.TLS = tlsSample.TLS
		if !tlsSample.Success {
			// Record error but proceed to HTTP check if possible
			sample.Error = tlsSample.Error
			if sample.TLS == nil || sample.TLS.IsExpired {
				sample.Success = false
			}
		}
	}

	// 4. HTTP Probe (if scheme is http/https or URL defined)
	if target.Scheme == "http" || target.Scheme == "https" || target.URL != "" {
		httpSample := ProbeHTTPExec(ctx, target, opts)
		sample.HTTP = httpSample.HTTP
		sample.Breakdown = httpSample.Breakdown
		if !httpSample.Success {
			sample.Success = false
			if sample.Error == "" {
				sample.Error = httpSample.Error
			}
		}
	}

	sample.Duration = time.Since(overallStart)
	return sample
}

// AnalyzeIssues evaluates target results and returns identified diagnostic issues.
func AnalyzeIssues(res TargetResult, opts Options) []DiagnosticIssue {
	var issues []DiagnosticIssue

	// High Packet Loss / Total Failure
	if res.FailedSamples > 0 {
		if res.FailedSamples == res.TotalSamples {
			issues = append(issues, DiagnosticIssue{
				Code:     "ERR_TOTAL_FAILURE",
				Severity: SeverityError,
				Message:  fmt.Sprintf("Endpoint failed 100%% of probe attempts (%d/%d failed)", res.FailedSamples, res.TotalSamples),
			})
		} else {
			issues = append(issues, DiagnosticIssue{
				Code:     "WARN_HIGH_FAILURE_RATE",
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("Partial failure rate: %.1f%% (%d/%d failed)", res.PacketLossPct, res.FailedSamples, res.TotalSamples),
			})
		}
	}

	if res.LastSample != nil {
		s := res.LastSample

		// Error detail inspection
		if s.Error != "" {
			errLower := strings.ToLower(s.Error)
			if strings.Contains(errLower, "no such host") || strings.Contains(errLower, "dns resolution failed") {
				issues = append(issues, DiagnosticIssue{
					Code:     "ERR_DNS_LOOKUP",
					Severity: SeverityError,
					Message:  fmt.Sprintf("DNS resolution failed for hostname '%s'", res.ParsedTarget.Host),
					Details:  s.Error,
				})
			} else if strings.Contains(errLower, "connection refused") {
				issues = append(issues, DiagnosticIssue{
					Code:     "ERR_TCP_REFUSED",
					Severity: SeverityError,
					Message:  fmt.Sprintf("TCP connection refused on %s", res.ParsedTarget.Address),
					Details:  s.Error,
				})
			} else if strings.Contains(errLower, "i/o timeout") || strings.Contains(errLower, "context deadline exceeded") {
				issues = append(issues, DiagnosticIssue{
					Code:     "ERR_TIMEOUT",
					Severity: SeverityError,
					Message:  fmt.Sprintf("Connection timed out after %v", opts.Timeout),
					Details:  s.Error,
				})
			}
		}

		// DNS Analysis
		if s.DNS != nil {
			if s.DNS.LookupTime > 200*time.Millisecond {
				issues = append(issues, DiagnosticIssue{
					Code:     "WARN_SLOW_DNS",
					Severity: SeverityWarning,
					Message:  fmt.Sprintf("Slow DNS resolution: %v (threshold: 200ms)", s.DNS.LookupTime),
				})
			}
		}

		// TLS Analysis
		if s.TLS != nil {
			if s.TLS.IsExpired {
				issues = append(issues, DiagnosticIssue{
					Code:     "ERR_TLS_EXPIRED",
					Severity: SeverityError,
					Message:  fmt.Sprintf("TLS Certificate EXPIRED on %s", s.TLS.NotAfter.Format("2006-01-02")),
					Details:  fmt.Sprintf("Subject: %s, Issuer: %s", s.TLS.Subject, s.TLS.Issuer),
				})
			} else if s.TLS.DaysRemaining <= 30 && s.TLS.DaysRemaining >= 0 {
				issues = append(issues, DiagnosticIssue{
					Code:     "WARN_TLS_EXPIRING_SOON",
					Severity: SeverityWarning,
					Message:  fmt.Sprintf("TLS Certificate expires in %d days (%s)", s.TLS.DaysRemaining, s.TLS.NotAfter.Format("2006-01-02")),
				})
			}

			if s.TLS.VerificationError != "" && !s.TLS.IsExpired {
				issues = append(issues, DiagnosticIssue{
					Code:     "ERR_TLS_UNTRUSTED",
					Severity: SeverityError,
					Message:  "TLS Certificate verification failed (untrusted CA, hostname mismatch, or self-signed)",
					Details:  s.TLS.VerificationError,
				})
			}
		}

		// HTTP Analysis
		if s.HTTP != nil {
			if s.HTTP.StatusCode >= 500 {
				issues = append(issues, DiagnosticIssue{
					Code:     "ERR_HTTP_5XX",
					Severity: SeverityError,
					Message:  fmt.Sprintf("Server returned 5xx Error: %d (%s)", s.HTTP.StatusCode, s.HTTP.StatusText),
				})
			} else if s.HTTP.StatusCode >= 400 {
				issues = append(issues, DiagnosticIssue{
					Code:     "WARN_HTTP_4XX",
					Severity: SeverityWarning,
					Message:  fmt.Sprintf("Client returned 4xx Error: %d (%s)", s.HTTP.StatusCode, s.HTTP.StatusText),
				})
			}
		}
	}

	// Slow Service Latency Check (p95 latency threshold)
	if opts.SlowThreshold > 0 && res.SuccessSamples > 0 && res.Stats.P95 > opts.SlowThreshold {
		issues = append(issues, DiagnosticIssue{
			Code:     "WARN_SLOW_SERVICE",
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("High service latency: p95 = %v (exceeds threshold %v)", res.Stats.P95, opts.SlowThreshold),
		})
	}

	return issues
}
