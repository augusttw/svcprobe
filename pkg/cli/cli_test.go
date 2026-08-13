package cli

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCLIVersionAndHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"svcprobe", "version"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "svcprobe v1.0.0") {
		t.Errorf("expected version output, got %s", stdout.String())
	}

	stdout.Reset()
	code = Run([]string{"svcprobe", "help"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("expected exit code 0 for help, got %d", code)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Errorf("expected usage output, got %s", stdout.String())
	}
}

func TestCLICheckCommand(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"svcprobe", "check", "-n", "2", "-o", "json-pretty", ts.URL}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("expected exit code 0 for healthy endpoint check, got %d, stderr: %s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, `"total_targets": 1`) || !strings.Contains(out, `"healthy_count": 1`) {
		t.Errorf("unexpected check CLI output: %s", out)
	}
}

func TestCLIGraphCommand(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"svcprobe", "graph", "-n", "2", ts.URL}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("expected exit code 0 for graph command, got %d, stderr: %s", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "SERVICE DIAGNOSTIC TOPOLOGY") {
		t.Errorf("expected graph header, got: %s", out)
	}
}
