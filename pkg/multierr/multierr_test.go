package multierr

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"testing"
)

var (
	errA = errors.New("a")
	errB = errors.New("b")
	errC = errors.New("c")
)

func TestCombine(t *testing.T) {
	tests := []struct {
		name    string
		in      []error
		wantNil bool
		wantMsg string
		wantLen int
	}{
		{"nothing", nil, true, "", 0},
		{"only nils", []error{nil, nil}, true, "", 0},
		{"single error is returned as is", []error{nil, errA, nil}, false, "a", 1},
		{"two errors", []error{errA, errB}, false, "2 errors occurred:\n\t* a\n\t* b", 2},
		{"nested combinations are flattened", []error{Combine(errA, errB), errC}, false,
			"3 errors occurred:\n\t* a\n\t* b\n\t* c", 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Combine(tt.in...)
			if tt.wantNil {
				if err != nil {
					t.Fatalf("Combine = %v, want nil", err)
				}
				return
			}
			if err.Error() != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", err.Error(), tt.wantMsg)
			}
			if got := len(Errors(err)); got != tt.wantLen {
				t.Errorf("len(Errors) = %d, want %d", got, tt.wantLen)
			}
		})
	}
	if Combine(errA) != errA {
		t.Error("a lone error must be returned unchanged (same identity)")
	}
}

func TestIsAndAs(t *testing.T) {
	pathErr := &fs.PathError{Op: "open", Path: "x", Err: fs.ErrNotExist}
	err := Combine(errA, fmt.Errorf("wrapped: %w", pathErr), errC)

	for _, target := range []error{errA, errC, fs.ErrNotExist} {
		if !errors.Is(err, target) {
			t.Errorf("errors.Is(err, %v) = false, want true", target)
		}
	}
	if errors.Is(err, errB) {
		t.Error("errors.Is(err, errB) = true, want false")
	}
	var pe *fs.PathError
	if !errors.As(err, &pe) || pe.Path != "x" {
		t.Errorf("errors.As failed to find the *fs.PathError: %v", pe)
	}
}

func TestAppend(t *testing.T) {
	tests := []struct {
		name string
		err  error
		more []error
		want []error
	}{
		{"nil and nil", nil, []error{nil}, nil},
		{"nil base", nil, []error{errA}, []error{errA}},
		{"nil addition", errA, []error{nil}, []error{errA}},
		{"accumulates in order", errA, []error{errB, errC}, []error{errA, errB, errC}},
		{"flattens a combined base", Combine(errA, errB), []error{errC}, []error{errA, errB, errC}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Errors(Append(tt.err, tt.more...)); !slices.Equal(got, tt.want) {
				t.Errorf("Errors(Append) = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAppendIntoWithDeferredCleanup(t *testing.T) {
	work := func(mainErr, closeErr error) (err error) {
		defer func() { AppendInto(&err, closeErr) }()
		return mainErr
	}
	tests := []struct {
		name     string
		main, cl error
		want     []error
	}{
		{"both succeed", nil, nil, nil},
		{"only main fails", errA, nil, []error{errA}},
		{"only cleanup fails", nil, errB, []error{errB}},
		{"both fail and neither is lost", errA, errB, []error{errA, errB}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Errors(work(tt.main, tt.cl)); !slices.Equal(got, tt.want) {
				t.Errorf("errors = %v, want %v", got, tt.want)
			}
		})
	}
	var dst error
	if AppendInto(&dst, nil) || !AppendInto(&dst, errA) {
		t.Error("AppendInto should report whether the error was non-nil")
	}
}

func TestErrorsReturnsCopy(t *testing.T) {
	err := Combine(errA, errB)
	list := Errors(err)
	list[0] = errC
	if !errors.Is(err, errA) {
		t.Error("mutating the slice from Errors must not change the combined error")
	}
	if Errors(nil) != nil {
		t.Error("Errors(nil) should be nil")
	}
}
