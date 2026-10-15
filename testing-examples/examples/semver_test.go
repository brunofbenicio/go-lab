package semver

import (
	"errors"
	"testing"
)

// Examples document the happy paths; ordinary tests cover the edge cases that
// would only clutter the documentation.
func TestParseRejectsMalformedVersions(t *testing.T) {
	for _, in := range []string{"", "1", "1.2", "1.2.3.4", "a.b.c", "1.2.x", "1.-2.3", "1.+2.3", "1.2.3-"} {
		t.Run(in, func(t *testing.T) {
			if _, err := Parse(in); !errors.Is(err, ErrSyntax) {
				t.Errorf("Parse(%q) error = %v, want ErrSyntax", in, err)
			}
		})
	}
}

func TestBumpUnknownPart(t *testing.T) {
	if _, err := (Version{1, 2, 3, ""}).Bump("build"); err == nil {
		t.Error("Bump(build) succeeded, want an error")
	}
}

func TestBumpResetsLowerPartsAndPreRelease(t *testing.T) {
	v, _ := Parse("1.4.2-beta")
	for part, want := range map[string]string{"major": "2.0.0", "minor": "1.5.0", "patch": "1.4.3"} {
		got, err := v.Bump(part)
		if err != nil || got.String() != want {
			t.Errorf("Bump(%q) = %v, %v; want %s", part, got, err, want)
		}
	}
}
