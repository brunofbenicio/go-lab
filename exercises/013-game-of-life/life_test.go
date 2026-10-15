package life

import (
	"strings"
	"testing"
)

func mustParse(t *testing.T, wrap bool, lines ...string) *Grid {
	t.Helper()
	g, err := Parse(wrap, lines...)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	return g
}

func TestStep(t *testing.T) {
	tests := []struct {
		name  string
		wrap  bool
		start []string
		want  []string
	}{
		{
			name:  "block is a still life",
			start: []string{"....", ".##.", ".##.", "...."},
			want:  []string{"....", ".##.", ".##.", "...."},
		},
		{
			name:  "blinker flips to horizontal",
			start: []string{".....", "..#..", "..#..", "..#..", "....."},
			want:  []string{".....", ".....", ".###.", ".....", "....."},
		},
		{
			name:  "lonely cell dies",
			start: []string{"...", ".#.", "..."},
			want:  []string{"...", "...", "..."},
		},
		{
			name:  "three neighbors give birth",
			start: []string{"##.", "#..", "..."},
			want:  []string{"##.", "##.", "..."},
		},
		{
			name:  "bounded grid treats the outside as dead",
			start: []string{"###"},
			want:  []string{".#."},
		},
		{
			name:  "wrapping grid connects opposite edges",
			wrap:  true,
			start: []string{"..#..", "..#..", ".....", ".....", "..#.."},
			want:  []string{".###.", ".....", ".....", ".....", "....."},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mustParse(t, tt.wrap, tt.start...).Step()
			if want := strings.Join(tt.want, "\n"); got.String() != want {
				t.Errorf("Step() =\n%s\nwant\n%s", got, want)
			}
		})
	}
}

func TestBlinkerHasPeriodTwo(t *testing.T) {
	g := mustParse(t, false, ".....", "..#..", "..#..", "..#..", ".....")
	if !g.Step().Step().Equal(g) {
		t.Error("blinker did not return to its start after two steps")
	}
	if g.Step().Equal(g) {
		t.Error("blinker should change after one step")
	}
}

func TestGliderOnTorus(t *testing.T) {
	start := mustParse(t, true,
		".#....",
		"..#...",
		"###...",
		"......",
		"......",
		"......",
	)
	moved := mustParse(t, true,
		"......",
		"..#...",
		"...#..",
		".###..",
		"......",
		"......",
	)
	g := start
	for i := 0; i < 4; i++ {
		g = g.Step()
	}
	if !g.Equal(moved) {
		t.Errorf("after 4 steps glider =\n%s\nwant\n%s", g, moved)
	}
	// Moving one cell diagonally every 4 steps, it comes back after 6*4 steps.
	for i := 4; i < 24; i++ {
		g = g.Step()
	}
	if !g.Equal(start) {
		t.Errorf("after 24 steps glider =\n%s\nwant\n%s", g, start)
	}
	if g.Population() != 5 {
		t.Errorf("Population() = %d, want 5", g.Population())
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
	}{
		{"no lines", nil},
		{"empty line", []string{""}},
		{"ragged", []string{"..", "..."}},
		{"invalid character", []string{".x."}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if g, err := Parse(false, tt.lines...); err == nil {
				t.Errorf("Parse(%q) = %v, want error", tt.lines, g)
			}
		})
	}
}

func TestNewAndSet(t *testing.T) {
	if _, err := New(0, 3, false); err == nil {
		t.Error("New(0, 3) should fail")
	}
	g, err := New(2, 2, false)
	if err != nil {
		t.Fatal(err)
	}
	g.Set(1, 1, true)
	g.Set(5, 5, true) // ignored
	if !g.Alive(1, 1) || g.Alive(5, 5) || g.Population() != 1 {
		t.Errorf("unexpected grid state:\n%s", g)
	}
}
