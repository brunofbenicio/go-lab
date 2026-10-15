package anagram

import (
	"reflect"
	"testing"
)

func TestKey(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"listen", "eilnst"},
		{"Silent", "eilnst"},
		{"dirty room", "dimoorrty"},
		{"dormitory", "dimoorrty"},
		{"a1b2", "12ab"},
		{"", ""},
		{"ação", "aoãç"},
	}
	for _, tt := range tests {
		if got := Key(tt.in); got != tt.want {
			t.Errorf("Key(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestAreAnagrams(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{"classic", "listen", "silent", true},
		{"mixed case", "Listen", "SILENT", true},
		{"phrase", "dormitory", "dirty room", true},
		{"same word", "level", "Level", false},
		{"different letters", "abc", "abd", false},
		{"different length", "abc", "abcc", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AreAnagrams(tt.a, tt.b); got != tt.want {
				t.Errorf("AreAnagrams(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestGroup(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want [][]string
	}{
		{
			name: "classic example",
			in:   []string{"eat", "tea", "tan", "ate", "nat", "bat"},
			want: [][]string{{"eat", "tea", "ate"}, {"tan", "nat"}, {"bat"}},
		},
		{
			name: "case is ignored but preserved",
			in:   []string{"Listen", "Silent", "enlist", "google"},
			want: [][]string{{"Listen", "Silent", "enlist"}, {"google"}},
		},
		{
			name: "duplicates stay together",
			in:   []string{"a", "A", "a"},
			want: [][]string{{"a", "A", "a"}},
		},
		{name: "empty input", in: nil, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Group(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Group(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
