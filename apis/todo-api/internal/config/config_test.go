package config

import (
	"log/slog"
	"reflect"
	"testing"
	"time"
)

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestLoad(t *testing.T) {
	defaults := Config{
		Port: 8080, LogLevel: "info", CORSOrigins: []string{"*"}, RateLimitRPS: 20, RateLimitBurst: 40,
		ShutdownTimeout: 10 * time.Second,
	}

	tests := []struct {
		name    string
		env     map[string]string
		mutate  func(*Config)
		wantErr bool
	}{
		{name: "defaults", env: nil, mutate: func(*Config) {}},
		{name: "custom port and level", env: map[string]string{"PORT": "9000", "LOG_LEVEL": "debug"}, mutate: func(c *Config) { c.Port = 9000; c.LogLevel = "debug" }},
		{name: "log level is case insensitive", env: map[string]string{"LOG_LEVEL": "WARN"}, mutate: func(c *Config) { c.LogLevel = "warn" }},
		{name: "surrounding spaces are ignored", env: map[string]string{"PORT": " 3000 "}, mutate: func(c *Config) { c.Port = 3000 }},
		{name: "api keys", env: map[string]string{"TODO_API_KEYS": " one, two ,,three "}, mutate: func(c *Config) { c.APIKeys = []string{"one", "two", "three"} }},
		{name: "cors origins", env: map[string]string{"CORS_ORIGINS": "https://a.example.com, https://b.example.com"}, mutate: func(c *Config) { c.CORSOrigins = []string{"https://a.example.com", "https://b.example.com"} }},
		{name: "blank cors origins keep the default", env: map[string]string{"CORS_ORIGINS": " , "}, mutate: func(*Config) {}},
		{name: "rate limit", env: map[string]string{"RATE_LIMIT_RPS": "2.5", "RATE_LIMIT_BURST": "7"}, mutate: func(c *Config) { c.RateLimitRPS = 2.5; c.RateLimitBurst = 7 }},
		{name: "shutdown timeout", env: map[string]string{"SHUTDOWN_TIMEOUT": "1m30s"}, mutate: func(c *Config) { c.ShutdownTimeout = 90 * time.Second }},
		{name: "shutdown timeout without unit", env: map[string]string{"SHUTDOWN_TIMEOUT": "30"}, wantErr: true},
		{name: "shutdown timeout zero", env: map[string]string{"SHUTDOWN_TIMEOUT": "0s"}, wantErr: true},
		{name: "shutdown timeout negative", env: map[string]string{"SHUTDOWN_TIMEOUT": "-5s"}, wantErr: true},
		{name: "port not a number", env: map[string]string{"PORT": "abc"}, wantErr: true},
		{name: "port too low", env: map[string]string{"PORT": "0"}, wantErr: true},
		{name: "port too high", env: map[string]string{"PORT": "70000"}, wantErr: true},
		{name: "unknown log level", env: map[string]string{"LOG_LEVEL": "trace"}, wantErr: true},
		{name: "rps not a number", env: map[string]string{"RATE_LIMIT_RPS": "fast"}, wantErr: true},
		{name: "rps zero", env: map[string]string{"RATE_LIMIT_RPS": "0"}, wantErr: true},
		{name: "burst zero", env: map[string]string{"RATE_LIMIT_BURST": "0"}, wantErr: true},
		{name: "burst not an integer", env: map[string]string{"RATE_LIMIT_BURST": "1.5"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Load(env(tt.env))
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			want := defaults
			tt.mutate(&want)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Load() = %+v, want %+v", got, want)
			}
		})
	}
}

func TestAddr(t *testing.T) {
	if got := (Config{Port: 8081}).Addr(); got != ":8081" {
		t.Errorf("Addr() = %q, want %q", got, ":8081")
	}
}

func TestSlogLevel(t *testing.T) {
	tests := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
		"":      slog.LevelInfo,
	}
	for level, want := range tests {
		if got := (Config{LogLevel: level}).SlogLevel(); got != want {
			t.Errorf("SlogLevel(%q) = %v, want %v", level, got, want)
		}
	}
}
