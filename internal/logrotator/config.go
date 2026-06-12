package logrotator

import "time"

type RotationMode int

const (
	RotationModeNone RotationMode = iota
	RotationModeSize
	RotationModeHourly
	RotationModeDaily
)

type Config struct {
	LevelFileMap   map[Level]string
	RotationMode   RotationMode
	MaxFileSize    int64
	MaxBackups     int
	Compress       bool
	TTL            time.Duration
	CleanInterval  time.Duration
	FileDateFormat string
}

func DefaultConfig() *Config {
	return &Config{
		LevelFileMap: map[Level]string{
			LevelDebug: "app.log",
			LevelInfo:  "app.log",
			LevelWarn:  "app.log",
			LevelError: "app.log",
		},
		RotationMode:   RotationModeSize,
		MaxFileSize:    100 * 1024 * 1024,
		MaxBackups:     10,
		Compress:       true,
		TTL:            7 * 24 * time.Hour,
		CleanInterval:  time.Hour,
		FileDateFormat: "2006-01-02",
	}
}
