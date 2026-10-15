package slugify

import "testing"

func TestMake(t *testing.T) {
	tests := []struct {
		name string
		in   string
		opts []Option
		want string
	}{
		{"simple", "Hello World", nil, "hello-world"},
		{"portuguese accents", "Ação e Reação", nil, "acao-e-reacao"},
		{"punctuation and edges", "  Olá,   Mundo!!  ", nil, "ola-mundo"},
		{"apostrophe is dropped", "São João d'Aliança", nil, "sao-joao-dalianca"},
		{"curly apostrophe", "don’t stop", nil, "dont-stop"},
		{"repeated separators collapse", "Hello___World--again", nil, "hello-world-again"},
		{"symbols become separators", "C++ & Go: 100% fun", nil, "c-go-100-fun"},
		{"sharp s and ligatures", "Straße Œuvre", nil, "strasse-oeuvre"},
		{"decomposed accent", "café au lait", nil, "cafe-au-lait"},
		{"digits are kept", "Top 10 de 2024", nil, "top-10-de-2024"},
		{"other scripts are dropped", "日本語", nil, ""},
		{"empty", "", nil, ""},
		{"only separators", " -- !! ", nil, ""},
		{"custom separator", "Hello World", []Option{WithSeparator("_")}, "hello_world"},
		{"max length cuts at a word boundary", "the quick brown fox", []Option{WithMaxLength(10)}, "the-quick"},
		{"max length exactly at a boundary", "the quick brown fox", []Option{WithMaxLength(9)}, "the-quick"},
		{"max length smaller than the first word", "the quick", []Option{WithMaxLength(2)}, "th"},
		{"max length larger than the slug", "short", []Option{WithMaxLength(50)}, "short"},
		{"max length zero means unlimited", "a b c d", []Option{WithMaxLength(0)}, "a-b-c-d"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Make(tt.in, tt.opts...); got != tt.want {
				t.Errorf("Make(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestMakeIsIdempotent(t *testing.T) {
	for _, in := range []string{"Ação e Reação", "Hello, World!", "a--b"} {
		once := Make(in)
		if twice := Make(once); twice != once {
			t.Errorf("Make(Make(%q)) = %q, want %q", in, twice, once)
		}
	}
}
