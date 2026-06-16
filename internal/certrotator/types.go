package certrotator

import (
	"crypto/tls"
	"crypto/x509"
	"time"
)

type CertStatus int

const (
	CertStatusActive CertStatus = iota
	CertStatusPending
	CertStatusRetiring
	CertStatusRetired
)

func (s CertStatus) String() string {
	switch s {
	case CertStatusActive:
		return "ACTIVE"
	case CertStatusPending:
		return "PENDING"
	case CertStatusRetiring:
		return "RETIRING"
	case CertStatusRetired:
		return "RETIRED"
	default:
		return "UNKNOWN"
	}
}

type CertificateInfo struct {
	ID          string
	Certificate *x509.Certificate
	TLSCert     *tls.Certificate
	Status      CertStatus
	NotBefore   time.Time
	NotAfter    time.Time
	Issuer      string
	Subject     string
	Serial      string
	ActivatedAt time.Time
	RetiredAt   time.Time
}

type CertificateIssuer interface {
	IssueCertificate() (*tls.Certificate, error)
}

type CertificateLoader interface {
	LoadCertificate() (*tls.Certificate, error)
}

type ConnectionTracker interface {
	TrackConnection(certID string) func()
	ActiveConnections(certID string) int
	CloseConnections(certID string)
}

type RotationEvent struct {
	Type      string
	Timestamp time.Time
	CertID    string
	Details   string
}

type EventHandler func(event RotationEvent)
