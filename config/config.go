// Package config provides configuration management for gascity.
// It handles loading and validating configuration from environment variables
// and configuration files.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration values for the gascity application.
type Config struct {
	// Server configuration
	ServerHost string
	ServerPort int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// Database configuration
	DatabaseURL      string
	DatabaseMaxConns int
	DatabaseTimeout  time.Duration

	// Gas pricing configuration
	GasAPIEndpoint  string
	GasAPIKey       string
	GasPollInterval time.Duration

	// Logging
	LogLevel  string
	LogFormat string

	// Environment
	Environment string
}

// Load reads configuration from environment variables and returns a populated Config.
// It returns an error if any required configuration values are missing or invalid.
func Load() (*Config, error) {
	cfg := &Config{
		// Defaults
		ServerHost:       getEnv("SERVER_HOST", "0.0.0.0"),
		ServerPort:       getEnvAsInt("SERVER_PORT", 8080),
		ReadTimeout:      getEnvAsDuration("READ_TIMEOUT", 30*time.Second),
		WriteTimeout:     getEnvAsDuration("WRITE_TIMEOUT", 30*time.Second),
		DatabaseMaxConns: getEnvAsInt("DATABASE_MAX_CONNS", 10),
		DatabaseTimeout:  getEnvAsDuration("DATABASE_TIMEOUT", 10*time.Second),
		GasPollInterval:  getEnvAsDuration("GAS_POLL_INTERVAL", 15*time.Second),
		LogLevel:         getEnv("LOG_LEVEL", "info"),
		LogFormat:        getEnv("LOG_FORMAT", "json"),
		Environment:      getEnv("ENVIRONMENT", "development"),
	}

	// Required fields
	cfg.DatabaseURL = os.Getenv("DATABASE_URL")
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}

	cfg.GasAPIEndpoint = os.Getenv("GAS_API_ENDPOINT")
	if cfg.GasAPIEndpoint == "" {
		return nil, fmt.Errorf("GAS_API_ENDPOINT environment variable is required")
	}

	cfg.GasAPIKey = os.Getenv("GAS_API_KEY")
	if cfg.GasAPIKey == "" {
		return nil, fmt.Errorf("GAS_API_KEY environment variable is required")
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// validate checks that all configuration values are within acceptable ranges.
func (c *Config) validate() error {
	if c.ServerPort < 1 || c.ServerPort > 65535 {
		return fmt.Errorf("SERVER_PORT must be between 1 and 65535, got %d", c.ServerPort)
	}
	if c.DatabaseMaxConns < 1 {
		return fmt.Errorf("DATABASE_MAX_CONNS must be at least 1, got %d", c.DatabaseMaxConns)
	}
	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[c.LogLevel] {
		return fmt.Errorf("LOG_LEVEL must be one of debug, info, warn, error; got %q", c.LogLevel)
	}
	return nil
}

// Addr returns the formatted server address string.
func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.ServerHost, c.ServerPort)
}

// IsDevelopment returns true if the environment is set to development.
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

func getEnv(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return defaultValue
}
