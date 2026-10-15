package trie

import (
	"slices"
	"testing"
)

func build(keys ...string) *Trie[int] {
	t := New[int]()
	for i, k := range keys {
		t.Put(k, i+1)
	}
	return t
}

func TestPutGet(t *testing.T) {
	tr := build("car", "card", "care", "dog")
	tests := []struct {
		key    string
		want   int
		wantOK bool
	}{
		{"car", 1, true},
		{"card", 2, true},
		{"dog", 4, true},
		{"ca", 0, false}, // only a prefix, not a key
		{"cart", 0, false},
		{"", 0, false},
	}
	for _, tt := range tests {
		if got, ok := tr.Get(tt.key); got != tt.want || ok != tt.wantOK {
			t.Errorf("Get(%q) = %d, %v; want %d, %v", tt.key, got, ok, tt.want, tt.wantOK)
		}
	}
	tr.Put("car", 99)
	if v, _ := tr.Get("car"); v != 99 || tr.Len() != 4 {
		t.Errorf("overwrite: value = %d, Len = %d; want 99, 4", v, tr.Len())
	}
}

func TestKeysWithPrefix(t *testing.T) {
	tr := build("car", "card", "care", "dog", "ação", "aço")
	tests := []struct {
		prefix string
		want   []string
	}{
		{"car", []string{"car", "card", "care"}},
		{"card", []string{"card"}},
		{"d", []string{"dog"}},
		{"x", nil},
		{"aç", []string{"aço", "ação"}}, // multi-byte runes are single steps
		{"aço", []string{"aço"}},
		{"", []string{"car", "card", "care", "dog", "aço", "ação"}},
	}
	for _, tt := range tests {
		got := tr.KeysWithPrefix(tt.prefix)
		slices.Sort(got)
		want := slices.Clone(tt.want)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("KeysWithPrefix(%q) = %v, want %v", tt.prefix, got, want)
		}
	}
	if got := tr.KeysWithPrefix("c"); !slices.IsSorted(got) {
		t.Errorf("KeysWithPrefix must be sorted, got %v", got)
	}
}

func TestHasPrefix(t *testing.T) {
	tr := build("hello")
	for prefix, want := range map[string]bool{"": true, "he": true, "hello": true, "hello!": false, "x": false} {
		if got := tr.HasPrefix(prefix); got != want {
			t.Errorf("HasPrefix(%q) = %v, want %v", prefix, got, want)
		}
	}
}

func TestDelete(t *testing.T) {
	tr := build("car", "card", "dog")

	if tr.Delete("ca") || tr.Delete("zzz") {
		t.Fatal("deleting a non-key must report false")
	}
	if !tr.Delete("card") || tr.Len() != 2 {
		t.Fatalf("Delete(card) failed, Len = %d", tr.Len())
	}
	if _, ok := tr.Get("car"); !ok {
		t.Error("deleting card must keep car")
	}
	if tr.HasPrefix("card") {
		t.Error("the 'd' branch below car should have been pruned")
	}
	if !tr.Delete("dog") || tr.HasPrefix("d") {
		t.Error("deleting dog should prune the whole branch")
	}
	if tr.Delete("dog") {
		t.Error("second Delete(dog) should report false")
	}
	if len(tr.root.children) != 1 {
		t.Errorf("root should keep only the 'c' branch, has %d", len(tr.root.children))
	}
}
