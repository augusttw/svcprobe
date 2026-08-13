package stats

import (
	"testing"
	"time"
)

func TestCalculateEmpty(t *testing.T) {
	s := Calculate(nil)
	if s.Min != 0 || s.Max != 0 || s.P50 != 0 {
		t.Errorf("expected zero stats for empty slice, got %+v", s)
	}
}

func TestCalculateSingle(t *testing.T) {
	d := 10 * time.Millisecond
	s := Calculate([]time.Duration{d})
	if s.Min != d || s.Max != d || s.Mean != d || s.P50 != d || s.P95 != d || s.P99 != d {
		t.Errorf("single element stats mismatch: %+v", s)
	}
}

func TestCalculateMultiple(t *testing.T) {
	durations := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
		60 * time.Millisecond,
		70 * time.Millisecond,
		80 * time.Millisecond,
		90 * time.Millisecond,
		100 * time.Millisecond,
	}

	s := Calculate(durations)

	if s.Min != 10*time.Millisecond {
		t.Errorf("expected min 10ms, got %v", s.Min)
	}
	if s.Max != 100*time.Millisecond {
		t.Errorf("expected max 100ms, got %v", s.Max)
	}
	if s.Mean != 55*time.Millisecond {
		t.Errorf("expected mean 55ms, got %v", s.Mean)
	}

	// For 10 elements (indices 0..9), p50 index is 0.50 * 9 = 4.5 -> avg of 50ms and 60ms = 55ms
	if s.P50 != 55*time.Millisecond {
		t.Errorf("expected p50 55ms, got %v", s.P50)
	}

	// p95 index is 0.95 * 9 = 8.55 -> 90ms + 0.55*10ms = 95.5ms
	if s.P95 < 95*time.Millisecond || s.P95 > 96*time.Millisecond {
		t.Errorf("expected p95 around 95.5ms, got %v", s.P95)
	}
}
