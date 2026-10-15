// Package validator checks structs against rules declared in `validate` tags:
//
//	type SignUp struct {
//		Name  string `validate:"required,min=3,max=40"`
//		Email string `validate:"required,email"`
//		Age   int    `validate:"min=18"`
//		Plan  string `validate:"oneof=free pro team"`
//	}
//
// Rules are separated by commas. Available rules: required, min=N, max=N
// (string length in runes, collection length, or numeric value), oneof=a b c
// and email. A field that is empty and not required is not checked against
// its other rules. Nested structs are validated recursively.
package validator

import (
	"errors"
	"fmt"
	"net/mail"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ErrNotStruct is returned when the value is neither a struct nor a non-nil
// pointer to one.
var ErrNotStruct = errors.New("validator: value must be a struct or a non-nil pointer to a struct")

// FieldError describes one violated rule.
type FieldError struct {
	Field string // dotted path, such as "Address.City"
	Rule  string
	Param string
}

func (e FieldError) Error() string {
	if e.Param != "" {
		return fmt.Sprintf("%s: failed %s=%s", e.Field, e.Rule, e.Param)
	}
	return fmt.Sprintf("%s: failed %s", e.Field, e.Rule)
}

// Errors is the list of all violations found by Validate.
type Errors []FieldError

func (e Errors) Error() string {
	msgs := make([]string, len(e))
	for i, fe := range e {
		msgs[i] = fe.Error()
	}
	return strings.Join(msgs, "; ")
}

// Validate checks v and returns nil or an Errors value listing every
// violation. It panics on an unknown rule or a malformed rule parameter, since
// those are programming errors in the tags.
func Validate(v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer && !rv.IsNil() {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return ErrNotStruct
	}
	var errs Errors
	walk(rv, "", &errs)
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func walk(rv reflect.Value, path string, errs *Errors) {
	t := rv.Type()
	for i := 0; i < t.NumField(); i++ {
		field, fv := t.Field(i), rv.Field(i)
		if !field.IsExported() {
			continue
		}
		name := path + field.Name
		if tag := field.Tag.Get("validate"); tag != "" && tag != "-" {
			checkField(fv, name, strings.Split(tag, ","), errs)
		}
		if field.Type.Kind() == reflect.Struct {
			walk(fv, name+".", errs)
		}
	}
}

func checkField(fv reflect.Value, name string, rules []string, errs *Errors) {
	if fv.IsZero() {
		for _, rule := range rules {
			if strings.TrimSpace(rule) == "required" {
				*errs = append(*errs, FieldError{Field: name, Rule: "required"})
			}
		}
		return // empty and optional: nothing else to check
	}
	for _, rule := range rules {
		rule, param, _ := strings.Cut(strings.TrimSpace(rule), "=")
		if rule != "required" && !satisfies(fv, rule, param) {
			*errs = append(*errs, FieldError{Field: name, Rule: rule, Param: param})
		}
	}
}

func satisfies(fv reflect.Value, rule, param string) bool {
	for fv.Kind() == reflect.Pointer {
		fv = fv.Elem()
	}
	switch rule {
	case "min", "max":
		limit, err := strconv.ParseFloat(param, 64)
		if err != nil {
			panic(fmt.Sprintf("validator: invalid parameter %q for rule %s", param, rule))
		}
		n, ok := measure(fv)
		return ok && (rule == "min" && n >= limit || rule == "max" && n <= limit)
	case "oneof":
		return slices.Contains(strings.Fields(param), fmt.Sprint(fv.Interface()))
	case "email":
		if fv.Kind() != reflect.String {
			return false
		}
		addr, err := mail.ParseAddress(fv.String())
		return err == nil && addr.Address == fv.String()
	}
	panic(fmt.Sprintf("validator: unknown rule %q", rule))
}

// measure returns the quantity that min and max compare against.
func measure(fv reflect.Value) (float64, bool) {
	switch fv.Kind() {
	case reflect.String:
		return float64(utf8.RuneCountInString(fv.String())), true
	case reflect.Slice, reflect.Map, reflect.Array:
		return float64(fv.Len()), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(fv.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(fv.Uint()), true
	case reflect.Float32, reflect.Float64:
		return fv.Float(), true
	}
	return 0, false
}
