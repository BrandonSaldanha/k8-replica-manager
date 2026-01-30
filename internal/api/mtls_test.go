package api

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/BrandonSaldanha/k8-replica-manager/internal/config"
	"github.com/BrandonSaldanha/k8-replica-manager/internal/kube"
)

type readyStore struct{}

func (readyStore) Ready() bool { return true }
func (readyStore) ListDeployments(ctx context.Context) ([]string, error) {
	return []string{"demo"}, nil
}
func (readyStore) GetReplicas(ctx context.Context, name string) (int32, bool, error) {
	return 1, true, nil
}
func (readyStore) SetReplicas(ctx context.Context, name string, replicas int32) error { return nil }

var _ kube.Store = (*readyStore)(nil)

func TestProbeEndpointsUnauthenticated(t *testing.T) {
	s := New(config.Config{ListenAddr: ":0", ProbeListenAddr: ":0"}, readyStore{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	s.handleHealthz(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("healthz expected 200 got %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/readyz", nil)
	s.handleReadyz(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("readyz expected 200 got %d (%s)", rr.Code, rr.Body.String())
	}
}

func TestAPIMTLSRejectsMissingClientCert(t *testing.T) {
	ca, serverCert, clientCert, badClientCert, roots := mustMakeTestPKI(t)

	cfg := config.Config{
		ListenAddr:      ":0",
		ProbeListenAddr: ":0",
		TLSEnabled:      true,
	}
	s := New(cfg, readyStore{})

	// mirror production mTLS behavior
	tlsCfg := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    ca.pool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}

	// same API wiring as New(): only API routes
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/v1/", s.routeAPIv1)

	ts := httptest.NewUnstartedServer(apiMux)
	ts.TLS = tlsCfg
	ts.StartTLS()
	defer ts.Close()

	// 1) client WITHOUT cert (but trusts server) => handshake must fail
	noCertClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: roots},
		},
		Timeout: 2 * time.Second,
	}
	_, err := noCertClient.Get(ts.URL + "/api/v1/deployments")
	if err == nil {
		t.Fatalf("expected TLS handshake failure without client cert, got nil error")
	}

	// 2) client WITH valid cert => should succeed (HTTP response)
	okClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:      roots,
				Certificates: []tls.Certificate{clientCert},
			},
		},
		Timeout: 2 * time.Second,
	}
	resp, err := okClient.Get(ts.URL + "/api/v1/deployments")
	if err != nil {
		t.Fatalf("expected success with client cert, got %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 got %d", resp.StatusCode)
	}

	// 3) client WITH cert signed by wrong CA => handshake must fail
	badClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:      roots,
				Certificates: []tls.Certificate{badClientCert},
			},
		},
		Timeout: 2 * time.Second,
	}
	_, err = badClient.Get(ts.URL + "/api/v1/deployments")
	if err == nil {
		t.Fatalf("expected TLS handshake failure with wrong client cert, got nil error")
	}
}

// ---- PKI helpers (self-contained) ----

type testCA struct {
	cert *x509.Certificate
	key  *rsa.PrivateKey
	pool *x509.CertPool
}

func mustMakeTestPKI(t *testing.T) (ca testCA, server tls.Certificate, client tls.Certificate, badClient tls.Certificate, roots *x509.CertPool) {
	t.Helper()

	// CA
	caKey := mustRSA(t, 2048)
	caTpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	caDER := mustCreateCert(t, caTpl, caTpl, &caKey.PublicKey, caKey)
	caCert := mustParseCert(t, caDER)

	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	roots = x509.NewCertPool()
	roots.AddCert(caCert)

	ca = testCA{cert: caCert, key: caKey, pool: caPool}

	// Server cert with SANs (important even for httptest sometimes)
	serverKey := mustRSA(t, 2048)
	serverTpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	serverDER := mustCreateCert(t, serverTpl, caCert, &serverKey.PublicKey, caKey)
	server = mustTLSCert(t, serverDER, serverKey, caDER)

	// Good client cert (signed by CA)
	clientKey := mustRSA(t, 2048)
	clientTpl := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "good-client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	clientDER := mustCreateCert(t, clientTpl, caCert, &clientKey.PublicKey, caKey)
	client = mustTLSCert(t, clientDER, clientKey, caDER)

	// Bad client cert (signed by different CA)
	badCAKey := mustRSA(t, 2048)
	badCATpl := &x509.Certificate{
		SerialNumber:          big.NewInt(10),
		Subject:               pkix.Name{CommonName: "bad-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	badCADER := mustCreateCert(t, badCATpl, badCATpl, &badCAKey.PublicKey, badCAKey)
	badCACert := mustParseCert(t, badCADER)

	badClientKey := mustRSA(t, 2048)
	badClientTpl := &x509.Certificate{
		SerialNumber: big.NewInt(11),
		Subject:      pkix.Name{CommonName: "bad-client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	badClientDER := mustCreateCert(t, badClientTpl, badCACert, &badClientKey.PublicKey, badCAKey)
	badClient = mustTLSCert(t, badClientDER, badClientKey, badCADER)

	return
}

func mustRSA(t *testing.T, bits int) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		t.Fatalf("rsa key: %v", err)
	}
	return k
}

func mustCreateCert(t *testing.T, tpl, parent *x509.Certificate, pub any, parentKey any) []byte {
	t.Helper()
	der, err := x509.CreateCertificate(rand.Reader, tpl, parent, pub, parentKey)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	return der
}

func mustParseCert(t *testing.T, der []byte) *x509.Certificate {
	t.Helper()
	c, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	return c
}

func mustTLSCert(t *testing.T, leafDER []byte, leafKey *rsa.PrivateKey, caDER []byte) tls.Certificate {
	t.Helper()
	leafPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(leafKey)})

	// include CA in the chain so clients can build it if needed
	cert, err := tls.X509KeyPair(append(leafPEM, caPEM...), keyPEM)
	if err != nil {
		t.Fatalf("x509 key pair: %v", err)
	}
	return cert
}
