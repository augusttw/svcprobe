package probe

import (
	"context"
	"sync"
	"time"
)

// SuiteResult holds the total output for a probe run across multiple targets.
type SuiteResult struct {
	Timestamp      time.Time      `json:"timestamp"`
	TotalTargets   int            `json:"total_targets"`
	HealthyCount   int            `json:"healthy_count"`
	WarningCount   int            `json:"warning_count"`
	UnhealthyCount int            `json:"unhealthy_count"`
	Duration       time.Duration  `json:"total_duration_ns"`
	Targets        []TargetResult `json:"targets"`
}

// RunSuite executes probes concurrently across all target endpoints.
func RunSuite(ctx context.Context, rawTargets []string, forcedType ProbeType, opts Options) (SuiteResult, error) {
	start := time.Now()

	parsedTargets := make([]ParsedTarget, 0, len(rawTargets))
	for _, raw := range rawTargets {
		pt, err := ParseTarget(raw, forcedType)
		if err != nil {
			// If parse fails, still create a dummy target to report parse error
			pt = ParsedTarget{
				Original: raw,
				Type:     forcedType,
			}
		}
		parsedTargets = append(parsedTargets, pt)
	}

	results := make([]TargetResult, len(parsedTargets))
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = 10
	}
	if concurrency > len(parsedTargets) {
		concurrency = len(parsedTargets)
	}
	if concurrency <= 0 {
		concurrency = 1
	}

	targetChan := make(chan int, len(parsedTargets))
	for i := range parsedTargets {
		targetChan <- i
	}
	close(targetChan)

	var wg sync.WaitGroup
	wg.Add(concurrency)

	for worker := 0; worker < concurrency; worker++ {
		go func() {
			defer wg.Done()
			for idx := range targetChan {
				select {
				case <-ctx.Done():
					return
				default:
				}
				results[idx] = ProbeTarget(ctx, parsedTargets[idx], opts)
			}
		}()
	}

	wg.Wait()

	suite := SuiteResult{
		Timestamp:    start,
		TotalTargets: len(results),
		Duration:     time.Since(start),
		Targets:      results,
	}

	for _, res := range results {
		var hasError, hasWarning bool
		for _, issue := range res.Issues {
			if issue.Severity == SeverityError {
				hasError = true
			} else if issue.Severity == SeverityWarning {
				hasWarning = true
			}
		}

		if res.FailedSamples == res.TotalSamples || hasError {
			suite.UnhealthyCount++
		} else if hasWarning || res.FailedSamples > 0 {
			suite.WarningCount++
		} else {
			suite.HealthyCount++
		}
	}

	return suite, nil
}
