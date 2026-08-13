package watch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"svcprobe/pkg/probe"
)

func TestWatcher(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts := probe.DefaultOptions()
	opts.Interval = 100 * time.Millisecond
	opts.Samples = 1

	var updateCount int32

	w := NewWatcher([]string{ts.URL}, probe.ProbeHTTP, opts, func(iteration int, suite probe.SuiteResult, statsMap map[string]*TargetStats) {
		atomic.AddInt32(&updateCount, 1)
		if atomic.LoadInt32(&updateCount) >= 3 {
			cancel()
		}
	})

	_ = w.Start(ctx)

	if atomic.LoadInt32(&updateCount) < 3 {
		t.Errorf("expected at least 3 watch updates, got %d", updateCount)
	}
}
