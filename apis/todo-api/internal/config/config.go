// Package config loads the server settings from environment variables.
package config

import (
	"fmt"
	"strconv"
	"strings"
)

// Config holds the runtime settings of the server.
type Config struct {
	// Port is the TCP port the HTTP server listens on.
	Port int
	// LogLevel is one of debug, info, warn or error.
	LogLevel string
}

const (
	defaultPort     = 8080
	defaultLogLevel = "info"
)

// Load reads the settings through getenv (usually os.Getenv), applying
// defaults for anything that is unset and validating what is set.
//
//	PORT       TCP port, 1-65535 (default 8080)
//	LOG_LEVEL  debug, info, warn or error (default info)
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{Port: defaultPort, LogLevel: defaultLogLevel}

	if raw := strings.TrimSpace(getenv("PORT")); raw != "" {
		port, err := strconv.Atoi(raw)
		if err != nil || port < 1 || port > 65535 {
			return Config{}, fmt.Errorf("config: PORT must be a number between 1 and 65535, got %q", raw)
		}
		cfg.Port = port
	}

	if raw := strings.ToLower(strings.TrimSpace(getenv("LOG_LEVEL"))); raw != "" {
		switch raw {
		case "debug", "info", "warn", "error":
			cfg.LogLevel = raw
		default:
			return Config{}, fmt.Errorf("config: LOG_LEVEL must be debug, info, warn or error, got %q", raw)
		}
	}

	return cfg, nil
}

// Addr returns the listen address in host:port form for net/http.
func (c Config) Addr() string {
	return fmt.Sprintf(":%d", c.Port)
}
