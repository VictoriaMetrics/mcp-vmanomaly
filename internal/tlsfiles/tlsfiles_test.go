package tlsfiles

import (
	"bytes"
	"crypto/tls"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/mcp-vmanomaly/internal/testtls"
)

func setCheckInterval(t *testing.T, d time.Duration) {
	t.Helper()
	original := checkInterval
	checkInterval = d
	t.Cleanup(func() { checkInterval = original })
}

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var out bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&out, nil)))
	t.Cleanup(func() { slog.SetDefault(original) })
	return &out
}

// copyFile writes src over dst in place, as a mounted secret update does.
func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("failed to read %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		t.Fatalf("failed to write %s: %v", dst, err)
	}
}

// mounted copies the PKI files a client uses into a separate directory.
func mounted(t *testing.T, pki testtls.PKI) testtls.PKI {
	t.Helper()
	dir := t.TempDir()
	m := testtls.PKI{
		CAFile:     filepath.Join(dir, "ca.pem"),
		ClientCert: filepath.Join(dir, "client.pem"),
		ClientKey:  filepath.Join(dir, "client-key.pem"),
	}
	copyFile(t, pki.CAFile, m.CAFile)
	copyFile(t, pki.ClientCert, m.ClientCert)
	copyFile(t, pki.ClientKey, m.ClientKey)
	return m
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
}

func newClient(t *testing.T, files testtls.PKI) *http.Client {
	t.Helper()
	certs, err := NewCertReloader(files.ClientCert, files.ClientKey)
	if err != nil {
		t.Fatalf("NewCertReloader failed: %v", err)
	}
	base := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, GetClientCertificate: certs.GetClientCertificate}}
	transport, err := NewCATransport(base, files.CAFile, "TEST_CA_FILE")
	if err != nil {
		t.Fatalf("NewCATransport failed: %v", err)
	}
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	t.Cleanup(client.CloseIdleConnections)
	return client
}

func get(client *http.Client, url string) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

func TestRotatedCAAndClientCertificateAreUsedWithoutRestart(t *testing.T) {
	setCheckInterval(t, 0)
	oldServer, oldPKI := testtls.NewMTLSServer(t, okHandler())
	newServer, newPKI := testtls.NewMTLSServer(t, okHandler())
	files := mounted(t, oldPKI)
	client := newClient(t, files)

	if err := get(client, oldServer.URL); err != nil {
		t.Fatalf("request with the initial files failed: %v", err)
	}
	if err := get(client, newServer.URL); err == nil {
		t.Fatal("the new server must be rejected before rotation")
	}

	copyFile(t, newPKI.CAFile, files.CAFile)
	copyFile(t, newPKI.ClientCert, files.ClientCert)
	copyFile(t, newPKI.ClientKey, files.ClientKey)

	if err := get(client, newServer.URL); err != nil {
		t.Fatalf("request after rotation failed: %v", err)
	}
	client.CloseIdleConnections()
	if err := get(client, oldServer.URL); err == nil {
		t.Fatal("the old CA must no longer be trusted after rotation")
	}
}

func TestFilesAreReReadAtMostOncePerInterval(t *testing.T) {
	setCheckInterval(t, time.Hour)
	_, oldPKI := testtls.NewMTLSServer(t, okHandler())
	newServer, newPKI := testtls.NewMTLSServer(t, okHandler())
	files := mounted(t, oldPKI)
	client := newClient(t, files)

	copyFile(t, newPKI.CAFile, files.CAFile)
	copyFile(t, newPKI.ClientCert, files.ClientCert)
	copyFile(t, newPKI.ClientKey, files.ClientKey)

	if err := get(client, newServer.URL); err == nil {
		t.Fatal("files must not be re-read before the check interval elapses")
	}
}

func TestInvalidRotationKeepsPreviousMaterial(t *testing.T) {
	setCheckInterval(t, 0)
	logs := captureLogs(t)
	server, pki := testtls.NewMTLSServer(t, okHandler())
	_, otherPKI := testtls.NewMTLSServer(t, okHandler())
	files := mounted(t, pki)
	client := newClient(t, files)

	// A half-written rotation: a garbage CA and a certificate that does not match the key.
	if err := os.WriteFile(files.CAFile, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	copyFile(t, otherPKI.ClientCert, files.ClientCert)

	for range 3 {
		client.CloseIdleConnections()
		if err := get(client, server.URL); err != nil {
			t.Fatalf("request with the previous material failed: %v", err)
		}
	}
	output := logs.String()
	if got := strings.Count(output, "contains no PEM certificates"); got != 1 {
		t.Errorf("CA failure logged %d times, want once:\n%s", got, output)
	}
	if got := strings.Count(output, "invalid client certificate or key"); got != 1 {
		t.Errorf("certificate failure logged %d times, want once:\n%s", got, output)
	}

	// Completing the rotation recovers.
	copyFile(t, pki.CAFile, files.CAFile)
	copyFile(t, pki.ClientCert, files.ClientCert)
	client.CloseIdleConnections()
	if err := get(client, server.URL); err != nil {
		t.Fatalf("request after a completed rotation failed: %v", err)
	}
}

func TestConstructorsRejectInvalidFiles(t *testing.T) {
	pki := testtls.WritePKI(t, t.TempDir())
	missing := filepath.Join(t.TempDir(), "missing.pem")
	if _, err := NewCertReloader(missing, pki.ClientKey); err == nil {
		t.Error("expected an error for a missing certificate")
	}
	if _, err := NewCertReloader(pki.ServerCert, pki.ClientKey); err == nil {
		t.Error("expected an error for a mismatched key")
	}
	if _, err := NewCATransport(&http.Transport{}, missing, "TEST_CA_FILE"); err == nil {
		t.Error("expected an error for a missing CA file")
	}
	if _, err := NewCATransport(&http.Transport{}, pki.ClientKey, "TEST_CA_FILE"); err == nil || !strings.Contains(err.Error(), "TEST_CA_FILE") {
		t.Errorf("expected a no-certificates error naming the setting, got %v", err)
	}
}
