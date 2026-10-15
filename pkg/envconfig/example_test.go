package envconfig_test

import (
	"fmt"
	"time"

	"github.com/brunofbenicio/go-lab/pkg/envconfig"
)

func ExampleLoad() {
	type Config struct {
		Port    int           `env:"PORT" default:"8080"`
		Timeout time.Duration `env:"TIMEOUT" default:"5s"`
		Hosts   []string      `env:"HOSTS"`
	}
	vars := map[string]string{"HOSTS": "a.example.com,b.example.com"}

	var c Config
	err := envconfig.Load(&c, envconfig.WithLookup(func(k string) (string, bool) {
		v, ok := vars[k]
		return v, ok
	}))
	fmt.Println(c.Port, c.Timeout, c.Hosts, err)
	// Output: 8080 5s [a.example.com b.example.com] <nil>
}
