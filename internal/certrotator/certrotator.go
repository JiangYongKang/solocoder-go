package certrotator

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type trackedConn struct {
	certID string
	conn   interface{}
	close  func() error
}

type CertRotator struct {
	config *Config

	activeCert  atomic.Pointer[CertificateInfo]
	pendingCert *CertificateInfo
	retiringCerts map[string]*CertificateInfo

	certMu sync.RWMutex

	issuer  CertificateIssuer
	loader  CertificateLoader

	connections map[string]map[*trackedConn]struct{}
	connMu      sync.RWMutex

	ctx       context.Context
	cancel    context.CancelFunc
	closed    bool
	closeOnce sync.Once
	wg        sync.WaitGroup

	eventHandler EventHandler
	clock        func() time.Time
}

func New(issuer CertificateIssuer, loader CertificateLoader, config *Config) (*CertRotator, error) {
	if issuer == nil {
		return nil, ErrIssuerNil
	}
	if loader == nil {
		return nil, ErrLoaderNil
	}
	if config == nil {
		config = DefaultConfig()
	}
	if config.CheckInterval <= 0 {
		config.CheckInterval = DefaultConfig().CheckInterval
	}
	if config.RenewalBuffer <= 0 {
		config.RenewalBuffer = DefaultConfig().RenewalBuffer
	}
	if config.RetirementTimeout <= 0 {
		config.RetirementTimeout = DefaultConfig().RetirementTimeout
	}

	cr := &CertRotator{
		config:        config,
		issuer:        issuer,
		loader:        loader,
		retiringCerts: make(map[string]*CertificateInfo),
		connections:   make(map[string]map[*trackedConn]struct{}),
		clock:         time.Now,
	}
	cr.ctx, cr.cancel = context.WithCancel(context.Background())

	if err := cr.loadInitialCertificate(); err != nil {
		return nil, fmt.Errorf("load initial certificate failed: %w", err)
	}

	cr.startMonitor()

	return cr, nil
}

func (cr *CertRotator) loadInitialCertificate() error {
	tlsCert, err := cr.loader.LoadCertificate()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrLoadCertificateFailed, err)
	}

	certInfo, err := cr.validateAndCreateInfo(tlsCert)
	if err != nil {
		return err
	}

	certInfo.Status = CertStatusActive
	certInfo.ActivatedAt = cr.clock()
	cr.activeCert.Store(certInfo)

	cr.emitEvent("CERT_LOADED", certInfo.ID, "Initial certificate loaded successfully")
	return nil
}

func (cr *CertRotator) validateAndCreateInfo(tlsCert *tls.Certificate) (*CertificateInfo, error) {
	if len(tlsCert.Certificate) == 0 {
		return nil, fmt.Errorf("%w: no certificate data", ErrCertChainIncomplete)
	}

	leaf, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("parse leaf certificate failed: %w", err)
	}

	tlsCert.Leaf = leaf

	if cr.config.PreValidationChecks {
		if err := cr.VerifyCertificateChain(tlsCert); err != nil {
			return nil, err
		}
	}

	now := cr.clock()
	if now.After(leaf.NotAfter) {
		return nil, fmt.Errorf("%w: certificate expired at %v", ErrCertExpired, leaf.NotAfter)
	}
	if now.Before(leaf.NotBefore) {
		return nil, fmt.Errorf("%w: certificate not valid until %v", ErrCertNotYetValid, leaf.NotBefore)
	}

	certID := fmt.Sprintf("%x", leaf.SerialNumber.Bytes())

	return &CertificateInfo{
		ID:          certID,
		Certificate: leaf,
		TLSCert:     tlsCert,
		Status:      CertStatusPending,
		NotBefore:   leaf.NotBefore,
		NotAfter:    leaf.NotAfter,
		Issuer:      leaf.Issuer.String(),
		Subject:     leaf.Subject.String(),
		Serial:      leaf.SerialNumber.String(),
	}, nil
}

func (cr *CertRotator) VerifyCertificateChain(tlsCert *tls.Certificate) error {
	if len(tlsCert.Certificate) == 0 {
		return ErrCertChainIncomplete
	}

	leaf := tlsCert.Leaf
	if leaf == nil {
		var err error
		leaf, err = x509.ParseCertificate(tlsCert.Certificate[0])
		if err != nil {
			return fmt.Errorf("parse leaf certificate failed: %w", err)
		}
	}

	var chainIntermediates []*x509.Certificate
	for i := 1; i < len(tlsCert.Certificate); i++ {
		intermediate, err := x509.ParseCertificate(tlsCert.Certificate[i])
		if err != nil {
			return fmt.Errorf("parse intermediate certificate %d failed: %w", i, err)
		}
		chainIntermediates = append(chainIntermediates, intermediate)
	}

	var intermediates *x509.CertPool
	if cr.config.IntermediateCAs != nil {
		intermediates = cr.config.IntermediateCAs
		for _, cert := range chainIntermediates {
			intermediates.AddCert(cert)
		}
	} else {
		intermediates = x509.NewCertPool()
		for _, cert := range chainIntermediates {
			intermediates.AddCert(cert)
		}
	}

	now := cr.clock()
	if now.After(leaf.NotAfter) {
		return fmt.Errorf("%w: leaf certificate expired at %v", ErrCertExpired, leaf.NotAfter)
	}
	if now.Before(leaf.NotBefore) {
		return fmt.Errorf("%w: leaf certificate not valid until %v", ErrCertNotYetValid, leaf.NotBefore)
	}

	for i, intermediate := range chainIntermediates {
		if now.After(intermediate.NotAfter) {
			return fmt.Errorf("%w: intermediate certificate %d expired at %v", ErrCertExpired, i+1, intermediate.NotAfter)
		}
		if now.Before(intermediate.NotBefore) {
			return fmt.Errorf("%w: intermediate certificate %d not valid until %v", ErrCertNotYetValid, i+1, intermediate.NotBefore)
		}
	}

	verifyOpts := x509.VerifyOptions{
		Intermediates: intermediates,
		CurrentTime:   now,
	}

	if cr.config.RootCAs != nil {
		verifyOpts.Roots = cr.config.RootCAs
	}

	chains, err := leaf.Verify(verifyOpts)
	if err != nil {
		var certErr x509.CertificateInvalidError
		if errors.As(err, &certErr) {
			switch certErr.Reason {
			case x509.Expired:
				return fmt.Errorf("%w: %v", ErrCertExpired, err)
			case x509.NotAuthorizedToSign:
				return fmt.Errorf("%w: %v", ErrCertSignatureInvalid, err)
			case x509.CANotAuthorizedForThisName:
				return fmt.Errorf("%w: %v", ErrCertRootUntrusted, err)
			}
		}
		var unknownAuthErr x509.UnknownAuthorityError
		if errors.As(err, &unknownAuthErr) {
			return fmt.Errorf("%w: %v", ErrCertRootUntrusted, err)
		}
		return fmt.Errorf("%w: %v", ErrCertValidationFailed, err)
	}

	if len(chains) == 0 {
		return ErrCertChainIncomplete
	}

	return nil
}

func (cr *CertRotator) startMonitor() {
	cr.wg.Add(1)
	go cr.monitorLoop()
}

func (cr *CertRotator) monitorLoop() {
	defer cr.wg.Done()

	ticker := time.NewTicker(cr.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-cr.ctx.Done():
			return
		case <-ticker.C:
			cr.checkAndRenew()
		}
	}
}

func (cr *CertRotator) checkAndRenew() {
	cr.certMu.RLock()
	active := cr.activeCert.Load()
	cr.certMu.RUnlock()

	if active == nil {
		return
	}

	now := cr.clock()
	expiresAt := active.NotAfter
	renewalThreshold := expiresAt.Add(-cr.config.RenewalBuffer)

	if now.After(renewalThreshold) {
		cr.emitEvent("RENEWAL_TRIGGERED", active.ID,
			fmt.Sprintf("Certificate expires at %v, renewal threshold was at %v", expiresAt, renewalThreshold))

		if err := cr.renewCertificate(); err != nil {
			cr.emitEvent("RENEWAL_FAILED", active.ID, fmt.Sprintf("Renewal failed: %v", err))
			return
		}

		if err := cr.switchToPendingCert(); err != nil {
			cr.emitEvent("SWITCH_FAILED", active.ID, fmt.Sprintf("Switch to pending cert failed: %v", err))
		}
	}
}

func (cr *CertRotator) renewCertificate() error {
	if cr.isClosed() {
		return ErrRotatorClosed
	}

	tlsCert, err := cr.issuer.IssueCertificate()
	if err != nil {
		return fmt.Errorf("issue certificate failed: %w", err)
	}

	certInfo, err := cr.validateAndCreateInfo(tlsCert)
	if err != nil {
		return fmt.Errorf("new certificate validation failed: %w", err)
	}

	cr.certMu.Lock()
	cr.pendingCert = certInfo
	cr.certMu.Unlock()

	cr.emitEvent("CERT_PENDING", certInfo.ID, "New certificate issued and validated, pending activation")

	return nil
}

func (cr *CertRotator) switchToPendingCert() error {
	cr.certMu.Lock()
	defer cr.certMu.Unlock()

	if cr.pendingCert == nil {
		return ErrNoPendingCert
	}

	oldActive := cr.activeCert.Load()
	if oldActive != nil && oldActive.ID == cr.pendingCert.ID {
		return ErrCertAlreadyActive
	}

	newCert := cr.pendingCert
	newCert.Status = CertStatusActive
	newCert.ActivatedAt = cr.clock()

	cr.activeCert.Store(newCert)
	cr.pendingCert = nil

	cr.emitEvent("CERT_ACTIVATED", newCert.ID, "New certificate is now active")

	if oldActive != nil {
		cr.startRetirement(oldActive)
	}

	return nil
}

func (cr *CertRotator) startRetirement(oldCert *CertificateInfo) {
	oldCert.Status = CertStatusRetiring
	oldCert.RetiredAt = cr.clock()
	cr.retiringCerts[oldCert.ID] = oldCert

	cr.emitEvent("CERT_RETIRING", oldCert.ID,
		fmt.Sprintf("Old certificate entering retirement phase, waiting for active connections to close"))

	cr.wg.Add(1)
	go cr.waitForRetirement(oldCert)
}

func (cr *CertRotator) waitForRetirement(oldCert *CertificateInfo) {
	defer cr.wg.Done()

	deadline := cr.clock().Add(cr.config.RetirementTimeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-cr.ctx.Done():
			cr.forceRetire(oldCert)
			return
		case <-ticker.C:
			activeConns := cr.ActiveConnections(oldCert.ID)
			if activeConns == 0 {
				cr.completeRetirement(oldCert)
				return
			}
			if cr.clock().After(deadline) {
				cr.emitEvent("RETIREMENT_TIMEOUT", oldCert.ID,
					fmt.Sprintf("Retirement timed out after %v, force closing %d connections",
						cr.config.RetirementTimeout, activeConns))
				cr.forceRetire(oldCert)
				return
			}
		}
	}
}

func (cr *CertRotator) completeRetirement(oldCert *CertificateInfo) {
	cr.certMu.Lock()
	delete(cr.retiringCerts, oldCert.ID)
	cr.certMu.Unlock()

	oldCert.Status = CertStatusRetired

	cr.connMu.Lock()
	delete(cr.connections, oldCert.ID)
	cr.connMu.Unlock()

	cr.emitEvent("CERT_RETIRED", oldCert.ID,
		fmt.Sprintf("Old certificate retired successfully, all connections closed naturally"))
}

func (cr *CertRotator) forceRetire(oldCert *CertificateInfo) {
	cr.CloseConnections(oldCert.ID)

	cr.certMu.Lock()
	delete(cr.retiringCerts, oldCert.ID)
	cr.certMu.Unlock()

	oldCert.Status = CertStatusRetired

	cr.connMu.Lock()
	delete(cr.connections, oldCert.ID)
	cr.connMu.Unlock()

	cr.emitEvent("CERT_FORCE_RETIRED", oldCert.ID,
		fmt.Sprintf("Old certificate force retired due to timeout"))
}

func (cr *CertRotator) GetCertificate(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	active := cr.activeCert.Load()
	if active == nil {
		return nil, ErrNoActiveCert
	}
	return active.TLSCert, nil
}

func (cr *CertRotator) TrackConnection(certID string, conn interface{}, closeFn func() error) func() {
	tc := &trackedConn{
		certID: certID,
		conn:   conn,
		close:  closeFn,
	}

	cr.connMu.Lock()
	if _, exists := cr.connections[certID]; !exists {
		cr.connections[certID] = make(map[*trackedConn]struct{})
	}
	cr.connections[certID][tc] = struct{}{}
	cr.connMu.Unlock()

	return func() {
		cr.connMu.Lock()
		if conns, exists := cr.connections[certID]; exists {
			delete(conns, tc)
		}
		cr.connMu.Unlock()
	}
}

func (cr *CertRotator) ActiveConnections(certID string) int {
	cr.connMu.RLock()
	defer cr.connMu.RUnlock()
	if conns, exists := cr.connections[certID]; exists {
		return len(conns)
	}
	return 0
}

func (cr *CertRotator) CloseConnections(certID string) {
	cr.connMu.Lock()
	conns, exists := cr.connections[certID]
	if !exists {
		cr.connMu.Unlock()
		return
	}

	toClose := make([]*trackedConn, 0, len(conns))
	for tc := range conns {
		toClose = append(toClose, tc)
	}
	cr.connMu.Unlock()

	for _, tc := range toClose {
		if tc.close != nil {
			_ = tc.close()
		}
	}

	cr.connMu.Lock()
	delete(cr.connections, certID)
	cr.connMu.Unlock()
}

func (cr *CertRotator) ActiveCertificate() *CertificateInfo {
	return cr.activeCert.Load()
}

func (cr *CertRotator) PendingCertificate() *CertificateInfo {
	cr.certMu.RLock()
	defer cr.certMu.RUnlock()
	return cr.pendingCert
}

func (cr *CertRotator) RetiringCertificates() []*CertificateInfo {
	cr.certMu.RLock()
	defer cr.certMu.RUnlock()
	result := make([]*CertificateInfo, 0, len(cr.retiringCerts))
	for _, cert := range cr.retiringCerts {
		result = append(result, cert)
	}
	return result
}

func (cr *CertRotator) ForceRenew() error {
	return cr.renewCertificate()
}

func (cr *CertRotator) ForceSwitch() error {
	return cr.switchToPendingCert()
}

func (cr *CertRotator) SetEventHandler(handler EventHandler) {
	cr.eventHandler = handler
}

func (cr *CertRotator) emitEvent(eventType, certID, details string) {
	if cr.eventHandler != nil {
		cr.eventHandler(RotationEvent{
			Type:      eventType,
			Timestamp: cr.clock(),
			CertID:    certID,
			Details:   details,
		})
	}
}

func (cr *CertRotator) isClosed() bool {
	cr.certMu.RLock()
	defer cr.certMu.RUnlock()
	return cr.closed
}

func (cr *CertRotator) Close() error {
	var firstErr error

	cr.closeOnce.Do(func() {
		cr.certMu.Lock()
		cr.closed = true
		cr.certMu.Unlock()

		cr.cancel()

		cr.wg.Wait()

		cr.connMu.Lock()
		for certID, conns := range cr.connections {
			for tc := range conns {
				if tc.close != nil {
					if err := tc.close(); err != nil && firstErr == nil {
						firstErr = err
					}
				}
			}
			delete(cr.connections, certID)
		}
		cr.connMu.Unlock()
	})

	return firstErr
}

func (cr *CertRotator) NeedsRenewal() bool {
	active := cr.activeCert.Load()
	if active == nil {
		return false
	}
	now := cr.clock()
	renewalThreshold := active.NotAfter.Add(-cr.config.RenewalBuffer)
	return now.After(renewalThreshold)
}

func (cr *CertRotator) TimeUntilExpiry() time.Duration {
	active := cr.activeCert.Load()
	if active == nil {
		return 0
	}
	return active.NotAfter.Sub(cr.clock())
}

func (cr *CertRotator) TimeUntilRenewal() time.Duration {
	active := cr.activeCert.Load()
	if active == nil {
		return 0
	}
	renewalThreshold := active.NotAfter.Add(-cr.config.RenewalBuffer)
	return renewalThreshold.Sub(cr.clock())
}
