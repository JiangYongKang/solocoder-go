package certrotator

import (
	"crypto/x509"
	"time"
)

type Config struct {
	CheckInterval       time.Duration
	RenewalBuffer       time.Duration
	RetirementTimeout   time.Duration
	PreValidationChecks bool
	RootCAs             *x509.CertPool
	IntermediateCAs     *x509.CertPool
	EnableLogging       bool
}

func DefaultConfig() *Config {
	return &Config{
		CheckInterval:       1 * time.Hour,
		RenewalBuffer:       30 * 24 * time.Hour,
		RetirementTimeout:   5 * time.Minute,
		PreValidationChecks: true,
		RootCAs:             nil,
		IntermediateCAs:     nil,
		EnableLogging:       true,
	}
}
