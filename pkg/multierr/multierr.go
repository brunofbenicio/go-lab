// Package multierr combines several errors into one. Unlike errors.Join, it
// flattens nested combinations, returns a lone error unchanged, ignores nil
// values, prints a count and offers helpers for the common "append the error
// of a deferred cleanup" pattern. The combined error works with errors.Is and
// errors.As.
package multierr

import (
	"fmt"
	"slices"
	"strings"
)

type multiError struct{ errs []error }

func (m *multiError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d errors occurred:", len(m.errs))
	for _, err := range m.errs {
		b.WriteString("\n\t* ")
		b.WriteString(err.Error())
	}
	return b.String()
}

// Unwrap lets errors.Is and errors.As inspect every combined error.
func (m *multiError) Unwrap() []error { return slices.Clone(m.errs) }

// Combine merges errs into a single error. Nil values are dropped and
// combined errors produced by this package are flattened. It returns nil if
// nothing is left, and the error itself if exactly one is.
func Combine(errs ...error) error {
	var all []error
	for _, err := range errs {
		switch e := err.(type) {
		case nil:
		case *multiError:
			all = append(all, e.errs...)
		default:
			all = append(all, err)
		}
	}
	switch len(all) {
	case 0:
		return nil
	case 1:
		return all[0]
	}
	return &multiError{errs: all}
}

// Append adds more to err, with the same rules as Combine.
func Append(err error, more ...error) error {
	return Combine(append([]error{err}, more...)...)
}

// AppendInto appends err to *into and reports whether err was non-nil. It is
// meant for deferred cleanups that must not hide the function's own error:
//
//	func work() (err error) {
//		f, _ := os.Open(name)
//		defer func() { multierr.AppendInto(&err, f.Close()) }()
//		...
//	}
func AppendInto(into *error, err error) bool {
	if err == nil {
		return false
	}
	*into = Append(*into, err)
	return true
}

// Errors returns the individual errors inside err: none for nil, err itself
// for an ordinary error, and every member of a combined one.
func Errors(err error) []error {
	switch e := err.(type) {
	case nil:
		return nil
	case *multiError:
		return slices.Clone(e.errs)
	}
	return []error{err}
}
