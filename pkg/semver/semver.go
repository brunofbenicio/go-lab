// Package semver parses, formats and compares semantic versions as defined by
// semver.org 2.0.0: MAJOR.MINOR.PATCH with optional pre-release and build
// metadata, such as "1.4.0-rc.1+build.5". A leading "v" is accepted when
// parsing.
package semver

import (
	"cmp"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrInvalid wraps every error returned by Parse.
var ErrInvalid = errors.New("semver: invalid version")

// Version is a parsed semantic version.
type Version struct {
	Major, Minor, Patch uint64
	Pre                 []string // pre-release identifiers, e.g. ["rc", "1"]
	Build               string   // build metadata; ignored when comparing
}

// Parse decodes s, rejecting leading zeros in numeric parts, empty
// identifiers and characters outside [0-9A-Za-z-].
func Parse(s string) (Version, error) {
	raw := s
	s = strings.TrimPrefix(s, "v")
	var v Version
	if core, build, ok := strings.Cut(s, "+"); ok {
		if !validIdentifiers(build, false) {
			return Version{}, fmt.Errorf("%w %q: bad build metadata", ErrInvalid, raw)
		}
		s, v.Build = core, build
	}
	if core, pre, ok := strings.Cut(s, "-"); ok {
		if !validIdentifiers(pre, true) {
			return Version{}, fmt.Errorf("%w %q: bad pre-release", ErrInvalid, raw)
		}
		s, v.Pre = core, strings.Split(pre, ".")
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("%w %q: want MAJOR.MINOR.PATCH", ErrInvalid, raw)
	}
	var nums [3]uint64
	for i, p := range parts {
		if len(p) > 1 && p[0] == '0' || !isDigits(p) {
			return Version{}, fmt.Errorf("%w %q: bad number %q", ErrInvalid, raw, p)
		}
		n, err := strconv.ParseUint(p, 10, 64)
		if err != nil {
			return Version{}, fmt.Errorf("%w %q: bad number %q", ErrInvalid, raw, p)
		}
		nums[i] = n
	}
	v.Major, v.Minor, v.Patch = nums[0], nums[1], nums[2]
	return v, nil
}

// MustParse is like Parse but panics on error. Use it for constants.
func MustParse(s string) Version {
	v, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return v
}

// String formats v without the leading "v".
func (v Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if len(v.Pre) > 0 {
		s += "-" + strings.Join(v.Pre, ".")
	}
	if v.Build != "" {
		s += "+" + v.Build
	}
	return s
}

// Compare returns -1, 0 or +1 according to semantic version precedence. Build
// metadata is ignored, and a pre-release sorts before its release.
func (v Version) Compare(o Version) int {
	if c := cmp.Or(cmp.Compare(v.Major, o.Major), cmp.Compare(v.Minor, o.Minor), cmp.Compare(v.Patch, o.Patch)); c != 0 {
		return c
	}
	return comparePre(v.Pre, o.Pre)
}

// Less reports whether v has lower precedence than o.
func (v Version) Less(o Version) bool { return v.Compare(o) < 0 }

// IncMajor returns the next major version (pre-release and build cleared).
func (v Version) IncMajor() Version { return Version{Major: v.Major + 1} }

// IncMinor returns the next minor version (pre-release and build cleared).
func (v Version) IncMinor() Version { return Version{Major: v.Major, Minor: v.Minor + 1} }

// IncPatch returns the next patch version (pre-release and build cleared).
func (v Version) IncPatch() Version {
	return Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch + 1}
}

func comparePre(a, b []string) int {
	switch {
	case len(a) == 0 && len(b) == 0:
		return 0
	case len(a) == 0:
		return 1 // a release is greater than any of its pre-releases
	case len(b) == 0:
		return -1
	}
	for i := 0; i < len(a) && i < len(b); i++ {
		if c := compareIdentifier(a[i], b[i]); c != 0 {
			return c
		}
	}
	return cmp.Compare(len(a), len(b)) // more identifiers = higher precedence
}

func compareIdentifier(a, b string) int {
	an, bn := isDigits(a), isDigits(b)
	switch {
	case an && bn: // numeric: compare as numbers (no leading zeros allowed)
		return cmp.Or(cmp.Compare(len(a), len(b)), strings.Compare(a, b))
	case an:
		return -1 // numeric identifiers sort before alphanumeric ones
	case bn:
		return 1
	}
	return strings.Compare(a, b)
}

func validIdentifiers(s string, pre bool) bool {
	for _, id := range strings.Split(s, ".") {
		if id == "" {
			return false
		}
		for _, r := range id {
			if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '-') {
				return false
			}
		}
		if pre && isDigits(id) && len(id) > 1 && id[0] == '0' {
			return false
		}
	}
	return true
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
