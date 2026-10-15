// Package config loads the server settings from environment variables.
package config

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
)

// Config holds the runtime settings of the server.
type Config struct {
	// Port is the TCP port the HTTP server listens on.
	Port int
	// LogLevel is one of debug, info, warn or error.
	LogLevel string
	// APIKeys are the credentials accepted for write operations. Empty means
	// authentication is disabled.
	APIKeys []string
	// CORSOrigins lists the origins allowed to call the API from a browser.
	CORSOrigins []string
	// RateLimitRPS is the sustained requests per second allowed per client.
	RateLimitRPS float64
	// RateLimitBurst is how many requests a client may send at once.
	RateLimitBurst int
}

const (
	defaultPort       = 8080
	defaultLogLevel   = "info"
	defaultRateRPS    = 20
	defaultRateBurst  = 40
	defaultCORSOrigin = "*"
)

// Load reads the settings through getenv (usually os.Getenv), applying
// defaults for anything that is unset and validating what is set.
//
//	PORT              TCP port, 1-65535 (default 8080)
//	LOG_LEVEL         debug, info, warn or error (default info)
//	TODO_API_KEYS     comma-separated API keys for write operations (default none)
//	CORS_ORIGINS      comma-separated allowed origins (default *)
//	RATE_LIMIT_RPS    requests per second per client, > 0 (default 20)
//	RATE_LIMIT_BURST  burst size per client, >= 1 (default 40)
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		Port:           defaultPort,
		LogLevel:       defaultLogLevel,
		CORSOrigins:    []string{defaultCORSOrigin},
		RateLimitRPS:   defaultRateRPS,
		RateLimitBurst: defaultRateBurst,
	}

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

	cfg.APIKeys = splitList(getenv("TODO_API_KEYS"))

	if origins := splitList(getenv("CORS_ORIGINS")); len(origins) > 0 {
		cfg.CORSOrigins = origins
	}

	if raw := strings.TrimSpace(getenv("RATE_LIMIT_RPS")); raw != "" {
		rps, err := strconv.ParseFloat(raw, 64)
		if err != nil || rps <= 0 {
			return Config{}, fmt.Errorf("config: RATE_LIMIT_RPS must be a number greater than 0, got %q", raw)
		}
		cfg.RateLimitRPS = rps
	}

	if raw := strings.TrimSpace(getenv("RATE_LIMIT_BURST")); raw != "" {
		burst, err := strconv.Atoi(raw)
		if err != nil || burst < 1 {
			return Config{}, fmt.Errorf("config: RATE_LIMIT_BURST must be an integer greater than or equal to 1, got %q", raw)
		}
		cfg.RateLimitBurst = burst
	}

	return cfg, nil
}

// Addr returns the listen address in host:port form for net/http.
func (c Config) Addr() string {
	return fmt.Sprintf(":%d", c.Port)
}

// SlogLevel converts LogLevel to the matching slog level.
func (c Config) SlogLevel() slog.Level {
	switch c.LogLevel {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// splitList splits a comma-separated value, trimming blanks and dropping empty items.
func splitList(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
