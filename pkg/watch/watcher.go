package watch

import (
	"context"
	"fmt"
	"sync"
	"time"

	"svcprobe/pkg/probe"
)

// TargetStats aggregates rolling statistics over time for watch mode.
type TargetStats struct {
	Target          string
	TotalRuns       int
	TotalSamples    int
	SuccessSamples  int
	FailedSamples   int
	CumulativeP95   time.Duration
	LastResult      probe.TargetResult
}

// Watcher manages continuous service monitoring.
type Watcher struct {
	targets    []string
	probeType  probe.ProbeType
	opts       probe.Options
	onUpdate   func(iteration int, suite probe.SuiteResult, statsMap map[string]*TargetStats)
	stats      map[string]*TargetStats
	mu         sync.RWMutex
}

// NewWatcher initializes a Watcher.
func NewWatcher(targets []string, probeType probe.ProbeType, opts probe.Options, onUpdate func(int, probe.SuiteResult, map[string]*TargetStats)) *Watcher {
	return &Watcher{
		targets:   targets,
		probeType: probeType,
		opts:      opts,
		onUpdate:  onUpdate,
		stats:     make(map[string]*TargetStats),
	}
}

// Start runs the continuous monitoring loop until context cancellation.
func (w *Watcher) Start(ctx context.Context) error {
	interval := w.opts.Interval
	if interval <= 0 {
		interval = 2 * time.Second
	}

	iteration := 0
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initial immediate run
	if err := w.step(ctx, iteration); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			iteration++
			if err := w.step(ctx, iteration); err != nil {
				return err
			}
		}
	}
}

func (w *Watcher) step(ctx context.Context, iteration int) error {
	suite, err := probe.RunSuite(ctx, w.targets, w.probeType, w.opts)
	if err != nil {
		return fmt.Errorf("watch run failed: %w", err)
	}

	w.mu.Lock()
	for _, tr := range suite.Targets {
		st, exists := w.stats[tr.Target]
		if !exists {
			st = &TargetStats{Target: tr.Target}
			w.stats[tr.Target] = st
		}

		st.TotalRuns++
		st.TotalSamples += tr.TotalSamples
		st.SuccessSamples += tr.SuccessSamples
		st.FailedSamples += tr.FailedSamples
		st.CumulativeP95 += tr.Stats.P95
		st.LastResult = tr
	}
	w.mu.Unlock()

	if w.onUpdate != nil {
		w.mu.RLock()
		snapshot := copyStats(w.stats)
		w.mu.RUnlock()
		w.onUpdate(iteration, suite, snapshot)
	}

	return nil
}

func copyStats(m map[string]*TargetStats) map[string]*TargetStats {
	cp := make(map[string]*TargetStats, len(m))
	for k, v := range m {
		val := *v
		cp[k] = &val
	}
	return cp
}
