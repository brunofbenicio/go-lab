package uuid

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"testing"
)

var v4Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestNewIsVersion4AndUnique(t *testing.T) {
	seen := make(map[UUID]bool)
	for i := 0; i < 1000; i++ {
		u := New()
		if !v4Pattern.MatchString(u.String()) {
			t.Fatalf("%s does not look like a version 4 UUID", u)
		}
		if u.Version() != 4 || u.IsNil() {
			t.Fatalf("Version = %d, IsNil = %v", u.Version(), u.IsNil())
		}
		if seen[u] {
			t.Fatalf("duplicate UUID %s", u)
		}
		seen[u] = true
	}
}

func TestNewFromReaderIsDeterministic(t *testing.T) {
	src := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	u, err := NewFromReader(bytes.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	// Byte 6 becomes 0x46 (version 4) and byte 8 becomes 0x88 (RFC 4122 variant).
	if want := "00010203-0405-4607-8809-0a0b0c0d0e0f"; u.String() != want {
		t.Errorf("got %s, want %s", u, want)
	}
	if _, err := NewFromReader(bytes.NewReader(src[:5])); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("short reader: err = %v, want io.ErrUnexpectedEOF", err)
	}
}

func TestParse(t *testing.T) {
	const canonical = "123e4567-e89b-42d3-a456-426614174000"
	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"canonical", canonical, false},
		{"uppercase", "123E4567-E89B-42D3-A456-426614174000", false},
		{"nil uuid", "00000000-0000-0000-0000-000000000000", false},
		{"empty", "", true},
		{"too short", "123e4567-e89b-42d3-a456-42661417400", true},
		{"too long", canonical + "0", true},
		{"missing dashes", "123e4567e89b42d3a456426614174000", true},
		{"dash in the wrong place", "123e4567e-89b-42d3-a456-426614174000", true},
		{"non-hex digit", "123e4567-e89b-42d3-a456-42661417400g", true},
		{"braces", "{" + canonical[:35], true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := Parse(tt.in)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalid) {
					t.Errorf("err = %v, want ErrInvalid", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if _, err := Parse(u.String()); err != nil {
				t.Errorf("String output %q does not parse back: %v", u, err)
			}
		})
	}
}

func TestStringRoundTrip(t *testing.T) {
	for i := 0; i < 100; i++ {
		u := New()
		back, err := Parse(u.String())
		if err != nil || back != u {
			t.Fatalf("round trip of %s gave %s, %v", u, back, err)
		}
	}
}

func TestJSON(t *testing.T) {
	type doc struct {
		ID UUID `json:"id"`
	}
	in := doc{ID: New()}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"id":"` + in.ID.String() + `"}`; string(data) != want {
		t.Errorf("Marshal = %s, want %s", data, want)
	}
	var out doc
	if err := json.Unmarshal(data, &out); err != nil || out != in {
		t.Errorf("Unmarshal = %+v, %v; want %+v", out, err, in)
	}
	if err := json.Unmarshal([]byte(`{"id":"nope"}`), &out); err == nil {
		t.Error("Unmarshal of an invalid UUID should fail")
	}
}
