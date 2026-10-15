package config

import "testing"

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    Config
		wantErr bool
	}{
		{name: "defaults", env: nil, want: Config{Port: 8080, LogLevel: "info"}},
		{name: "custom values", env: map[string]string{"PORT": "9000", "LOG_LEVEL": "debug"}, want: Config{Port: 9000, LogLevel: "debug"}},
		{name: "log level is case insensitive", env: map[string]string{"LOG_LEVEL": "WARN"}, want: Config{Port: 8080, LogLevel: "warn"}},
		{name: "surrounding spaces are ignored", env: map[string]string{"PORT": " 3000 "}, want: Config{Port: 3000, LogLevel: "info"}},
		{name: "port not a number", env: map[string]string{"PORT": "abc"}, wantErr: true},
		{name: "port too low", env: map[string]string{"PORT": "0"}, wantErr: true},
		{name: "port too high", env: map[string]string{"PORT": "70000"}, wantErr: true},
		{name: "unknown log level", env: map[string]string{"LOG_LEVEL": "trace"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Load(env(tt.env))
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Load() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestAddr(t *testing.T) {
	if got := (Config{Port: 8081}).Addr(); got != ":8081" {
		t.Errorf("Addr() = %q, want %q", got, ":8081")
	}
}
