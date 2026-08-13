package reporter

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"svcprobe/pkg/probe"
	"svcprobe/pkg/stats"
)

func TestPrintJSON(t *testing.T) {
	suite := probe.SuiteResult{
		TotalTargets: 1,
		HealthyCount: 1,
		Targets: []probe.TargetResult{
			{
				Target:         "http://localhost:8080",
				ProbeType:      probe.ProbeHTTP,
				TotalSamples:   1,
				SuccessSamples: 1,
				Stats: stats.LatencyStats{
					P50: 10 * time.Millisecond,
				},
			},
		},
	}

	var buf bytes.Buffer
	err := PrintJSON(&buf, suite, true)
	if err != nil {
		t.Fatalf("PrintJSON error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"total_targets": 1`) || !strings.Contains(out, `"http://localhost:8080"`) {
		t.Errorf("JSON output invalid: %s", out)
	}
}

func TestPrintTable(t *testing.T) {
	suite := probe.SuiteResult{
		TotalTargets: 1,
		HealthyCount: 1,
		Targets: []probe.TargetResult{
			{
				Target:         "http://localhost:8080",
				ProbeType:      probe.ProbeHTTP,
				TotalSamples:   1,
				SuccessSamples: 1,
				Stats: stats.LatencyStats{
					P50: 10 * time.Millisecond,
				},
			},
		},
	}

	var buf bytes.Buffer
	PrintTable(&buf, suite, false)
	out := buf.String()

	if !strings.Contains(out, "SVCPROBE DIAGNOSTIC SUMMARY") || !strings.Contains(out, "localhost:8080") {
		t.Errorf("Table output invalid: %s", out)
	}
}
