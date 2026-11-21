// Package testtls provides a disposable PKI and mutual-TLS test server.
// It is used by tests only.
package testtls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// PKI holds paths to PEM files of a disposable CA and its certificates.
type PKI struct {
	CAFile     string
	CAKeyFile  string
	ServerCert string
	ServerKey  string
	ClientCert string
	ClientKey  string
}

// WritePKI writes a CA, a 127.0.0.1 server certificate and a client certificate to dir.
func WritePKI(t *testing.T, dir string) PKI {
	t.Helper()
	caKey := newKey(t)
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "disposable test CA"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create CA: %v", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("failed to parse CA: %v", err)
	}

	pki := PKI{CAFile: filepath.Join(dir, "ca.pem"), CAKeyFile: filepath.Join(dir, "ca.key")}
	writePEM(t, pki.CAFile, "CERTIFICATE", caDER)
	caKeyDER, err := x509.MarshalPKCS8PrivateKey(caKey)
	if err != nil {
		t.Fatalf("failed to marshal CA key: %v", err)
	}
	writePEM(t, pki.CAKeyFile, "PRIVATE KEY", caKeyDER)
	pki.ServerCert, pki.ServerKey = issue(t, dir, "server", caCert, caKey, x509.ExtKeyUsageServerAuth, 2)
	pki.ClientCert, pki.ClientKey = issue(t, dir, "client", caCert, caKey, x509.ExtKeyUsageClientAuth, 3)
	return pki
}

// NewMTLSServer starts an HTTPS server that requires client certificates signed by the PKI CA.
func NewMTLSServer(t *testing.T, handler http.Handler) (*httptest.Server, PKI) {
	t.Helper()
	pki := WritePKI(t, t.TempDir())
	serverCert, err := tls.LoadX509KeyPair(pki.ServerCert, pki.ServerKey)
	if err != nil {
		t.Fatalf("failed to load server certificate: %v", err)
	}
	caPEM, err := os.ReadFile(pki.CAFile)
	if err != nil {
		t.Fatalf("failed to read CA: %v", err)
	}
	clientCAs := x509.NewCertPool()
	clientCAs.AppendCertsFromPEM(caPEM)

	server := httptest.NewUnstartedServer(handler)
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    clientCAs,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
	}
	server.StartTLS()
	t.Cleanup(server.Close)
	return server, pki
}

// IssueClientCert issues another client certificate with the given serial from the
// PKI CA into dir and returns its paths in ClientCert and ClientKey.
func IssueClientCert(t *testing.T, dir string, pki PKI, serial int64) PKI {
	t.Helper()
	ca, err := tls.LoadX509KeyPair(pki.CAFile, pki.CAKeyFile)
	if err != nil {
		t.Fatalf("failed to load CA: %v", err)
	}
	caCert, err := x509.ParseCertificate(ca.Certificate[0])
	if err != nil {
		t.Fatalf("failed to parse CA: %v", err)
	}
	caKey, ok := ca.PrivateKey.(*ecdsa.PrivateKey)
	if !ok {
		t.Fatalf("unexpected CA key type %T", ca.PrivateKey)
	}
	issued := pki
	issued.ClientCert, issued.ClientKey = issue(t, dir, "client", caCert, caKey, x509.ExtKeyUsageClientAuth, serial)
	return issued
}

func issue(t *testing.T, dir, name string, ca *x509.Certificate, caKey *ecdsa.PrivateKey, usage x509.ExtKeyUsage, serial int64) (string, string) {
	t.Helper()
	key := newKey(t)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{usage},
	}
	if usage == x509.ExtKeyUsageServerAuth {
		template.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create %s certificate: %v", name, err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("failed to marshal %s key: %v", name, err)
	}
	certFile, keyFile := filepath.Join(dir, name+".pem"), filepath.Join(dir, name+".key")
	writePEM(t, certFile, "CERTIFICATE", der)
	writePEM(t, keyFile, "PRIVATE KEY", keyDER)
	return certFile, keyFile
}

func newKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	return key
}

func writePEM(t *testing.T, path, blockType string, der []byte) {
	t.Helper()
	data := pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der})
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}
