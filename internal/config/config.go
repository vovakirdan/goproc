package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"
)

const (
	defaultLivenessInterval   = 10 * time.Second
	defaultLastSeenInterval   = 30 * time.Second
	envLivenessInterval       = "GOPROC_LIVENESS_INTERVAL"
	envLastSeenUpdateInterval = "GOPROC_LAST_SEEN_INTERVAL"
)

// Config aggregates tunable timeouts/intervals for the daemon.
type Config struct {
	LivenessInterval       time.Duration
	LastSeenUpdateInterval time.Duration
}

// Load builds a Config from an optional JSON file path plus environment overrides.
func Load(path string) (Config, error) {
	cfg := Config{
		LivenessInterval:       defaultLivenessInterval,
		LastSeenUpdateInterval: defaultLastSeenInterval,
	}

	if path != "" {
		fileCfg, err := loadFromFile(path)
		if err != nil {
			return cfg, fmt.Errorf("load config %s: %w", path, err)
		}
		if fileCfg.LivenessInterval != 0 {
			cfg.LivenessInterval = fileCfg.LivenessInterval
		}
		if fileCfg.LastSeenUpdateInterval != 0 {
			cfg.LastSeenUpdateInterval = fileCfg.LastSeenUpdateInterval
		}
	}

	applyEnvOverrides(&cfg)
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv(envLivenessInterval); v != "" {
		if dur, err := time.ParseDuration(v); err == nil && dur > 0 {
			cfg.LivenessInterval = dur
		} else if err != nil {
			log.Printf("invalid %s value %q: %v", envLivenessInterval, v, err)
		}
	}

	if v := os.Getenv(envLastSeenUpdateInterval); v != "" {
		if dur, err := time.ParseDuration(v); err == nil && dur > 0 {
			cfg.LastSeenUpdateInterval = dur
		} else if err != nil {
			log.Printf("invalid %s value %q: %v", envLastSeenUpdateInterval, v, err)
		}
	}
}

type fileConfig struct {
	LivenessInterval       string `json:"liveness_interval"`
	LastSeenUpdateInterval string `json:"last_seen_interval"`
}

func loadFromFile(path string) (Config, error) {
	var cfg Config

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}

	var raw fileConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return cfg, err
	}

	if raw.LivenessInterval != "" {
		dur, err := time.ParseDuration(raw.LivenessInterval)
		if err != nil {
			return cfg, fmt.Errorf("parse liveness_interval: %w", err)
		}
		if dur <= 0 {
			return cfg, errors.New("liveness_interval must be > 0")
		}
		cfg.LivenessInterval = dur
	}
	if raw.LastSeenUpdateInterval != "" {
		dur, err := time.ParseDuration(raw.LastSeenUpdateInterval)
		if err != nil {
			return cfg, fmt.Errorf("parse last_seen_interval: %w", err)
		}
		if dur <= 0 {
			return cfg, errors.New("last_seen_interval must be > 0")
		}
		cfg.LastSeenUpdateInterval = dur
	}

	return cfg, nil
}
