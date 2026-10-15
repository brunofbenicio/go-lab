package appconfig

import (
	"errors"
	"os"
	"testing"
	"time"
)

// unsetenv guarantees a variable is *absent* for the duration of the test.
// t.Setenv has no "unset" mode, but it remembers the original state and
// restores it on cleanup, so calling os.Unsetenv right after it is safe.
func unsetenv(t *testing.T, keys ...string) {
	t.Helper()
	for _, k := range keys {
		t.Setenv(k, "")
		if err := os.Unsetenv(k); err != nil {
			t.Fatalf("Unsetenv(%s): %v", k, err)
		}
	}
}

// cleanEnv starts every test from a known state, whatever the developer's or
// the CI's environment looks like.
func cleanEnv(t *testing.T) {
	t.Helper()
	unsetenv(t, "APP_NAME", "APP_PORT", "APP_DEBUG", "APP_TIMEOUT")
}

func TestDefaults(t *testing.T) {
	cleanEnv(t)
	t.Setenv("APP_NAME", "demo")

	got, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	want := Config{Name: "demo", Port: 8080, Debug: false, Timeout: 5 * time.Second}
	if got != want {
		t.Errorf("FromEnv() = %+v, want %+v", got, want)
	}
}

func TestOverrides(t *testing.T) {
	cleanEnv(t)
	t.Setenv("APP_NAME", "demo")
	t.Setenv("APP_PORT", "9090")
	t.Setenv("APP_DEBUG", "true")
	t.Setenv("APP_TIMEOUT", "1m30s")

	got, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	want := Config{Name: "demo", Port: 9090, Debug: true, Timeout: 90 * time.Second}
	if got != want {
		t.Errorf("FromEnv() = %+v, want %+v", got, want)
	}
}

func TestInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		key  string
		val  string
	}{
		{"empty name", "APP_NAME", ""},
		{"port is not a number", "APP_PORT", "http"},
		{"port too large", "APP_PORT", "70000"},
		{"port zero", "APP_PORT", "0"},
		{"bad bool", "APP_DEBUG", "maybe"},
		{"duration without unit", "APP_TIMEOUT", "30"},
		{"negative duration", "APP_TIMEOUT", "-1s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanEnv(t)
			t.Setenv("APP_NAME", "demo")
			t.Setenv(tt.key, tt.val) // overrides the valid value for APP_NAME too

			if _, err := FromEnv(); !errors.Is(err, ErrInvalid) {
				t.Errorf("FromEnv error = %v, want ErrInvalid", err)
			}
		})
	}
}

// Unset and empty are different situations, which is why the code uses
// os.LookupEnv; the test needs both states, hence unsetenv above.
func TestMissingName(t *testing.T) {
	cleanEnv(t)
	if _, err := FromEnv(); !errors.Is(err, ErrMissing) {
		t.Errorf("FromEnv error = %v, want ErrMissing", err)
	}
}

// t.Setenv restores the previous value when the test (or subtest) ends, so a
// change never leaks into the tests that run after it.
func TestSetenvIsRestored(t *testing.T) {
	t.Setenv("APP_PORT", "1111")

	t.Run("inner", func(t *testing.T) {
		t.Setenv("APP_PORT", "2222")
		if got := os.Getenv("APP_PORT"); got != "2222" {
			t.Errorf("inside the subtest APP_PORT = %q, want 2222", got)
		}
	})

	if got := os.Getenv("APP_PORT"); got != "1111" {
		t.Errorf("after the subtest APP_PORT = %q, want 1111", got)
	}
}
