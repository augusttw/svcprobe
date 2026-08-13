package probe

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"
)

// ProbeTLSExec performs a TLS handshake and certificate inspection probe.
func ProbeTLSExec(ctx context.Context, target ParsedTarget, opts Options) Sample {
	sample := Sample{
		Timestamp: time.Now(),
		ProbeType: ProbeTLS,
	}

	dialer := &net.Dialer{
		Timeout: opts.Timeout,
	}

	cfg := &tls.Config{
		ServerName:         target.Host,
		InsecureSkipVerify: opts.InsecureSkipVerify,
	}

	start := time.Now()
	conn, err := tls.DialWithDialer(dialer, "tcp", target.Address, cfg)
	duration := time.Since(start)
	sample.Duration = duration

	var verificationErr string
	if err != nil {
		verificationErr = err.Error()

		// Attempt dial with InsecureSkipVerify to fetch certificate info for diagnosis even if untrusted/expired
		if !opts.InsecureSkipVerify {
			insecureCfg := &tls.Config{
				ServerName:         target.Host,
				InsecureSkipVerify: true,
			}
			insConn, insErr := tls.DialWithDialer(dialer, "tcp", target.Address, insecureCfg)
			if insErr == nil {
				defer insConn.Close()
				connState := insConn.ConnectionState()
				sample.TLS = extractTLSCertInfo(target.Host, connState, verificationErr)
				sample.Success = false
				sample.Error = fmt.Sprintf("TLS verification failed for %s: %v", target.Address, err)
				return sample
			}
		}

		sample.Success = false
		sample.Error = fmt.Sprintf("TLS handshake failed to %s: %v", target.Address, err)
		return sample
	}
	defer conn.Close()

	connState := conn.ConnectionState()
	sample.Success = true
	sample.TLS = extractTLSCertInfo(target.Host, connState, "")
	return sample
}

func extractTLSCertInfo(host string, state tls.ConnectionState, verifyErr string) *TLSCertInfo {
	if len(state.PeerCertificates) == 0 {
		return &TLSCertInfo{
			VerificationError: verifyErr,
		}
	}

	cert := state.PeerCertificates[0]
	now := time.Now()
	daysRemaining := int(time.Until(cert.NotAfter).Hours() / 24)

	versionStr := tlsVersionToString(state.Version)
	cipherSuiteStr := tls.CipherSuiteName(state.CipherSuite)
	if cipherSuiteStr == "" {
		cipherSuiteStr = fmt.Sprintf("0x%04x", state.CipherSuite)
	}

	return &TLSCertInfo{
		Subject:           cert.Subject.CommonName,
		Issuer:            cert.Issuer.CommonName,
		NotBefore:         cert.NotBefore,
		NotAfter:          cert.NotAfter,
		DaysRemaining:     daysRemaining,
		IsExpired:         now.After(cert.NotAfter),
		DNSNames:          cert.DNSNames,
		CipherSuite:       cipherSuiteStr,
		Version:           versionStr,
		VerificationError: verifyErr,
	}
}

func tlsVersionToString(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("TLS 0x%04x", version)
	}
}
