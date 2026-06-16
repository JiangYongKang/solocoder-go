package certrotator

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"math/big"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type mockLoader struct {
	cert *tls.Certificate
	err  error
}

func (m *mockLoader) LoadCertificate() (*tls.Certificate, error) {
	return m.cert, m.err
}

type mockIssuer struct {
	certs  []*tls.Certificate
	idx    int
	mu     sync.Mutex
	err    error
	callCount int
}

func (m *mockIssuer) IssueCertificate() (*tls.Certificate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	if m.err != nil {
		return nil, m.err
	}
	if m.certs == nil || m.idx >= len(m.certs) {
		return nil, errors.New("no more certificates")
	}
	cert := m.certs[m.idx]
	m.idx++
	return cert, nil
}

func (m *mockIssuer) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

type mockConn struct {
	closed atomic.Bool
}

func (m *mockConn) Close() error {
	m.closed.Store(true)
	return nil
}

func generateTestCertificate(notBefore, notAfter time.Time, isCA bool, parent *x509.Certificate, parentKey *ecdsa.PrivateKey) (*tls.Certificate, *ecdsa.PrivateKey, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	var keyUsage x509.KeyUsage
	var extKeyUsage []x509.ExtKeyUsage
	var cn string
	var dnsNames []string
	var ipAddresses []net.IP

	if isCA {
		keyUsage = x509.KeyUsageCertSign | x509.KeyUsageCRLSign
		extKeyUsage = nil
		if parent == nil {
			cn = "Test Root CA"
		} else {
			cn = "Test Intermediate CA"
		}
	} else {
		keyUsage = x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment
		extKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		cn = "test.example.com"
		dnsNames = []string{"test.example.com"}
		ipAddresses = []net.IP{net.ParseIP("127.0.0.1")}
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
			CommonName:   cn,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              keyUsage,
		ExtKeyUsage:           extKeyUsage,
		BasicConstraintsValid: true,
		IsCA:                  isCA,
		DNSNames:              dnsNames,
		IPAddresses:           ipAddresses,
	}

	var signingCert *x509.Certificate
	var signingKey *ecdsa.PrivateKey
	if parent != nil && parentKey != nil {
		signingCert = parent
		signingKey = parentKey
	} else {
		signingCert = template
		signingKey = priv
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, signingCert, &priv.PublicKey, signingKey)
	if err != nil {
		return nil, nil, err
	}

	tlsCert := &tls.Certificate{
		Certificate: [][]byte{derBytes},
		PrivateKey:  priv,
	}

	leaf, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return nil, nil, err
	}
	tlsCert.Leaf = leaf

	return tlsCert, priv, nil
}

func generateCertificateChain() (*tls.Certificate, *x509.Certificate, *x509.CertPool, error) {
	now := time.Now()

	rootCACert, rootKey, err := generateTestCertificate(
		now.Add(-time.Hour), now.Add(10*365*24*time.Hour), true, nil, nil,
	)
	if err != nil {
		return nil, nil, nil, err
	}

	leafCert, _, err := generateTestCertificate(
		now.Add(-time.Hour), now.Add(365*24*time.Hour), false, rootCACert.Leaf, rootKey,
	)
	if err != nil {
		return nil, nil, nil, err
	}

	chainCert := &tls.Certificate{
		Certificate: [][]byte{
			leafCert.Certificate[0],
		},
		PrivateKey: leafCert.PrivateKey,
	}
	chainCert.Leaf = leafCert.Leaf

	rootPool := x509.NewCertPool()
	rootPool.AddCert(rootCACert.Leaf)

	return chainCert, rootCACert.Leaf, rootPool, nil
}

func generateSelfSignedCert(notBefore, notAfter time.Time) *tls.Certificate {
	cert, _, err := generateTestCertificate(notBefore, notAfter, false, nil, nil)
	if err != nil {
		panic(err)
	}
	return cert
}

func TestCertStatusString(t *testing.T) {
	tests := []struct {
		status CertStatus
		want   string
	}{
		{CertStatusActive, "ACTIVE"},
		{CertStatusPending, "PENDING"},
		{CertStatusRetiring, "RETIRING"},
		{CertStatusRetired, "RETIRED"},
		{CertStatus(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("CertStatus(%d).String() = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}
	if cfg.CheckInterval != time.Hour {
		t.Errorf("Default CheckInterval = %v, want %v", cfg.CheckInterval, time.Hour)
	}
	if cfg.RenewalBuffer != 30*24*time.Hour {
		t.Errorf("Default RenewalBuffer = %v, want 30 days", cfg.RenewalBuffer)
	}
	if cfg.RetirementTimeout != 5*time.Minute {
		t.Errorf("Default RetirementTimeout = %v, want 5 minutes", cfg.RetirementTimeout)
	}
	if !cfg.PreValidationChecks {
		t.Error("Default PreValidationChecks should be true")
	}
	if !cfg.EnableLogging {
		t.Error("Default EnableLogging should be true")
	}
}

func TestNewWithNilIssuer(t *testing.T) {
	loader := &mockLoader{cert: generateSelfSignedCert(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))}
	_, err := New(nil, loader, nil)
	if err != ErrIssuerNil {
		t.Errorf("New(nil issuer) error = %v, want %v", err, ErrIssuerNil)
	}
}

func TestNewWithNilLoader(t *testing.T) {
	issuer := &mockIssuer{}
	_, err := New(issuer, nil, nil)
	if err == nil {
		t.Error("New(nil loader) should return error")
	}
}

func TestNewWithDefaultConfig(t *testing.T) {
	loader := &mockLoader{cert: generateSelfSignedCert(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))}
	issuer := &mockIssuer{}
	cr, err := New(issuer, loader, &Config{PreValidationChecks: false})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	if cr.ActiveCertificate() == nil {
		t.Error("ActiveCertificate() returned nil")
	}
}

func TestNewWithCustomConfig(t *testing.T) {
	loader := &mockLoader{cert: generateSelfSignedCert(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))}
	issuer := &mockIssuer{}

	customInterval := 5 * time.Minute
	cfg := &Config{
		CheckInterval: customInterval,
		RenewalBuffer: 7 * 24 * time.Hour,
		PreValidationChecks: false,
	}

	cr, err := New(issuer, loader, cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	if cr.config.CheckInterval != customInterval {
		t.Errorf("CheckInterval = %v, want %v", cr.config.CheckInterval, customInterval)
	}
	if cr.config.RenewalBuffer != 7*24*time.Hour {
		t.Errorf("RenewalBuffer = %v, want 7 days", cr.config.RenewalBuffer)
	}
	if cr.config.PreValidationChecks != false {
		t.Error("PreValidationChecks should be false")
	}
}

func TestNewWithInvalidInitialCert(t *testing.T) {
	expiredCert := generateSelfSignedCert(time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour))
	loader := &mockLoader{cert: expiredCert}
	issuer := &mockIssuer{}

	_, err := New(issuer, loader, &Config{PreValidationChecks: false})
	if err == nil {
		t.Error("New() with expired cert should return error")
	}
}

func TestNewWithNotYetValidCert(t *testing.T) {
	futureCert := generateSelfSignedCert(time.Now().Add(time.Hour), time.Now().Add(2*time.Hour))
	loader := &mockLoader{cert: futureCert}
	issuer := &mockIssuer{}

	_, err := New(issuer, loader, &Config{PreValidationChecks: false})
	if err == nil {
		t.Error("New() with not-yet-valid cert should return error")
	}
}

func TestNewWithLoaderError(t *testing.T) {
	loader := &mockLoader{err: errors.New("load error")}
	issuer := &mockIssuer{}

	_, err := New(issuer, loader, nil)
	if err == nil {
		t.Error("New() with loader error should return error")
	}
}

func TestVerifyCertificateChain_Success(t *testing.T) {
	chainCert, _, rootPool, err := generateCertificateChain()
	if err != nil {
		t.Fatalf("generateCertificateChain() error = %v", err)
	}

	cfg := &Config{
		RootCAs: rootPool,
		PreValidationChecks: true,
	}
	loader := &mockLoader{cert: chainCert}
	issuer := &mockIssuer{}

	cr, err := New(issuer, loader, cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	if err := cr.VerifyCertificateChain(chainCert); err != nil {
		t.Errorf("VerifyCertificateChain() error = %v", err)
	}
}

func TestVerifyCertificateChain_EmptyCert(t *testing.T) {
	loader := &mockLoader{cert: generateSelfSignedCert(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))}
	issuer := &mockIssuer{}
	cr, err := New(issuer, loader, &Config{PreValidationChecks: false})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	emptyCert := &tls.Certificate{}
	err = cr.VerifyCertificateChain(emptyCert)
	if err != ErrCertChainIncomplete {
		t.Errorf("VerifyCertificateChain(empty) error = %v, want %v", err, ErrCertChainIncomplete)
	}
}

func TestVerifyCertificateChain_ExpiredLeaf(t *testing.T) {
	loader := &mockLoader{cert: generateSelfSignedCert(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))}
	issuer := &mockIssuer{}
	cr, err := New(issuer, loader, &Config{PreValidationChecks: false})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	expiredCert := generateSelfSignedCert(time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour))
	err = cr.VerifyCertificateChain(expiredCert)
	if err == nil {
		t.Error("VerifyCertificateChain(expired) should return error")
	}
}

func TestVerifyCertificateChain_NotYetValidLeaf(t *testing.T) {
	loader := &mockLoader{cert: generateSelfSignedCert(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))}
	issuer := &mockIssuer{}
	cr, err := New(issuer, loader, &Config{PreValidationChecks: false})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	futureCert := generateSelfSignedCert(time.Now().Add(time.Hour), time.Now().Add(2*time.Hour))
	err = cr.VerifyCertificateChain(futureCert)
	if err == nil {
		t.Error("VerifyCertificateChain(future) should return error")
	}
}

func TestVerifyCertificateChain_UntrustedRoot(t *testing.T) {
	chainCert, _, _, err := generateCertificateChain()
	if err != nil {
		t.Fatalf("generateCertificateChain() error = %v", err)
	}

	otherRoot := x509.NewCertPool()
	otherRootCert, _, err := generateTestCertificate(time.Now().Add(-time.Hour), time.Now().Add(time.Hour), true, nil, nil)
	if err != nil {
		t.Fatalf("generate other root error = %v", err)
	}
	otherRoot.AddCert(otherRootCert.Leaf)

	cfg := &Config{
		RootCAs: otherRoot,
		PreValidationChecks: false,
	}
	loader := &mockLoader{cert: generateSelfSignedCert(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))}
	issuer := &mockIssuer{}

	cr, err := New(issuer, loader, cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	err = cr.VerifyCertificateChain(chainCert)
	if err == nil {
		t.Error("VerifyCertificateChain(untrusted root) should return error")
	}
}

func TestGetCertificate(t *testing.T) {
	loader := &mockLoader{cert: generateSelfSignedCert(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))}
	issuer := &mockIssuer{}
	cr, err := New(issuer, loader, &Config{PreValidationChecks: false})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	cert, err := cr.GetCertificate(nil)
	if err != nil {
		t.Fatalf("GetCertificate() error = %v", err)
	}
	if cert == nil {
		t.Fatal("GetCertificate() returned nil cert")
	}
	if cert != cr.ActiveCertificate().TLSCert {
		t.Error("GetCertificate() should return active cert")
	}
}

func TestNeedsRenewal(t *testing.T) {
	now := time.Now()
	loader := &mockLoader{cert: generateSelfSignedCert(now.Add(-time.Hour), now.Add(10*24*time.Hour))}
	issuer := &mockIssuer{}
	cfg := &Config{
		RenewalBuffer: 30 * 24 * time.Hour,
		PreValidationChecks: false,
	}
	cr, err := New(issuer, loader, cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	cr.clock = func() time.Time { return now }

	if !cr.NeedsRenewal() {
		t.Error("NeedsRenewal() should be true when cert expiring in 10 days with 30 day buffer")
	}
}

func TestNeedsRenewal_False(t *testing.T) {
	now := time.Now()
	loader := &mockLoader{cert: generateSelfSignedCert(now.Add(-time.Hour), now.Add(60*24*time.Hour))}
	issuer := &mockIssuer{}
	cfg := &Config{
		RenewalBuffer: 30 * 24 * time.Hour,
		PreValidationChecks: false,
	}
	cr, err := New(issuer, loader, cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	cr.clock = func() time.Time { return now }

	if cr.NeedsRenewal() {
		t.Error("NeedsRenewal() should be false when cert expiring in 60 days with 30 day buffer")
	}
}

func TestTimeUntilExpiry(t *testing.T) {
	now := time.Now()
	expiry := now.Add(24 * time.Hour)
	loader := &mockLoader{cert: generateSelfSignedCert(now.Add(-time.Hour), expiry)}
	issuer := &mockIssuer{}
	cr, err := New(issuer, loader, &Config{PreValidationChecks: false})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	cr.clock = func() time.Time { return now }

	expected := 24 * time.Hour
	actual := cr.TimeUntilExpiry()
	if actual < expected-time.Second || actual > expected {
		t.Errorf("TimeUntilExpiry() = %v, want ~%v", actual, expected)
	}
}

func TestTimeUntilRenewal(t *testing.T) {
	now := time.Now()
	expiry := now.Add(60 * 24 * time.Hour)
	buffer := 30 * 24 * time.Hour
	loader := &mockLoader{cert: generateSelfSignedCert(now.Add(-time.Hour), expiry)}
	issuer := &mockIssuer{}
	cfg := &Config{
		RenewalBuffer: buffer,
		PreValidationChecks: false,
	}
	cr, err := New(issuer, loader, cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	cr.clock = func() time.Time { return now }

	expected := 30 * 24 * time.Hour
	actual := cr.TimeUntilRenewal()
	if actual < expected-time.Second || actual > expected {
		t.Errorf("TimeUntilRenewal() = %v, want ~%v", actual, expected)
	}
}

func TestForceRenew(t *testing.T) {
	now := time.Now()
	initialCert := generateSelfSignedCert(now.Add(-time.Hour), now.Add(time.Hour))
	newCert := generateSelfSignedCert(now.Add(-time.Hour), now.Add(2*time.Hour))

	loader := &mockLoader{cert: initialCert}
	issuer := &mockIssuer{certs: []*tls.Certificate{newCert}}

	cr, err := New(issuer, loader, &Config{
		PreValidationChecks: false,
		RenewalBuffer: 30 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	originalActive := cr.ActiveCertificate()
	originalID := originalActive.ID

	var events []RotationEvent
	cr.SetEventHandler(func(event RotationEvent) {
		events = append(events, event)
	})

	if err := cr.ForceRenew(); err != nil {
		t.Fatalf("ForceRenew() error = %v", err)
	}

	newActive := cr.ActiveCertificate()
	if newActive == nil {
		t.Fatal("ActiveCertificate() is nil after renew")
	}
	if newActive.ID == originalID {
		t.Error("Certificate ID should change after renew")
	}
	if newActive.Status != CertStatusActive {
		t.Errorf("New cert status = %v, want ACTIVE", newActive.Status)
	}

	retiring := cr.RetiringCertificates()
	if len(retiring) != 1 {
		t.Errorf("Retiring certificates count = %d, want 1", len(retiring))
	}
	if retiring[0].ID != originalID {
		t.Errorf("Retiring cert ID = %s, want %s", retiring[0].ID, originalID)
	}
	if retiring[0].Status != CertStatusRetiring {
		t.Errorf("Retiring cert status = %v, want RETIRING", retiring[0].Status)
	}

	eventTypes := make([]string, 0, len(events))
	for _, e := range events {
		eventTypes = append(eventTypes, e.Type)
	}

	expectedEvents := []string{"CERT_PENDING", "CERT_ACTIVATED", "CERT_RETIRING"}
	for _, expected := range expectedEvents {
		found := false
		for _, et := range eventTypes {
			if et == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected event %s not found in events: %v", expected, eventTypes)
		}
	}
}

func TestForceRenew_IssuerError(t *testing.T) {
	loader := &mockLoader{cert: generateSelfSignedCert(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))}
	issuer := &mockIssuer{err: errors.New("issuer error")}

	cr, err := New(issuer, loader, &Config{PreValidationChecks: false})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	err = cr.ForceRenew()
	if err == nil {
		t.Error("ForceRenew() should return error when issuer fails")
	}
}

func TestForceRenew_InvalidNewCert(t *testing.T) {
	now := time.Now()
	initialCert := generateSelfSignedCert(now.Add(-time.Hour), now.Add(time.Hour))
	invalidCert := generateSelfSignedCert(now.Add(2*time.Hour), now.Add(3*time.Hour))

	loader := &mockLoader{cert: initialCert}
	issuer := &mockIssuer{certs: []*tls.Certificate{invalidCert}}

	cr, err := New(issuer, loader, &Config{PreValidationChecks: false})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	err = cr.ForceRenew()
	if err == nil {
		t.Error("ForceRenew() should return error for invalid new cert")
	}
}

func TestForceSwitch_NoPending(t *testing.T) {
	loader := &mockLoader{cert: generateSelfSignedCert(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))}
	issuer := &mockIssuer{}
	cr, err := New(issuer, loader, &Config{PreValidationChecks: false})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	err = cr.ForceSwitch()
	if err != ErrNoPendingCert {
		t.Errorf("ForceSwitch() error = %v, want %v", err, ErrNoPendingCert)
	}
}

func TestForceSwitch_AlreadyActive(t *testing.T) {
	now := time.Now()
	initialCert := generateSelfSignedCert(now.Add(-time.Hour), now.Add(time.Hour))
	loader := &mockLoader{cert: initialCert}
	issuer := &mockIssuer{certs: []*tls.Certificate{initialCert}}

	cr, err := New(issuer, loader, &Config{PreValidationChecks: false})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	cr.certMu.Lock()
	cr.pendingCert = cr.ActiveCertificate()
	cr.certMu.Unlock()

	err = cr.ForceSwitch()
	if err != ErrCertAlreadyActive {
		t.Errorf("ForceSwitch() error = %v, want %v", err, ErrCertAlreadyActive)
	}
}

func TestTrackConnection(t *testing.T) {
	loader := &mockLoader{cert: generateSelfSignedCert(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))}
	issuer := &mockIssuer{}
	cr, err := New(issuer, loader, &Config{PreValidationChecks: false})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	active := cr.ActiveCertificate()
	certID := active.ID

	mc1 := &mockConn{}
	release1 := cr.TrackConnection(certID, mc1, mc1.Close)

	mc2 := &mockConn{}
	release2 := cr.TrackConnection(certID, mc2, mc2.Close)

	if count := cr.ActiveConnections(certID); count != 2 {
		t.Errorf("ActiveConnections() = %d, want 2", count)
	}

	release1()

	if count := cr.ActiveConnections(certID); count != 1 {
		t.Errorf("ActiveConnections() after release = %d, want 1", count)
	}

	release2()

	if count := cr.ActiveConnections(certID); count != 0 {
		t.Errorf("ActiveConnections() after all releases = %d, want 0", count)
	}
}

func TestCloseConnections(t *testing.T) {
	loader := &mockLoader{cert: generateSelfSignedCert(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))}
	issuer := &mockIssuer{}
	cr, err := New(issuer, loader, &Config{PreValidationChecks: false})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	active := cr.ActiveCertificate()
	certID := active.ID

	conns := make([]*mockConn, 3)
	releases := make([]func(), 3)
	for i := 0; i < 3; i++ {
		conns[i] = &mockConn{}
		releases[i] = cr.TrackConnection(certID, conns[i], conns[i].Close)
	}

	cr.CloseConnections(certID)

	for i, mc := range conns {
		if !mc.closed.Load() {
			t.Errorf("Connection %d should be closed", i)
		}
	}

	if count := cr.ActiveConnections(certID); count != 0 {
		t.Errorf("ActiveConnections() after CloseConnections = %d, want 0", count)
	}
}

func TestCompleteRetirement(t *testing.T) {
	now := time.Now()
	initialCert := generateSelfSignedCert(now.Add(-time.Hour), now.Add(time.Hour))
	newCert := generateSelfSignedCert(now.Add(-time.Hour), now.Add(2*time.Hour))

	loader := &mockLoader{cert: initialCert}
	issuer := &mockIssuer{certs: []*tls.Certificate{newCert}}

	cr, err := New(issuer, loader, &Config{
		PreValidationChecks: false,
		RetirementTimeout: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	cr.clock = func() time.Time { return now }

	events := make(map[string]int)
	cr.SetEventHandler(func(event RotationEvent) {
		events[event.Type]++
	})

	originalActive := cr.ActiveCertificate()
	_ = originalActive.ID

	if err := cr.ForceRenew(); err != nil {
		t.Fatalf("ForceRenew() error = %v", err)
	}

	retiringCerts := cr.RetiringCertificates()
	if len(retiringCerts) != 1 {
		t.Fatalf("Retiring certificates count = %d, want 1", len(retiringCerts))
	}

	time.Sleep(200 * time.Millisecond)

	retiringCerts = cr.RetiringCertificates()
	if len(retiringCerts) != 0 {
		t.Errorf("Retiring certificates count after timeout = %d, want 0", len(retiringCerts))
	}

	if events["CERT_RETIRED"] != 1 {
		t.Errorf("CERT_RETIRED events = %d, want 1", events["CERT_RETIRED"])
	}
}

func TestForceRetirement(t *testing.T) {
	now := time.Now()
	initialCert := generateSelfSignedCert(now.Add(-time.Hour), now.Add(time.Hour))
	newCert := generateSelfSignedCert(now.Add(-time.Hour), now.Add(2*time.Hour))

	loader := &mockLoader{cert: initialCert}
	issuer := &mockIssuer{certs: []*tls.Certificate{newCert}}

	cr, err := New(issuer, loader, &Config{
		PreValidationChecks: false,
		RetirementTimeout: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	events := make(map[string]int)
	cr.SetEventHandler(func(event RotationEvent) {
		events[event.Type]++
	})

	active := cr.ActiveCertificate()
	originalID := active.ID

	mc := &mockConn{}
	release := cr.TrackConnection(originalID, mc, mc.Close)
	_ = release

	if err := cr.ForceRenew(); err != nil {
		t.Fatalf("ForceRenew() error = %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	if !mc.closed.Load() {
		t.Error("Connection should be force-closed after retirement timeout")
	}

	if events["CERT_FORCE_RETIRED"] != 1 {
		t.Errorf("CERT_FORCE_RETIRED events = %d, want 1", events["CERT_FORCE_RETIRED"])
	}

	retiringCerts := cr.RetiringCertificates()
	if len(retiringCerts) != 0 {
		t.Errorf("Retiring certificates count after force retire = %d, want 0", len(retiringCerts))
	}
}

func TestClose(t *testing.T) {
	loader := &mockLoader{cert: generateSelfSignedCert(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))}
	issuer := &mockIssuer{}
	cr, err := New(issuer, loader, &Config{PreValidationChecks: false})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	active := cr.ActiveCertificate()
	mc := &mockConn{}
	_ = cr.TrackConnection(active.ID, mc, mc.Close)

	if err := cr.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	if !mc.closed.Load() {
		t.Error("Connection should be closed on rotator close")
	}

	if err := cr.Close(); err != nil {
		t.Errorf("Second Close() error = %v (expected no error)", err)
	}
}

func TestClose_AfterClose(t *testing.T) {
	loader := &mockLoader{cert: generateSelfSignedCert(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))}
	issuer := &mockIssuer{}
	cr, err := New(issuer, loader, &Config{PreValidationChecks: false})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	cr.Close()

	err = cr.ForceRenew()
	if err != ErrRotatorClosed {
		t.Errorf("ForceRenew() after close error = %v, want %v", err, ErrRotatorClosed)
	}
}

func TestMonitorLoop(t *testing.T) {
	now := time.Now()
	initialCert := generateSelfSignedCert(now.Add(-time.Hour), now.Add(10*24*time.Hour))
	newCert := generateSelfSignedCert(now.Add(-time.Hour), now.Add(40*24*time.Hour))

	loader := &mockLoader{cert: initialCert}
	issuer := &mockIssuer{certs: []*tls.Certificate{newCert}}

	monitorCalled := make(chan bool, 1)
	cr, err := New(issuer, loader, &Config{
		CheckInterval:       50 * time.Millisecond,
		RenewalBuffer:       30 * 24 * time.Hour,
		RetirementTimeout: 10 * time.Millisecond,
		PreValidationChecks: false,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	cr.clock = func() time.Time { return now }

	cr.SetEventHandler(func(event RotationEvent) {
		if event.Type == "RENEWAL_TRIGGERED" {
			select {
			case monitorCalled <- true:
			default:
			}
		}
	})

	select {
	case <-monitorCalled:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Monitor did not trigger renewal")
	}

	time.Sleep(100 * time.Millisecond)

	if issuer.CallCount() < 1 {
		t.Errorf("Issuer call count = %d, want >= 1", issuer.CallCount())
	}
}

func TestConcurrentAccess(t *testing.T) {
	now := time.Now()
	initialCert := generateSelfSignedCert(now.Add(-time.Hour), now.Add(24*time.Hour))

	loader := &mockLoader{cert: initialCert}
	issuer := &mockIssuer{}

	cr, err := New(issuer, loader, &Config{
		CheckInterval:       10 * time.Millisecond,
		RenewalBuffer:       30 * 24 * time.Hour,
		PreValidationChecks: false,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	cr.clock = func() time.Time { return now }

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				active := cr.ActiveCertificate()
				if active == nil {
					errors <- fmt.Errorf("goroutine %d: ActiveCertificate() returned nil", id)
					return
				}
				_, err := cr.GetCertificate(nil)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d: GetCertificate() error = %v", id, err)
					return
				}

				mc := &mockConn{}
				release := cr.TrackConnection(active.ID, mc, mc.Close)

				_ = cr.NeedsRenewal()
				_ = cr.TimeUntilExpiry()
				_ = cr.TimeUntilRenewal()
				_ = cr.ActiveConnections(active.ID)
				_ = cr.RetiringCertificates()

				release()

				time.Sleep(1 * time.Millisecond)
			}
		}(g)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

func TestPendingCertificate(t *testing.T) {
	now := time.Now()
	initialCert := generateSelfSignedCert(now.Add(-time.Hour), now.Add(time.Hour))
	newCert := generateSelfSignedCert(now.Add(-time.Hour), now.Add(2*time.Hour))

	loader := &mockLoader{cert: initialCert}
	issuer := &mockIssuer{certs: []*tls.Certificate{newCert}}

	cr, err := New(issuer, loader, &Config{PreValidationChecks: false})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	if cr.PendingCertificate() != nil {
		t.Error("PendingCertificate() should be nil initially")
	}

	cr.certMu.Lock()
	pendingInfo, err := cr.validateAndCreateInfo(newCert)
	if err != nil {
		cr.certMu.Unlock()
		t.Fatalf("validateAndCreateInfo() error = %v", err)
	}
	cr.pendingCert = pendingInfo
	cr.certMu.Unlock()

	pending := cr.PendingCertificate()
	if pending == nil {
		t.Fatal("PendingCertificate() should not be nil after setting")
	}
	if pending.Status != CertStatusPending {
		t.Errorf("Pending cert status = %v, want PENDING", pending.Status)
	}
}

func TestEventHandler(t *testing.T) {
	now := time.Now()
	initialCert := generateSelfSignedCert(now.Add(-time.Hour), now.Add(time.Hour))
	newCert := generateSelfSignedCert(now.Add(-time.Hour), now.Add(2*time.Hour))

	loader := &mockLoader{cert: initialCert}
	issuer := &mockIssuer{certs: []*tls.Certificate{newCert}}

	cr, err := New(issuer, loader, &Config{
		PreValidationChecks: false,
		RetirementTimeout: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	cr.clock = func() time.Time { return now }

	var mu sync.Mutex
	events := make([]RotationEvent, 0)
	cr.SetEventHandler(func(event RotationEvent) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, event)
	})

	if err := cr.ForceRenew(); err != nil {
		t.Fatalf("ForceRenew() error = %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(events) < 3 {
		t.Errorf("Event count = %d, want >= 3", len(events))
	}

	eventTypes := make(map[string]bool)
	for _, e := range events {
		eventTypes[e.Type] = true
	}

	requiredEvents := []string{"CERT_PENDING", "CERT_ACTIVATED", "CERT_RETIRING"}
	for _, et := range requiredEvents {
		if !eventTypes[et] {
			t.Errorf("Missing event: %s", et)
		}
	}
}

func TestZeroConfigValues(t *testing.T) {
	loader := &mockLoader{cert: generateSelfSignedCert(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))}
	issuer := &mockIssuer{}

	cfg := &Config{
		CheckInterval:       0,
		RenewalBuffer: 0,
		RetirementTimeout: 0,
		PreValidationChecks: false,
	}

	cr, err := New(issuer, loader, cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	if cr.config.CheckInterval != DefaultConfig().CheckInterval {
		t.Errorf("CheckInterval should default to %v, got %v",
			DefaultConfig().CheckInterval, cr.config.CheckInterval)
	}
	if cr.config.RenewalBuffer != DefaultConfig().RenewalBuffer {
		t.Errorf("RenewalBuffer should default to %v, got %v",
			DefaultConfig().RenewalBuffer, cr.config.RenewalBuffer)
	}
	if cr.config.RetirementTimeout != DefaultConfig().RetirementTimeout {
		t.Errorf("RetirementTimeout should default to %v, got %v",
			DefaultConfig().RetirementTimeout, cr.config.RetirementTimeout)
	}
}

func TestActiveConnections_NonExistentCert(t *testing.T) {
	loader := &mockLoader{cert: generateSelfSignedCert(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))}
	issuer := &mockIssuer{}
	cr, err := New(issuer, loader, &Config{PreValidationChecks: false})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	if count := cr.ActiveConnections("non-existent-cert-id"); count != 0 {
		t.Errorf("ActiveConnections(non-existent) = %d, want 0", count)
	}
}

func TestCloseConnections_NonExistentCert(t *testing.T) {
	loader := &mockLoader{cert: generateSelfSignedCert(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))}
	issuer := &mockIssuer{}
	cr, err := New(issuer, loader, &Config{PreValidationChecks: false})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	cr.CloseConnections("non-existent-cert-id")
}

func TestRetiringCerts_Empty(t *testing.T) {
	loader := &mockLoader{cert: generateSelfSignedCert(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))}
	issuer := &mockIssuer{}
	cr, err := New(issuer, loader, &Config{PreValidationChecks: false})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	retiring := cr.RetiringCertificates()
	if len(retiring) != 0 {
		t.Errorf("RetiringCertificates() count = %d, want 0", len(retiring))
	}
}

func TestNeedsRenewal_NoActive(t *testing.T) {
	loader := &mockLoader{cert: generateSelfSignedCert(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))}
	issuer := &mockIssuer{}
	cr, err := New(issuer, loader, &Config{PreValidationChecks: false})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	cr.activeCert.Store(nil)

	if cr.NeedsRenewal() {
		t.Error("NeedsRenewal() should be false with no active cert")
	}
}

func TestTimeUntilExpiry_NoActive(t *testing.T) {
	loader := &mockLoader{cert: generateSelfSignedCert(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))}
	issuer := &mockIssuer{}
	cr, err := New(issuer, loader, &Config{PreValidationChecks: false})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	cr.activeCert.Store(nil)

	if cr.TimeUntilExpiry() != 0 {
		t.Error("TimeUntilExpiry() should be 0 with no active cert")
	}
}

func TestTimeUntilRenewal_NoActive(t *testing.T) {
	loader := &mockLoader{cert: generateSelfSignedCert(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))}
	issuer := &mockIssuer{}
	cr, err := New(issuer, loader, &Config{PreValidationChecks: false})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	cr.activeCert.Store(nil)

	if cr.TimeUntilRenewal() != 0 {
		t.Error("TimeUntilRenewal() should be 0 with no active cert")
	}
}

func TestGetCertificate_NoActive(t *testing.T) {
	loader := &mockLoader{cert: generateSelfSignedCert(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))}
	issuer := &mockIssuer{}
	cr, err := New(issuer, loader, &Config{PreValidationChecks: false})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	cr.activeCert.Store(nil)

	_, err = cr.GetCertificate(nil)
	if err != ErrNoActiveCert {
		t.Errorf("GetCertificate() error = %v, want %v", err, ErrNoActiveCert)
	}
}

func TestCertificateInfoFields(t *testing.T) {
	now := time.Now()
	notBefore := now.Add(-time.Hour).Truncate(time.Second)
	notAfter := now.Add(time.Hour).Truncate(time.Second)
	cert := generateSelfSignedCert(notBefore, notAfter)

	loader := &mockLoader{cert: cert}
	issuer := &mockIssuer{}
	cr, err := New(issuer, loader, &Config{PreValidationChecks: false})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	info := cr.ActiveCertificate()
	if info == nil {
		t.Fatal("ActiveCertificate() returned nil")
	}

	if info.Status != CertStatusActive {
		t.Errorf("Status = %v, want ACTIVE", info.Status)
	}
	if !info.NotBefore.Equal(notBefore) {
		t.Errorf("NotBefore = %v, want %v", info.NotBefore, notBefore)
	}
	if !info.NotAfter.Equal(notAfter) {
		t.Errorf("NotAfter = %v, want %v", info.NotAfter, notAfter)
	}
	if info.ID == "" {
		t.Error("ID should not be empty")
	}
	if info.Serial == "" {
		t.Error("Serial should not be empty")
	}
}

func TestVerifyCertificateChain_NoLeafParsed(t *testing.T) {
	chainCert, _, rootPool, err := generateCertificateChain()
	if err != nil {
		t.Fatalf("generateCertificateChain() error = %v", err)
	}

	cfg := &Config{
		RootCAs: rootPool,
		PreValidationChecks: true,
	}
	loader := &mockLoader{cert: chainCert}
	issuer := &mockIssuer{}

	cr, err := New(issuer, loader, cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	certWithoutLeaf := &tls.Certificate{
		Certificate: chainCert.Certificate,
		PrivateKey:  chainCert.PrivateKey,
	}

	if err := cr.VerifyCertificateChain(certWithoutLeaf); err != nil {
		t.Errorf("VerifyCertificateChain() without pre-parsed leaf error = %v", err)
	}
}

func TestVerifyCertificateChain_InvalidIntermediate(t *testing.T) {
	now := time.Now()

	rootCert, rootKey, err := generateTestCertificate(
		now.Add(-time.Hour), now.Add(10*365*24*time.Hour), true, nil, nil,
	)
	if err != nil {
		t.Fatalf("generate root error = %v", err)
	}

	leafCert, _, err := generateTestCertificate(
		now.Add(-time.Hour), now.Add(365*24*time.Hour), false, rootCert.Leaf, rootKey,
	)
	if err != nil {
		t.Fatalf("generate leaf error = %v", err)
	}

	badIntermediate, _, err := generateTestCertificate(
		now.Add(-time.Hour), now.Add(-365*24*time.Hour), true, nil, nil,
	)
	if err != nil {
		t.Fatalf("generate bad intermediate error = %v", err)
	}

	chainCert := &tls.Certificate{
		Certificate: [][]byte{
			leafCert.Certificate[0],
			badIntermediate.Certificate[0],
			rootCert.Certificate[0],
		},
		PrivateKey: leafCert.PrivateKey,
	}

	rootPool := x509.NewCertPool()
	rootPool.AddCert(rootCert.Leaf)

	cfg := &Config{
		RootCAs: rootPool,
		PreValidationChecks: false,
	}
	loader := &mockLoader{cert: generateSelfSignedCert(now.Add(-time.Hour), now.Add(time.Hour))}
	issuer := &mockIssuer{}

	cr, err := New(issuer, loader, cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	err = cr.VerifyCertificateChain(chainCert)
	if err == nil {
		t.Error("VerifyCertificateChain() with invalid intermediate chain should fail")
	}
}

func TestTrackConnection_ReleaseAfterClose(t *testing.T) {
	loader := &mockLoader{cert: generateSelfSignedCert(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))}
	issuer := &mockIssuer{}
	cr, err := New(issuer, loader, &Config{PreValidationChecks: false})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	active := cr.ActiveCertificate()
	certID := active.ID

	mc := &mockConn{}
	release := cr.TrackConnection(certID, mc, mc.Close)

	cr.Close()

	release()

	if count := cr.ActiveConnections(certID); count != 0 {
		t.Errorf("ActiveConnections() after close and release = %d, want 0", count)
	}
}

func TestMultipleRenewals(t *testing.T) {
	now := time.Now()
	initialCert := generateSelfSignedCert(now.Add(-time.Hour), now.Add(time.Hour))

	certs := make([]*tls.Certificate, 5)
	for i := 0; i < 5; i++ {
		certs[i] = generateSelfSignedCert(
			now.Add(-time.Hour),
			now.Add(time.Duration(i+2)*time.Hour),
		)
	}

	loader := &mockLoader{cert: initialCert}
	issuer := &mockIssuer{certs: certs}

	cr, err := New(issuer, loader, &Config{
		PreValidationChecks: false,
		RetirementTimeout: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer cr.Close()

	cr.clock = func() time.Time { return now }

	originalID := cr.ActiveCertificate().ID

	for i := 0; i < 5; i++ {
		if err := cr.ForceRenew(); err != nil {
			t.Fatalf("ForceRenew() #%d error = %v", i+1, err)
		}

		newActive := cr.ActiveCertificate()
		if newActive.ID == originalID {
			t.Errorf("Renewal #%d: cert ID should change", i+1)
		}

		time.Sleep(200 * time.Millisecond)

		retiring := cr.RetiringCertificates()
		if len(retiring) != 0 {
			t.Errorf("Renewal #%d: retiring certs count = %d, want 0", i+1, len(retiring))
		}
	}

	if issuer.CallCount() != 5 {
		t.Errorf("Issuer call count = %d, want 5", issuer.CallCount())
	}
}
