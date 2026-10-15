package envconfig

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

type database struct {
	Host string `env:"HOST" default:"localhost"`
	Port int    `env:"PORT" default:"5432"`
}

type config struct {
	Name     string        `env:"NAME" required:"true"`
	Debug    bool          `env:"DEBUG"`
	Timeout  time.Duration `env:"TIMEOUT" default:"5s"`
	Ratio    float64       `env:"RATIO"`
	Workers  uint8         `env:"WORKERS"`
	Tags     []string      `env:"TAGS"`
	IDs      []int         `env:"IDS"`
	DB       database      `env:"DB_"`
	Skipped  string        `env:"-"`
	NoTag    string
	internal string `env:"INTERNAL"`
}

func env(pairs ...string) func(string) (string, bool) {
	m := map[string]string{}
	for i := 0; i < len(pairs); i += 2 {
		m[pairs[i]] = pairs[i+1]
	}
	return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
}

func TestLoadSuccess(t *testing.T) {
	c := config{Skipped: "keep", internal: "keep"}
	err := Load(&c, WithLookup(env(
		"NAME", "svc", "DEBUG", "true", "TIMEOUT", "1m30s", "RATIO", "0.25", "WORKERS", "8",
		"TAGS", "a, b ,c", "IDS", "1,2,3", "DB_HOST", "db.example.com", "SKIPPED", "ignored", "INTERNAL", "ignored",
	)))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Name != "svc" || !c.Debug || c.Timeout != 90*time.Second || c.Ratio != 0.25 || c.Workers != 8 {
		t.Errorf("scalars not loaded correctly: %+v", c)
	}
	if !slices.Equal(c.Tags, []string{"a", "b", "c"}) || !slices.Equal(c.IDs, []int{1, 2, 3}) {
		t.Errorf("slices not loaded correctly: tags=%v ids=%v", c.Tags, c.IDs)
	}
	if c.DB.Host != "db.example.com" || c.DB.Port != 5432 {
		t.Errorf("nested struct = %+v, want host from env and default port", c.DB)
	}
	if c.Skipped != "keep" || c.internal != "keep" || c.NoTag != "" {
		t.Errorf("skipped/unexported/untagged fields must be left alone: %+v", c)
	}
}

func TestLoadDefaultsAndPrefix(t *testing.T) {
	tests := []struct {
		name   string
		opts   []Option
		lookup func(string) (string, bool)
		check  func(c config) bool
	}{
		{"default applies when unset", nil, env("NAME", "x"), func(c config) bool { return c.Timeout == 5*time.Second }},
		{"empty value counts as unset", nil, env("NAME", "x", "TIMEOUT", ""), func(c config) bool { return c.Timeout == 5*time.Second }},
		{"prefix is prepended", []Option{WithPrefix("APP_")}, env("APP_NAME", "x", "APP_DB_PORT", "6000"),
			func(c config) bool { return c.Name == "x" && c.DB.Port == 6000 }},
		{"unprefixed names are ignored under a prefix", []Option{WithPrefix("APP_")}, env("APP_NAME", "x", "DEBUG", "true"),
			func(c config) bool { return !c.Debug }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c config
			if err := Load(&c, append(tt.opts, WithLookup(tt.lookup))...); err != nil {
				t.Fatalf("Load: %v", err)
			}
			if !tt.check(c) {
				t.Errorf("unexpected result: %+v", c)
			}
		})
	}
}

func TestLoadErrors(t *testing.T) {
	tests := []struct {
		name     string
		lookup   func(string) (string, bool)
		contains []string
	}{
		{"missing required", env(), []string{"NAME is required"}},
		{"invalid bool", env("NAME", "x", "DEBUG", "maybe"), []string{"DEBUG"}},
		{"uint8 overflow", env("NAME", "x", "WORKERS", "300"), []string{"WORKERS"}},
		{"invalid duration", env("NAME", "x", "TIMEOUT", "soon"), []string{"TIMEOUT"}},
		{"bad slice element", env("NAME", "x", "IDS", "1,x,3"), []string{"IDS", "element 1"}},
		{"all problems are reported together", env("DEBUG", "nope", "RATIO", "abc"), []string{"NAME is required", "DEBUG", "RATIO"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c config
			err := Load(&c, WithLookup(tt.lookup))
			if err == nil {
				t.Fatal("Load = nil, want an error")
			}
			for _, want := range tt.contains {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not mention %q", err, want)
				}
			}
		})
	}
}

func TestLoadRejectsInvalidDestination(t *testing.T) {
	var c config
	var nilPtr *config
	n := 5
	for name, dst := range map[string]any{"value": c, "nil pointer": nilPtr, "pointer to int": &n, "nil": nil} {
		if err := Load(dst); !errors.Is(err, ErrNotStructPointer) {
			t.Errorf("%s: err = %v, want ErrNotStructPointer", name, err)
		}
	}
}

func TestLoadUnsupportedType(t *testing.T) {
	var c struct {
		M map[string]string `env:"M"`
	}
	err := Load(&c, WithLookup(env("M", "a=b")))
	if err == nil || !strings.Contains(err.Error(), "unsupported type") {
		t.Errorf("err = %v, want unsupported type", err)
	}
}

func TestLoadFromProcessEnvironment(t *testing.T) {
	t.Setenv("ENVCONFIG_TEST_PORT", "9090")
	var c struct {
		Port int `env:"ENVCONFIG_TEST_PORT"`
	}
	if err := Load(&c); err != nil || c.Port != 9090 {
		t.Errorf("Load = %v, Port = %d; want nil, 9090", err, c.Port)
	}
}
