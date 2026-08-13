package graph

import (
	"strings"
	"testing"
	"time"

	"svcprobe/pkg/probe"
	"svcprobe/pkg/stats"
)

func TestRenderGraph(t *testing.T) {
	suite := probe.SuiteResult{
		TotalTargets:   2,
		HealthyCount:   1,
		UnhealthyCount: 1,
		Targets: []probe.TargetResult{
			{
				Target:         "https://api.service.internal",
				ProbeType:      probe.ProbeHTTP,
				TotalSamples:   5,
				SuccessSamples: 5,
				Stats: stats.LatencyStats{
					P50: 10 * time.Millisecond,
					P95: 25 * time.Millisecond,
					P99: 40 * time.Millisecond,
				},
			},
			{
				Target:         "tcp://db.internal:5432",
				ProbeType:      probe.ProbeTCP,
				TotalSamples:   5,
				SuccessSamples: 0,
				FailedSamples:  5,
				Issues: []probe.DiagnosticIssue{
					{
						Code:     "ERR_TCP_REFUSED",
						Severity: probe.SeverityError,
						Message:  "TCP connection refused on db.internal:5432",
					},
				},
			},
		},
	}

	out := RenderGraph(suite, false)
	if !strings.Contains(out, "SERVICE DIAGNOSTIC TOPOLOGY") {
		t.Errorf("expected header in graph output, got: %s", out)
	}
	if !strings.Contains(out, "api.service.internal") || !strings.Contains(out, "db.internal:5432") {
		t.Errorf("expected targets in graph output, got: %s", out)
	}
	if !strings.Contains(out, "[FAIL]") || !strings.Contains(out, "[PASS]") {
		t.Errorf("expected pass/fail status in non-colorized output, got: %s", out)
	}
}
