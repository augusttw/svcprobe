package graph

import (
	"fmt"
	"math"
	"strings"
	"time"

	"svcprobe/pkg/probe"
)

// RenderGraph builds an ASCII visual representation of service health, network topology, and latency percentiles.
func RenderGraph(suite probe.SuiteResult, colorize bool) string {
	var sb strings.Builder

	sb.WriteString("\n┌────────────────────────────────────────────────────────────────────────┐\n")
	sb.WriteString("│                       SERVICE DIAGNOSTIC TOPOLOGY                      │\n")
	sb.WriteString("└────────────────────────────────────────────────────────────────────────┘\n\n")

	// Find max latency for scaling bar chart
	var maxP95 time.Duration
	for _, tr := range suite.Targets {
		if tr.Stats.P95 > maxP95 {
			maxP95 = tr.Stats.P95
		}
	}
	if maxP95 == 0 {
		maxP95 = 1 * time.Millisecond
	}

	sb.WriteString("Endpoints Dependency & Latency Breakdown:\n")
	sb.WriteString("─────────────────────────────────────────\n")

	for i, tr := range suite.Targets {
		isLast := (i == len(suite.Targets)-1)
		prefix := "├──"
		if isLast {
			prefix = "└──"
		}

		statusSymbol := "✔ PASS"
		if colorize {
			statusSymbol = colorizeStatus(tr)
		} else {
			if tr.FailedSamples == tr.TotalSamples {
				statusSymbol = "[FAIL]"
			} else if len(tr.Issues) > 0 {
				statusSymbol = "[WARN]"
			} else {
				statusSymbol = "[PASS]"
			}
		}

		typeStr := strings.ToUpper(string(tr.ProbeType))
		sb.WriteString(fmt.Sprintf("%s [%-4s] %-30s %s\n", prefix, typeStr, tr.Target, statusSymbol))

		// Latency bar chart
		indent := "│   "
		if isLast {
			indent = "    "
		}

		if tr.SuccessSamples > 0 {
			p50Str := formatDuration(tr.Stats.P50)
			p95Str := formatDuration(tr.Stats.P95)
			p99Str := formatDuration(tr.Stats.P99)
			bar := renderBar(tr.Stats.P95, maxP95, 25)

			sb.WriteString(fmt.Sprintf("%s├── Percentiles: p50=%-7s p95=%-7s p99=%-7s\n", indent, p50Str, p95Str, p99Str))
			sb.WriteString(fmt.Sprintf("%s└── Latency Visual: [%s] (p95: %s)\n", indent, bar, p95Str))
		} else {
			sb.WriteString(fmt.Sprintf("%s└── Latency Visual: [UNREACHABLE / CONNECTION FAILED]\n", indent))
		}

		// Display diagnostic issues if any
		for j, issue := range tr.Issues {
			issuePrefix := "├──"
			if j == len(tr.Issues)-1 {
				issuePrefix = "└──"
			}
			sevLabel := issue.Severity
			sb.WriteString(fmt.Sprintf("%s    %s Issue [%s]: %s\n", indent, issuePrefix, sevLabel, issue.Message))
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

func renderBar(val, maxVal time.Duration, width int) string {
	if maxVal <= 0 {
		return strings.Repeat(" ", width)
	}
	ratio := float64(val) / float64(maxVal)
	if ratio > 1.0 {
		ratio = 1.0
	}

	filledLen := int(math.Round(ratio * float64(width)))
	if filledLen < 1 && val > 0 {
		filledLen = 1
	}

	filled := strings.Repeat("█", filledLen)
	empty := strings.Repeat("░", width-filledLen)

	return filled + empty
}

func colorizeStatus(tr probe.TargetResult) string {
	var hasError, hasWarning bool
	for _, issue := range tr.Issues {
		if issue.Severity == probe.SeverityError {
			hasError = true
		} else if issue.Severity == probe.SeverityWarning {
			hasWarning = true
		}
	}

	if tr.FailedSamples == tr.TotalSamples || hasError {
		return "\033[31m✖ FAIL\033[0m" // Red
	} else if hasWarning || tr.FailedSamples > 0 {
		return "\033[33m⚠ WARN\033[0m" // Yellow
	}
	return "\033[32m✔ PASS\033[0m" // Green
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
