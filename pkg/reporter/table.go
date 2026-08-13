package reporter

import (
	"fmt"
	"io"
	"strings"
	"time"

	"svcprobe/pkg/probe"
)

// PrintTable formats and prints a probe suite result as a clean CLI table.
func PrintTable(w io.Writer, suite probe.SuiteResult, colorize bool) {
	fmt.Fprintln(w)
	fmt.Fprintf(w, "SVCPROBE DIAGNOSTIC SUMMARY (%d targets probed in %s)\n", suite.TotalTargets, formatDuration(suite.Duration))
	fmt.Fprintf(w, "Health Overview: %s | %s | %s\n",
		formatHealthBadge("HEALTHY", suite.HealthyCount, colorize),
		formatHealthBadge("WARNING", suite.WarningCount, colorize),
		formatHealthBadge("UNHEALTHY", suite.UnhealthyCount, colorize),
	)
	fmt.Fprintln(w, strings.Repeat("─", 98))

	// Table Header
	fmt.Fprintf(w, "%-32s %-6s %-8s %-8s %-8s %-8s %-8s %-6s %-8s\n",
		"TARGET", "TYPE", "STATUS", "MIN", "P50", "P95", "P99", "LOSS%", "ISSUES")
	fmt.Fprintln(w, strings.Repeat("─", 98))

	for _, tr := range suite.Targets {
		statusStr := formatStatus(tr, colorize)
		typeStr := strings.ToUpper(string(tr.ProbeType))

		minStr := "-"
		p50Str := "-"
		p95Str := "-"
		p99Str := "-"
		if tr.SuccessSamples > 0 {
			minStr = formatDuration(tr.Stats.Min)
			p50Str = formatDuration(tr.Stats.P50)
			p95Str = formatDuration(tr.Stats.P95)
			p99Str = formatDuration(tr.Stats.P99)
		}

		lossStr := fmt.Sprintf("%.0f%%", tr.PacketLossPct)
		issuesCountStr := fmt.Sprintf("%d issue(s)", len(tr.Issues))
		if len(tr.Issues) == 0 {
			issuesCountStr = "None"
		}

		targetDisp := truncateString(tr.Target, 32)
		fmt.Fprintf(w, "%-32s %-6s %-17s %-8s %-8s %-8s %-8s %-6s %-8s\n",
			targetDisp, typeStr, statusStr, minStr, p50Str, p95Str, p99Str, lossStr, issuesCountStr)
	}

	fmt.Fprintln(w, strings.Repeat("─", 98))

	// Print Detailed Issues Section
	hasAnyIssues := false
	for _, tr := range suite.Targets {
		if len(tr.Issues) > 0 {
			if !hasAnyIssues {
				fmt.Fprintln(w, "\nDETAILED DIAGNOSTIC FINDINGS:")
				hasAnyIssues = true
			}
			fmt.Fprintf(w, "\n  ► Endpoints: %s (%s)\n", tr.Target, strings.ToUpper(string(tr.ProbeType)))
			for _, issue := range tr.Issues {
				sevBadge := fmt.Sprintf("[%s]", issue.Severity)
				if colorize {
					if issue.Severity == probe.SeverityError {
						sevBadge = "\033[31m[ERROR]\033[0m"
					} else if issue.Severity == probe.SeverityWarning {
						sevBadge = "\033[33m[WARN]\033[0m"
					}
				}
				fmt.Fprintf(w, "    %s %s: %s\n", sevBadge, issue.Code, issue.Message)
				if issue.Details != "" {
					fmt.Fprintf(w, "           Details: %s\n", issue.Details)
				}
			}
		}
	}

	// Print Detailed Latency Breakdown for HTTP/Check probes
	for _, tr := range suite.Targets {
		if tr.LastSample != nil && tr.LastSample.HTTP != nil {
			b := tr.LastSample.Breakdown
			if b.Total > 0 {
				fmt.Fprintf(w, "\n  ► Latency Phase Decomposition [%s]:\n", tr.Target)
				fmt.Fprintf(w, "    DNS Lookup:    %-10s | TCP Handshake: %-10s | TLS Handshake: %-10s\n",
					formatDuration(b.DNSLookup), formatDuration(b.TCPConnect), formatDuration(b.TLSHandshake))
				fmt.Fprintf(w, "    Time to 1st Byte: %-10s | Data Transfer: %-10s | Total Request: %-10s\n",
					formatDuration(b.TTFB), formatDuration(b.Transfer), formatDuration(b.Total))
			}
		}

		if tr.LastSample != nil && tr.LastSample.TLS != nil {
			tlsInfo := tr.LastSample.TLS
			if tlsInfo.Subject != "" {
				fmt.Fprintf(w, "\n  ► TLS Certificate Analysis [%s]:\n", tr.Target)
				fmt.Fprintf(w, "    Subject: %s | Issuer: %s\n", tlsInfo.Subject, tlsInfo.Issuer)
				fmt.Fprintf(w, "    Protocol: %s | Cipher: %s | Expires: %s (%d days remaining)\n",
					tlsInfo.Version, tlsInfo.CipherSuite, tlsInfo.NotAfter.Format("2006-01-02"), tlsInfo.DaysRemaining)
			}
		}
	}

	fmt.Fprintln(w)
}

func formatHealthBadge(label string, count int, colorize bool) string {
	str := fmt.Sprintf("%s: %d", label, count)
	if !colorize {
		return str
	}
	switch label {
	case "HEALTHY":
		return fmt.Sprintf("\033[32m%s\033[0m", str)
	case "WARNING":
		return fmt.Sprintf("\033[33m%s\033[0m", str)
	case "UNHEALTHY":
		return fmt.Sprintf("\033[31m%s\033[0m", str)
	default:
		return str
	}
}

func formatStatus(tr probe.TargetResult, colorize bool) string {
	var hasError, hasWarning bool
	for _, issue := range tr.Issues {
		if issue.Severity == probe.SeverityError {
			hasError = true
		} else if issue.Severity == probe.SeverityWarning {
			hasWarning = true
		}
	}

	if tr.FailedSamples == tr.TotalSamples || hasError {
		if colorize {
			return "\033[31m✖ FAIL\033[0m"
		}
		return "FAIL"
	} else if hasWarning || tr.FailedSamples > 0 {
		if colorize {
			return "\033[33m⚠ WARN\033[0m"
		}
		return "WARN"
	}
	if colorize {
		return "\033[32m✔ PASS\033[0m"
	}
	return "PASS"
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0ms"
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%.2fµs", float64(d.Nanoseconds())/1000.0)
	}
	if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Milliseconds())+(float64(d.Nanoseconds()%1e6)/1e6))
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
