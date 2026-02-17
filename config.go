package main

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Supervisor SupervisorConfig `yaml:"supervisor"`
	Services   []ServiceConfig  `yaml:"services"`
}

type SupervisorConfig struct {
	MaxRestarts  int           `yaml:"maxRestarts"`
	RestartDelay time.Duration `yaml:"restartDelay"`
	PortTimeout  time.Duration `yaml:"portTimeout"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse yaml: %w", err)
	}

	if cfg.Supervisor.MaxRestarts == 0 {
		cfg.Supervisor.MaxRestarts = 5
	}
	if cfg.Supervisor.RestartDelay == 0 {
		cfg.Supervisor.RestartDelay = 2 * time.Second
	}
	if cfg.Supervisor.PortTimeout == 0 {
		cfg.Supervisor.PortTimeout = 30 * time.Second
	}

	return &cfg, nil
}
