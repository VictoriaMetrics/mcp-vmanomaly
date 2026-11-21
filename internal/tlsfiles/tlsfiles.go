// Package tlsfiles loads TLS material from files and picks up rotated files
// without a restart, as cert-manager or Vault agents rotate them in place.
// Files are re-read at most once per checkInterval, on demand, so idle
// servers do no background work.
package tlsfiles

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"
)

// checkInterval bounds how often rotated files are re-read. Tests lower it.
var checkInterval = time.Second

// CertPool returns the system roots extended with the PEM certificates in caPEM,
// as -tlsCAFile does in VictoriaMetrics components. source names the setting in errors.
func CertPool(caPEM []byte, source string) (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("failed to load system certificate pool: %w", err)
	}
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("%s contains no PEM certificates", source)
	}
	return pool, nil
}

// fileWatch re-reads a set of files at most once per checkInterval and
// reports when their content differs from the last accepted content.
type fileWatch struct {
	paths     []string
	lastCheck time.Time
	contents  [][]byte
	lastError string
}

// poll returns the current file contents when they differ from the last accepted
// content. It reports no change when the check interval has not elapsed or a file
// cannot be read; a read failure is logged and retried after the interval.
func (w *fileWatch) poll(now time.Time) (contents [][]byte, changed bool) {
	if now.Sub(w.lastCheck) < checkInterval {
		return nil, false
	}
	w.lastCheck = now
	contents = make([][]byte, len(w.paths))
	for i, path := range w.paths {
		data, err := os.ReadFile(path)
		if err != nil {
			w.reportError(fmt.Errorf("failed to read %s: %w", path, err))
			return nil, false
		}
		contents[i] = data
	}
	for i := range contents {
		if !bytes.Equal(contents[i], w.contents[i]) {
			return contents, true
		}
	}
	return nil, false
}

// accept records contents as the active version after they were applied.
func (w *fileWatch) accept(contents [][]byte) {
	w.contents = contents
	w.lastError = ""
}

// reportError logs a reload failure once until the files change or reload succeeds,
// so a broken file does not flood the log every second.
func (w *fileWatch) reportError(err error) {
	if msg := err.Error(); msg != w.lastError {
		w.lastError = msg
		slog.Warn("Failed to reload TLS files, keeping the previous version", "error", msg)
	}
}

// CertReloader serves a client certificate from files and reloads it after rotation.
type CertReloader struct {
	mu    sync.Mutex
	watch fileWatch
	cert  *tls.Certificate
}

// NewCertReloader loads the certificate and key, failing if they are invalid.
func NewCertReloader(certFile, keyFile string) (*CertReloader, error) {
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return nil, err
	}
	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	r := &CertReloader{cert: &cert, watch: fileWatch{paths: []string{certFile, keyFile}, lastCheck: time.Now()}}
	r.watch.accept([][]byte{certPEM, keyPEM})
	return r, nil
}

// GetClientCertificate implements tls.Config.GetClientCertificate. A certificate and
// key that do not match, for example while a rotation is half written, keep the
// previous certificate until the next check.
func (r *CertReloader) GetClientCertificate(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if contents, changed := r.watch.poll(time.Now()); changed {
		cert, err := tls.X509KeyPair(contents[0], contents[1])
		if err != nil {
			r.watch.reportError(fmt.Errorf("invalid client certificate or key: %w", err))
		} else {
			r.cert = &cert
			r.watch.accept(contents)
			slog.Info("Reloaded TLS client certificate", "file", r.watch.paths[0])
		}
	}
	return r.cert, nil
}

// caTransport verifies servers against a CA file that may be rotated. A
// tls.Config cannot change its roots per handshake, so a changed CA file
// swaps in a cloned transport with the new pool.
type caTransport struct {
	base   *http.Transport
	source string

	mu      sync.Mutex
	watch   fileWatch
	current *http.Transport
}

// NewCATransport returns a RoundTripper that sends requests through a clone of base
// whose RootCAs are the system roots plus the certificates in caFile. The file is
// re-read at most once per second; when its content changes, new connections use
// the new roots and idle connections to the old ones are closed.
func NewCATransport(base *http.Transport, caFile, source string) (http.RoundTripper, error) {
	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", source, err)
	}
	t := &caTransport{base: base, source: source, watch: fileWatch{paths: []string{caFile}, lastCheck: time.Now()}}
	current, err := t.transportFor(caPEM)
	if err != nil {
		return nil, err
	}
	t.current = current
	t.watch.accept([][]byte{caPEM})
	return t, nil
}

func (t *caTransport) transportFor(caPEM []byte) (*http.Transport, error) {
	pool, err := CertPool(caPEM, t.source)
	if err != nil {
		return nil, err
	}
	transport := t.base.Clone()
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	transport.TLSClientConfig.RootCAs = pool
	return transport, nil
}

func (t *caTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.transport().RoundTrip(req)
}

func (t *caTransport) transport() *http.Transport {
	t.mu.Lock()
	defer t.mu.Unlock()
	if contents, changed := t.watch.poll(time.Now()); changed {
		next, err := t.transportFor(contents[0])
		if err != nil {
			t.watch.reportError(err)
		} else {
			previous := t.current
			t.current = next
			t.watch.accept(contents)
			previous.CloseIdleConnections()
			slog.Info("Reloaded TLS CA file", "setting", t.source)
		}
	}
	return t.current
}

// CloseIdleConnections lets http.Client.CloseIdleConnections reach the active transport.
func (t *caTransport) CloseIdleConnections() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.current.CloseIdleConnections()
}
