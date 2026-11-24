// Package tlsfiles loads TLS material from files and picks up rotated files
// without a restart, as cert-manager or Vault agents rotate them in place.
// Files are re-read at most once per checkInterval, on demand, so idle
// servers do no background work.
package tlsfiles

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"slices"
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

// Files names the TLS files to watch. Empty paths are not used.
type Files struct {
	CAFile   string
	CertFile string
	KeyFile  string
	// Setting names used in errors and logs, e.g. "VMANOMALY_TLS_CA_CERT_FILE".
	CASource   string
	CertSource string
}

func (f Files) paths() []string {
	return []string{f.CAFile, f.CertFile, f.KeyFile}
}

// sources names the setting of each path in paths, for error messages.
func (f Files) sources() []string {
	return []string{f.CASource, f.CertSource, f.CertSource}
}

// transport sends requests through a clone of base whose TLS settings come from
// the watched files. When any file changes, the clone is rebuilt and idle
// connections of the previous one are closed, so the next request performs a
// new handshake with the new CA and client certificate.
type transport struct {
	base  *http.Transport
	files Files

	mu        sync.Mutex
	lastCheck time.Time
	applied   [][]byte // file contents of the active transport
	rejected  string   // key of the last reported failure, see reject
	current   *http.Transport
}

// NewTransport returns a RoundTripper for base that reloads the CA bundle and the
// client certificate and key from files. The CA extends the system roots. Files are
// re-read at most once per second while requests are made; an invalid or half-written
// rotation keeps the previous material and logs one warning per distinct file content.
func NewTransport(base *http.Transport, files Files) (http.RoundTripper, error) {
	if (files.CertFile == "") != (files.KeyFile == "") {
		return nil, fmt.Errorf("%s requires both a certificate and a key file", files.CertSource)
	}
	t := &transport{base: base, files: files, lastCheck: time.Now()}
	contents, err := t.read()
	if err != nil {
		return nil, err
	}
	current, err := t.build(contents)
	if err != nil {
		return nil, err
	}
	t.current, t.applied = current, contents
	return t, nil
}

func (t *transport) read() ([][]byte, error) {
	contents := make([][]byte, 3)
	sources := t.files.sources()
	for i, path := range t.files.paths() {
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", sources[i], err)
		}
		contents[i] = data
	}
	return contents, nil
}

func (t *transport) build(contents [][]byte) (*http.Transport, error) {
	next := t.base.Clone()
	if next.TLSClientConfig == nil {
		next.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	if t.files.CAFile != "" {
		pool, err := CertPool(contents[0], t.files.CASource)
		if err != nil {
			return nil, err
		}
		next.TLSClientConfig.RootCAs = pool
	}
	if t.files.CertFile != "" {
		cert, err := tls.X509KeyPair(contents[1], contents[2])
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", t.files.CertSource, err)
		}
		// Certificates keeps Go's default selection, which honours the server's acceptable CAs.
		next.TLSClientConfig.Certificates = []tls.Certificate{cert}
		next.TLSClientConfig.GetClientCertificate = nil
	}
	return next, nil
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.active().RoundTrip(req)
}

func (t *transport) active() *http.Transport {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	if now.Sub(t.lastCheck) < checkInterval {
		return t.current
	}
	t.lastCheck = now

	contents, err := t.read()
	if err != nil {
		t.reject(err.Error(), err)
		return t.current
	}
	if slices.EqualFunc(contents, t.applied, bytes.Equal) {
		return t.current
	}
	next, err := t.build(contents)
	if err != nil {
		t.reject(contentKey(contents), err)
		return t.current
	}
	previous := t.current
	t.current, t.applied, t.rejected = next, contents, ""
	previous.CloseIdleConnections()
	slog.Info("Reloaded TLS files", "ca_file", t.files.CAFile != "", "client_certificate", t.files.CertFile != "")
	return t.current
}

// reject logs a failed reload once per key: the read error, or the content of files
// that failed to load. A broken file does not flood the log every second, while each
// new broken version is still reported.
func (t *transport) reject(key string, err error) {
	if key == t.rejected {
		return
	}
	t.rejected = key
	slog.Warn("Failed to reload TLS files, keeping the previous version", "error", err.Error())
}

func contentKey(contents [][]byte) string {
	h := sha256.New()
	for _, c := range contents {
		_, _ = fmt.Fprintf(h, "%d:", len(c))
		_, _ = h.Write(c)
	}
	return string(h.Sum(nil))
}

// CloseIdleConnections lets http.Client.CloseIdleConnections reach the active transport.
func (t *transport) CloseIdleConnections() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.current.CloseIdleConnections()
}
