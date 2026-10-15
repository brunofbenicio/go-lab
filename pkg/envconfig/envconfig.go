// Package envconfig fills a struct from environment variables, driven by
// struct tags:
//
//	type Config struct {
//		Port    int           `env:"PORT" default:"8080"`
//		Debug   bool          `env:"DEBUG"`
//		Timeout time.Duration `env:"TIMEOUT" default:"5s"`
//		Hosts   []string      `env:"HOSTS"`
//		DSN     string        `env:"DSN" required:"true"`
//	}
//
// Supported field types are strings, booleans, signed and unsigned integers,
// floats, time.Duration, slices of those (comma-separated) and nested structs.
package envconfig

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// ErrNotStructPointer is returned when the destination of Load is not a
// non-nil pointer to a struct.
var ErrNotStructPointer = errors.New("envconfig: destination must be a non-nil pointer to a struct")

var durationType = reflect.TypeOf(time.Duration(0))

type options struct {
	prefix string
	lookup func(string) (string, bool)
}

// Option customizes Load.
type Option func(*options)

// WithPrefix prepends prefix to every variable name, for example "APP_".
func WithPrefix(prefix string) Option { return func(o *options) { o.prefix = prefix } }

// WithLookup replaces the variable source (default: os.LookupEnv), which is
// handy in tests.
func WithLookup(fn func(string) (string, bool)) Option { return func(o *options) { o.lookup = fn } }

// Load populates the struct pointed to by dst. A variable that is unset or
// empty falls back to the field's default tag; without a default, the field
// keeps its current value, or the call fails if the field is tagged
// required:"true". All problems are collected and returned together.
//
// A nested struct field may carry an env tag that acts as a prefix for the
// variables inside it. Fields tagged env:"-" and unexported fields are
// skipped.
func Load(dst any, opts ...Option) error {
	o := options{lookup: os.LookupEnv}
	for _, opt := range opts {
		opt(&o)
	}
	v := reflect.ValueOf(dst)
	if v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		return ErrNotStructPointer
	}
	var errs []error
	o.fill(v.Elem(), o.prefix, &errs)
	return errors.Join(errs...)
}

func (o *options) fill(v reflect.Value, prefix string, errs *[]error) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field, fv := t.Field(i), v.Field(i)
		name, tagged := field.Tag.Lookup("env")
		if !field.IsExported() || name == "-" {
			continue
		}
		if field.Type.Kind() == reflect.Struct {
			o.fill(fv, prefix+name, errs)
			continue
		}
		if !tagged {
			continue
		}
		key := prefix + name
		raw, ok := o.lookup(key)
		if !ok || raw == "" {
			if raw, ok = field.Tag.Lookup("default"); !ok {
				if req, _ := strconv.ParseBool(field.Tag.Get("required")); req {
					*errs = append(*errs, fmt.Errorf("envconfig: %s is required", key))
				}
				continue
			}
		}
		if err := setValue(fv, raw); err != nil {
			*errs = append(*errs, fmt.Errorf("envconfig: %s: %w", key, err))
		}
	}
}

func setValue(v reflect.Value, raw string) error {
	if v.Type() == durationType {
		d, err := time.ParseDuration(raw)
		if err != nil {
			return err
		}
		v.SetInt(int64(d))
		return nil
	}
	switch v.Kind() {
	case reflect.String:
		v.SetString(raw)
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		v.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(raw, 10, v.Type().Bits())
		if err != nil {
			return err
		}
		v.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(raw, 10, v.Type().Bits())
		if err != nil {
			return err
		}
		v.SetUint(n)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(raw, v.Type().Bits())
		if err != nil {
			return err
		}
		v.SetFloat(f)
	case reflect.Slice:
		parts := strings.Split(raw, ",")
		s := reflect.MakeSlice(v.Type(), len(parts), len(parts))
		for i, part := range parts {
			if err := setValue(s.Index(i), strings.TrimSpace(part)); err != nil {
				return fmt.Errorf("element %d: %w", i, err)
			}
		}
		v.Set(s)
	default:
		return fmt.Errorf("unsupported type %s", v.Type())
	}
	return nil
}
