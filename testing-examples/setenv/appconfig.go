// Package appconfig builds an application configuration from environment
// variables, the usual way twelve-factor services are configured.
//
// Variables:
//
//	APP_NAME     required, non-empty
//	APP_PORT     optional, 1-65535, default 8080
//	APP_DEBUG    optional, any value accepted by strconv.ParseBool, default false
//	APP_TIMEOUT  optional, a time.ParseDuration string, default 5s
package appconfig

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config is the parsed configuration.
type Config struct {
	Name    string
	Port    int
	Debug   bool
	Timeout time.Duration
}

// Errors returned (wrapped) by FromEnv.
var (
	ErrMissing = errors.New("appconfig: required variable not set")
	ErrInvalid = errors.New("appconfig: invalid value")
)

// FromEnv reads the configuration from the process environment.
func FromEnv() (Config, error) {
	cfg := Config{Port: 8080, Timeout: 5 * time.Second}

	name, ok := os.LookupEnv("APP_NAME")
	if !ok {
		return Config{}, fmt.Errorf("%w: APP_NAME", ErrMissing)
	}
	if name == "" {
		return Config{}, fmt.Errorf("%w: APP_NAME is empty", ErrInvalid)
	}
	cfg.Name = name

	if v, ok := os.LookupEnv("APP_PORT"); ok {
		port, err := strconv.Atoi(v)
		if err != nil || port < 1 || port > 65535 {
			return Config{}, fmt.Errorf("%w: APP_PORT=%q", ErrInvalid, v)
		}
		cfg.Port = port
	}
	if v, ok := os.LookupEnv("APP_DEBUG"); ok {
		debug, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("%w: APP_DEBUG=%q", ErrInvalid, v)
		}
		cfg.Debug = debug
	}
	if v, ok := os.LookupEnv("APP_TIMEOUT"); ok {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return Config{}, fmt.Errorf("%w: APP_TIMEOUT=%q", ErrInvalid, v)
		}
		cfg.Timeout = d
	}
	return cfg, nil
}
