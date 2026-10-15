package caesar

import "testing"

func TestShift(t *testing.T) {
	tests := []struct {
		name string
		in   string
		k    int
		want string
	}{
		{"classic", "Hello, World!", 3, "Khoor, Zruog!"},
		{"wraps around", "xyz XYZ", 3, "abc ABC"},
		{"negative key", "Khoor", -3, "Hello"},
		{"key larger than alphabet", "abc", 27, "bcd"},
		{"large negative key", "abc", -53, "zab"},
		{"zero key", "unchanged", 0, "unchanged"},
		{"non letters kept", "a-1_b é!", 1, "b-1_c é!"},
		{"empty", "", 5, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Shift(tt.in, tt.k); got != tt.want {
				t.Errorf("Shift(%q, %d) = %q, want %q", tt.in, tt.k, got, tt.want)
			}
		})
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	const text = "The Quick Brown Fox, 42 times!"
	for key := -30; key <= 30; key++ {
		if got := Decrypt(Encrypt(text, key), key); got != text {
			t.Errorf("key %d: round trip = %q, want %q", key, got, text)
		}
	}
}

func TestCrack(t *testing.T) {
	const plain = "It was the best of times, it was the worst of times, it was the age " +
		"of wisdom, it was the age of foolishness, it was the epoch of belief"
	tests := []struct {
		name string
		key  int
	}{
		{"key 0", 0},
		{"key 3", 3},
		{"key 7", 7},
		{"key 13", 13},
		{"key 25", 25},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotKey, gotPlain := Crack(Encrypt(plain, tt.key))
			if gotKey != tt.key || gotPlain != plain {
				t.Errorf("Crack() = (%d, %q), want (%d, plain text)", gotKey, gotPlain, tt.key)
			}
		})
	}
}

func TestCrackWithoutLetters(t *testing.T) {
	key, got := Crack("1234 !?")
	if key != 0 || got != "1234 !?" {
		t.Errorf("Crack() = (%d, %q), want (0, unchanged text)", key, got)
	}
}
