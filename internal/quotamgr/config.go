package quotamgr

type Config struct {
	DefaultQuota     Quota
	DefaultLimitMode LimitMode
	SoftThreshold    float64
	AlertCallback    AlertCallback
}

func DefaultConfig() *Config {
	return &Config{
		DefaultQuota: Quota{
			CPU:         4.0,
			MemoryMB:    2048,
			Concurrency: 100,
		},
		DefaultLimitMode: LimitModeHard,
		SoftThreshold:    1.5,
	}
}

func DefaultQuota() Quota {
	return Quota{
		CPU:         4.0,
		MemoryMB:    2048,
		Concurrency: 100,
	}
}
