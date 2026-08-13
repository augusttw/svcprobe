package probe

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"time"
)

// ProbeHTTPExec performs an HTTP request probe with latency breakdown.
func ProbeHTTPExec(ctx context.Context, target ParsedTarget, opts Options) Sample {
	sample := Sample{
		Timestamp: time.Now(),
		ProbeType: ProbeHTTP,
	}

	method := opts.HTTPMethod
	if method == "" {
		method = "GET"
	}

	targetURL := target.URL
	if targetURL == "" {
		targetURL = fmt.Sprintf("http://%s%s", target.Address, target.Path)
	}

	req, err := http.NewRequestWithContext(ctx, method, targetURL, nil)
	if err != nil {
		sample.Success = false
		sample.Error = fmt.Sprintf("failed to create HTTP request: %v", err)
		return sample
	}

	for k, v := range opts.Headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "svcprobe/1.0")
	}

	var (
		dnsStart, dnsDone       time.Time
		tcpStart, tcpDone       time.Time
		tlsStart, tlsDone       time.Time
		wroteReqDone, got1stByte time.Time
	)

	trace := &httptrace.ClientTrace{
		DNSStart: func(info httptrace.DNSStartInfo) {
			dnsStart = time.Now()
		},
		DNSDone: func(info httptrace.DNSDoneInfo) {
			dnsDone = time.Now()
		},
		ConnectStart: func(network, addr string) {
			tcpStart = time.Now()
		},
		ConnectDone: func(network, addr string, err error) {
			tcpDone = time.Now()
		},
		TLSHandshakeStart: func() {
			tlsStart = time.Now()
		},
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			tlsDone = time.Now()
		},
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			wroteReqDone = time.Now()
		},
		GotFirstResponseByte: func() {
			got1stByte = time.Now()
		},
	}

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: opts.InsecureSkipVerify,
			ServerName:         target.Host,
		},
		Proxy: http.ProxyFromEnvironment,
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   opts.Timeout,
	}

	if !opts.FollowRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	overallStart := time.Now()
	resp, err := client.Do(req)
	totalDuration := time.Since(overallStart)
	sample.Duration = totalDuration

	if err != nil {
		sample.Success = false
		sample.Error = fmt.Sprintf("HTTP request failed to %s: %v", targetURL, err)
		return sample
	}
	defer resp.Body.Close()

	// Read body to compute transfer time and content length
	transferStart := time.Now()
	bodyBytes, _ := io.ReadAll(resp.Body)
	transferDone := time.Now()

	// Compute latency breakdown
	var breakdown LatencyBreakdown
	if !dnsStart.IsZero() && !dnsDone.IsZero() {
		breakdown.DNSLookup = dnsDone.Sub(dnsStart)
	}
	if !tcpStart.IsZero() && !tcpDone.IsZero() {
		breakdown.TCPConnect = tcpDone.Sub(tcpStart)
	}
	if !tlsStart.IsZero() && !tlsDone.IsZero() {
		breakdown.TLSHandshake = tlsDone.Sub(tlsStart)
	}
	if !wroteReqDone.IsZero() && !got1stByte.IsZero() {
		breakdown.TTFB = got1stByte.Sub(wroteReqDone)
	} else if !got1stByte.IsZero() {
		breakdown.TTFB = got1stByte.Sub(overallStart)
	}
	if !transferStart.IsZero() && !transferDone.IsZero() {
		breakdown.Transfer = transferDone.Sub(transferStart)
	}
	breakdown.Total = totalDuration
	sample.Breakdown = breakdown

	headersMap := make(map[string]string)
	for k := range resp.Header {
		headersMap[k] = resp.Header.Get(k)
	}

	contentLength := resp.ContentLength
	if contentLength <= 0 {
		contentLength = int64(len(bodyBytes))
	}

	sample.HTTP = &HTTPResult{
		URL:           targetURL,
		StatusCode:    resp.StatusCode,
		StatusText:    resp.Status,
		Proto:         resp.Proto,
		ContentLength: contentLength,
		Breakdown:     breakdown,
		Headers:       headersMap,
	}

	// Determine success based on status codes
	isExpectedStatus := false
	if len(opts.ExpectedStatusCodes) == 0 {
		isExpectedStatus = resp.StatusCode >= 200 && resp.StatusCode < 400
	} else {
		for _, code := range opts.ExpectedStatusCodes {
			if resp.StatusCode == code {
				isExpectedStatus = true
				break
			}
		}
	}

	if isExpectedStatus {
		sample.Success = true
	} else {
		sample.Success = false
		sample.Error = fmt.Sprintf("unexpected HTTP status code %d (%s)", resp.StatusCode, resp.Status)
	}

	return sample
}
