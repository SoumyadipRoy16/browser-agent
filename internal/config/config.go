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
	EnableRecovery bool
}

func NewConfig() *Config {
	return &Config{
		MaxSteps:      150,  // Increased for complex tasks
		StepTimeout:   45 * time.Second,
		TotalTimeout:  20 * time.Minute,  // Increased timeout
		Headless:      false,
		SlowMo:        100,
		MaxRetries:    3,
		RetryDelay:    2 * time.Second,
		EnableRecovery: true,
	}
}