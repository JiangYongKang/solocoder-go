package fallback

import "time"

type ChainConfig struct {
	Recovery RecoveryConfig
}

func DefaultChainConfig() *ChainConfig {
	return &ChainConfig{
		Recovery: RecoveryConfig{
			Mode:                  RecoveryModePassive,
			CheckInterval:         5 * time.Second,
			ProbeSuccessThreshold: 3,
			ProbeFailureThreshold: 1,
			WarmUpDuration:        10 * time.Second,
			PassiveSuccessWindow:  30 * time.Second,
			PassiveSuccessCount:   5,
		},
	}
}

func DefaultRecoveryConfig() RecoveryConfig {
	return RecoveryConfig{
		Mode:                  RecoveryModePassive,
		CheckInterval:         5 * time.Second,
		ProbeSuccessThreshold: 3,
		ProbeFailureThreshold: 1,
		WarmUpDuration:        10 * time.Second,
		PassiveSuccessWindow:  30 * time.Second,
		PassiveSuccessCount:   5,
	}
}
