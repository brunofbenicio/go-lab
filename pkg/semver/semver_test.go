package semver

import (
	"errors"
	"slices"
	"testing"
)

func TestParseValid(t *testing.T) {
	tests := []struct {
		in   string
		want Version
		str  string
	}{
		{"1.2.3", Version{Major: 1, Minor: 2, Patch: 3}, "1.2.3"},
		{"v1.2.3", Version{Major: 1, Minor: 2, Patch: 3}, "1.2.3"},
		{"0.0.0", Version{}, "0.0.0"},
		{"10.20.30", Version{Major: 10, Minor: 20, Patch: 30}, "10.20.30"},
		{"1.0.0-alpha.1", Version{Major: 1, Pre: []string{"alpha", "1"}}, "1.0.0-alpha.1"},
		{"1.0.0-rc-1", Version{Major: 1, Pre: []string{"rc-1"}}, "1.0.0-rc-1"},
		{"1.0.0+build.5", Version{Major: 1, Build: "build.5"}, "1.0.0+build.5"},
		{"1.0.0-beta+exp.sha.5114f85", Version{Major: 1, Pre: []string{"beta"}, Build: "exp.sha.5114f85"}, "1.0.0-beta+exp.sha.5114f85"},
		{"1.0.0+001", Version{Major: 1, Build: "001"}, "1.0.0+001"}, // zeros are fine in build metadata
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := Parse(tt.in)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if got.Major != tt.want.Major || got.Minor != tt.want.Minor || got.Patch != tt.want.Patch ||
				!slices.Equal(got.Pre, tt.want.Pre) || got.Build != tt.want.Build {
				t.Errorf("Parse = %+v, want %+v", got, tt.want)
			}
			if got.String() != tt.str {
				t.Errorf("String() = %q, want %q", got.String(), tt.str)
			}
		})
	}
}

func TestParseInvalid(t *testing.T) {
	for _, in := range []string{
		"", "1", "1.2", "1.2.3.4", "a.b.c", "1.2.x", "01.2.3", "1.02.3", "1.2.03",
		"-1.2.3", "1.2.-3", "1.2.3-", "1.2.3+", "1.2.3-01", "1.2.3-a..b", "1.2.3-a_b",
		"1.2.3+a b", "v", "1.2.3-.", "99999999999999999999.0.0",
	} {
		if _, err := Parse(in); !errors.Is(err, ErrInvalid) {
			t.Errorf("Parse(%q) error = %v, want ErrInvalid", in, err)
		}
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "2.0.0", -1},
		{"2.1.0", "2.0.9", 1},
		{"1.2.10", "1.2.9", 1}, // numeric, not lexical
		{"1.0.0-alpha", "1.0.0", -1},
		{"1.0.0", "1.0.0-rc.1", 1},
		{"1.0.0+a", "1.0.0+b", 0}, // build metadata is ignored
		// The precedence chain from the specification, section 11.
		{"1.0.0-alpha", "1.0.0-alpha.1", -1},
		{"1.0.0-alpha.1", "1.0.0-alpha.beta", -1},
		{"1.0.0-alpha.beta", "1.0.0-beta", -1},
		{"1.0.0-beta", "1.0.0-beta.2", -1},
		{"1.0.0-beta.2", "1.0.0-beta.11", -1},
		{"1.0.0-beta.11", "1.0.0-rc.1", -1},
		{"1.0.0-rc.1", "1.0.0", -1},
	}
	for _, tt := range tests {
		t.Run(tt.a+" vs "+tt.b, func(t *testing.T) {
			a, b := MustParse(tt.a), MustParse(tt.b)
			if got := a.Compare(b); got != tt.want {
				t.Errorf("Compare = %d, want %d", got, tt.want)
			}
			if got := b.Compare(a); got != -tt.want {
				t.Errorf("reverse Compare = %d, want %d", got, -tt.want)
			}
			if a.Less(b) != (tt.want < 0) {
				t.Errorf("Less = %v with Compare %d", a.Less(b), tt.want)
			}
		})
	}
}

func TestSort(t *testing.T) {
	in := []string{"1.10.0", "1.2.0", "1.2.0-rc.1", "1.2.0-beta", "0.9.9", "1.2.0+meta"}
	vs := make([]Version, len(in))
	for i, s := range in {
		vs[i] = MustParse(s)
	}
	slices.SortStableFunc(vs, Version.Compare)

	got := make([]string, len(vs))
	for i, v := range vs {
		got[i] = v.String()
	}
	// "1.2.0" and "1.2.0+meta" have equal precedence, so a stable sort keeps
	// them in input order; and 1.10.0 sorts after 1.2.0 numerically.
	want := []string{"0.9.9", "1.2.0-beta", "1.2.0-rc.1", "1.2.0", "1.2.0+meta", "1.10.0"}
	if !slices.Equal(got, want) {
		t.Errorf("sorted = %v, want %v", got, want)
	}
}

func TestIncrements(t *testing.T) {
	v := MustParse("1.4.7-rc.1+build")
	if got := []string{v.IncMajor().String(), v.IncMinor().String(), v.IncPatch().String()}; !slices.Equal(got, []string{"2.0.0", "1.5.0", "1.4.8"}) {
		t.Errorf("IncMajor/IncMinor/IncPatch = %v", got)
	}
}

func TestMustParsePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("MustParse of invalid input should panic")
		}
	}()
	MustParse("nope")
}
