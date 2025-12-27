package config

import "time"

type Config struct {
	MaxSteps      int
	StepTimeout   time.Duration
	TotalTimeout  time.Duration
	Headless      bool
	SlowMo        float64
	MaxRetries    int
	RetryDelay    time.Duration
}

func NewConfig() *Config {
	return &Config{
		MaxSteps:     100,
		StepTimeout:  30 * time.Second,
		TotalTimeout: 10 * time.Minute,
		Headless:     false,
		SlowMo:       100,
		MaxRetries:   3,
		RetryDelay:   2 * time.Second,
	}
}