package certrotator

import "fmt"

var (
	ErrCertExpired               = fmt.Errorf("certificate has expired")
	ErrCertNotYetValid           = fmt.Errorf("certificate is not yet valid")
	ErrCertChainIncomplete       = fmt.Errorf("certificate chain is incomplete")
	ErrCertRootUntrusted         = fmt.Errorf("root certificate is not trusted")
	ErrCertSignatureInvalid      = fmt.Errorf("certificate signature is invalid")
	ErrCertIssuerMismatch        = fmt.Errorf("certificate issuer mismatch")
	ErrCertValidationFailed      = fmt.Errorf("certificate validation failed")
	ErrCertPreValidationFailed   = fmt.Errorf("certificate pre-validation failed")
	ErrCertAlreadyActive         = fmt.Errorf("certificate is already active")
	ErrNoActiveCert              = fmt.Errorf("no active certificate")
	ErrNoPendingCert             = fmt.Errorf("no pending certificate")
	ErrInvalidConfig             = fmt.Errorf("invalid configuration")
	ErrRotatorClosed             = fmt.Errorf("certificate rotator is closed")
	ErrIssuerNil                 = fmt.Errorf("certificate issuer is nil")
	ErrLoaderNil                 = fmt.Errorf("certificate loader is nil")
	ErrLoadCertificateFailed     = fmt.Errorf("failed to load certificate")
	ErrRetirementTimeout         = fmt.Errorf("old certificate retirement timed out")
)
