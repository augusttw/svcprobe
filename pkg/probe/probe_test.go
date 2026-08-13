package probe

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseTarget(t *testing.T) {
	tests := []struct {
		input      string
		forcedType ProbeType
		expectedHost string
		expectedPort int
		expectedType ProbeType
	}{
		{"example.com", ProbeHTTP, "example.com", 80, ProbeHTTP},
		{"example.com:443", ProbeCheck, "example.com", 443, ProbeHTTP},
		{"http://localhost:8080/health", ProbeCheck, "localhost", 8080, ProbeHTTP},
		{"tcp://127.0.0.1:5432", ProbeCheck, "127.0.0.1", 5432, ProbeTCP},
		{"tls://secure.host:8443", ProbeCheck, "secure.host", 8443, ProbeTLS},
	}

	for _, tc := range tests {
		pt, err := ParseTarget(tc.input, tc.forcedType)
		if err != nil {
			t.Errorf("failed to parse %s: %v", tc.input, err)
			continue
		}
		if pt.Host != tc.expectedHost {
			t.Errorf("for %s: expected host %s, got %s", tc.input, tc.expectedHost, pt.Host)
		}
		if pt.Port != tc.expectedPort {
			t.Errorf("for %s: expected port %d, got %d", tc.input, tc.expectedPort, pt.Port)
		}
		if tc.expectedType != "" && pt.Type != tc.expectedType {
			t.Errorf("for %s: expected type %s, got %s", tc.input, tc.expectedType, pt.Type)
		}
	}
}

func TestTCPProbe(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer l.Close()

	addr := l.Addr().String()
	pt, err := ParseTarget(addr, ProbeTCP)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	opts := DefaultOptions()
	sample := ProbeTCPExec(ctx, pt, opts)

	if !sample.Success {
		t.Errorf("expected TCP probe to succeed, got error: %s", sample.Error)
	}
	if sample.TCP == nil || sample.TCP.ConnectTime == 0 {
		t.Errorf("expected non-zero TCP connect time")
	}
}

func TestHTTPProbe(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	}))
	defer ts.Close()

	pt, err := ParseTarget(ts.URL, ProbeHTTP)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	opts := DefaultOptions()
	opts.Samples = 1
	sample := ProbeHTTPExec(ctx, pt, opts)

	if !sample.Success {
		t.Errorf("expected HTTP probe to succeed, got error: %s", sample.Error)
	}
	if sample.HTTP == nil || sample.HTTP.StatusCode != 200 {
		t.Errorf("expected HTTP status 200, got %+v", sample.HTTP)
	}
	if sample.Breakdown.Total == 0 {
		t.Errorf("expected breakdown total latency > 0")
	}
}

func TestTLSProbe(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	pt, err := ParseTarget(ts.URL, ProbeTLS)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	opts := DefaultOptions()
	opts.InsecureSkipVerify = true

	sample := ProbeTLSExec(ctx, pt, opts)
	if !sample.Success {
		t.Errorf("expected TLS probe to succeed with skip verify, got error: %s", sample.Error)
	}
	if sample.TLS == nil || sample.TLS.Version == "" {
		t.Errorf("expected TLS info, got %+v", sample.TLS)
	}
}

func TestRunnerConcurrent(t *testing.T) {
	ts1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts1.Close()

	ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts2.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts := DefaultOptions()
	opts.Samples = 2
	opts.Concurrency = 5

	suite, err := RunSuite(ctx, []string{ts1.URL, ts2.URL}, ProbeHTTP, opts)
	if err != nil {
		t.Fatalf("RunSuite failed: %v", err)
	}

	if suite.TotalTargets != 2 {
		t.Errorf("expected 2 targets, got %d", suite.TotalTargets)
	}
	if suite.HealthyCount != 2 {
		t.Errorf("expected 2 healthy targets, got healthy=%d, unhealth=%d", suite.HealthyCount, suite.UnhealthyCount)
	}
}
