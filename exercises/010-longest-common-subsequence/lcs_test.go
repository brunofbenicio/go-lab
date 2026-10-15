package lcs

import "testing"

// isSubsequence reports whether sub can be obtained from s by deleting runes.
func isSubsequence(sub, s string) bool {
	r := []rune(sub)
	k := 0
	for _, c := range s {
		if k < len(r) && r[k] == c {
			k++
		}
	}
	return k == len(r)
}

func TestLCS(t *testing.T) {
	tests := []struct {
		name    string
		a, b    string
		want    string
		wantLen int
	}{
		{"classic", "AGGTAB", "GXTXAYB", "GTAB", 4},
		{"identical", "same", "same", "same", 4},
		{"disjoint", "abc", "def", "", 0},
		{"first empty", "", "abc", "", 0},
		{"both empty", "", "", "", 0},
		{"one inside the other", "ace", "abcde", "ace", 3},
		{"unicode", "ação", "nação", "ação", 4},
		{"textbook pair", "ABCBDAB", "BDCABA", "", 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LCS(tt.a, tt.b)
			if n := Length(tt.a, tt.b); n != tt.wantLen {
				t.Errorf("Length(%q, %q) = %d, want %d", tt.a, tt.b, n, tt.wantLen)
			}
			if tt.want != "" && got != tt.want {
				t.Errorf("LCS(%q, %q) = %q, want %q", tt.a, tt.b, got, tt.want)
			}
			// Whatever answer is chosen, it must be common and of maximal length.
			if !isSubsequence(got, tt.a) || !isSubsequence(got, tt.b) {
				t.Errorf("LCS(%q, %q) = %q is not a common subsequence", tt.a, tt.b, got)
			}
			if len([]rune(got)) != tt.wantLen {
				t.Errorf("LCS(%q, %q) = %q has length %d, want %d",
					tt.a, tt.b, got, len([]rune(got)), tt.wantLen)
			}
		})
	}
}

func TestLengthIsSymmetric(t *testing.T) {
	pairs := [][2]string{
		{"kitten", "sitting"},
		{"abcdef", "acf"},
		{"golang", "language"},
		{"", "x"},
	}
	for _, p := range pairs {
		if Length(p[0], p[1]) != Length(p[1], p[0]) {
			t.Errorf("Length(%q, %q) differs from Length(%q, %q)", p[0], p[1], p[1], p[0])
		}
	}
}
