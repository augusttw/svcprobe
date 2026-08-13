package probe

import (
	"context"
	"fmt"
	"net"
	"time"
)

// ProbeTCPExec performs a TCP handshake / connection probe.
func ProbeTCPExec(ctx context.Context, target ParsedTarget, opts Options) Sample {
	sample := Sample{
		Timestamp: time.Now(),
		ProbeType: ProbeTCP,
	}

	dialer := net.Dialer{
		Timeout: opts.Timeout,
	}

	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", target.Address)
	duration := time.Since(start)
	sample.Duration = duration

	if err != nil {
		sample.Success = false
		sample.Error = fmt.Sprintf("TCP connect failed to %s: %v", target.Address, err)
		return sample
	}
	defer conn.Close()

	sample.Success = true
	sample.TCP = &TCPResult{
		Address:     target.Address,
		ConnectTime: duration,
	}

	return sample
}
