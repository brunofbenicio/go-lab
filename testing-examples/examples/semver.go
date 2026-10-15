// Package semver parses and compares semantic versions such as "1.4.2" or
// "v2.0.0-rc1".
//
// It is a simplified implementation: pre-release identifiers are compared as
// plain strings and build metadata is not supported.
package semver

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ErrSyntax is returned (wrapped) by Parse for malformed versions.
var ErrSyntax = errors.New("semver: invalid version")

// Version is a parsed semantic version.
type Version struct {
	Major, Minor, Patch int
	Pre                 string // pre-release label, empty for a release
}

// Parse reads "MAJOR.MINOR.PATCH" with an optional leading "v" and an
// optional "-prerelease" suffix.
func Parse(s string) (Version, error) {
	core, pre, hasPre := strings.Cut(strings.TrimPrefix(s, "v"), "-")
	if hasPre && pre == "" {
		return Version{}, fmt.Errorf("%w: %q has an empty pre-release", ErrSyntax, s)
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("%w: %q needs three numeric parts", ErrSyntax, s)
	}
	var nums [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || p[0] == '+' {
			return Version{}, fmt.Errorf("%w: %q has a bad number %q", ErrSyntax, s, p)
		}
		nums[i] = n
	}
	return Version{nums[0], nums[1], nums[2], pre}, nil
}

// String formats v as "MAJOR.MINOR.PATCH[-pre]".
func (v Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Pre != "" {
		s += "-" + v.Pre
	}
	return s
}

// Compare returns -1, 0 or +1 depending on whether v is older than, equal to
// or newer than w. A pre-release is older than the release it precedes.
func (v Version) Compare(w Version) int {
	for _, d := range [...][2]int{{v.Major, w.Major}, {v.Minor, w.Minor}, {v.Patch, w.Patch}} {
		if d[0] != d[1] {
			if d[0] < d[1] {
				return -1
			}
			return 1
		}
	}
	switch {
	case v.Pre == w.Pre:
		return 0
	case v.Pre == "":
		return 1
	case w.Pre == "":
		return -1
	}
	return strings.Compare(v.Pre, w.Pre)
}

// Bump returns the next version for the given part ("major", "minor" or
// "patch"), resetting the lower parts and dropping any pre-release.
func (v Version) Bump(part string) (Version, error) {
	switch part {
	case "major":
		return Version{Major: v.Major + 1}, nil
	case "minor":
		return Version{Major: v.Major, Minor: v.Minor + 1}, nil
	case "patch":
		return Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch + 1}, nil
	}
	return Version{}, fmt.Errorf("semver: unknown part %q", part)
}

// Sort orders versions from oldest to newest, in place.
func Sort(vs []Version) {
	sort.Slice(vs, func(i, j int) bool { return vs[i].Compare(vs[j]) < 0 })
}

// GroupByMajor groups versions by their major number.
func GroupByMajor(vs []Version) map[int][]Version {
	groups := make(map[int][]Version)
	for _, v := range vs {
		groups[v.Major] = append(groups[v.Major], v)
	}
	return groups
}
