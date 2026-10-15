package rle

import (
	"bytes"
	"errors"
	"testing"
)

// Regular example-based tests still matter: they document known vectors.
func TestEncodeKnownVectors(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []byte
	}{
		{"empty", "", nil},
		{"single byte", "a", []byte{1, 'a'}},
		{"runs", "aaabcc", []byte{3, 'a', 1, 'b', 2, 'c'}},
		{"run of 255 fits in one pair", string(bytes.Repeat([]byte("z"), 255)), []byte{255, 'z'}},
		{"run of 256 is split", string(bytes.Repeat([]byte("z"), 256)), []byte{255, 'z', 1, 'z'}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Encode([]byte(tt.in)); !bytes.Equal(got, tt.want) {
				t.Errorf("Encode = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDecodeErrors(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want error
	}{
		{"odd length", []byte{3, 'a', 1}, ErrCorrupt},
		{"zero-length run", []byte{0, 'a'}, ErrCorrupt},
		{"output too large", bytes.Repeat([]byte{255, 'x'}, MaxDecoded/255+2), ErrTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Decode(tt.in); !errors.Is(err, tt.want) {
				t.Errorf("Decode error = %v, want %v", err, tt.want)
			}
		})
	}
}

// FuzzRoundTrip states a property: decoding what we encoded gives back the
// original, for ANY input.
//
// Under plain `go test` the fuzz function runs only on its seed corpus: the
// f.Add values below plus every file in testdata/fuzz/FuzzRoundTrip. That
// makes fuzz targets cheap regression tests. To actually search for failures:
//
//	go test -fuzz FuzzRoundTrip -fuzztime 20s ./testing-examples/fuzzing/
func FuzzRoundTrip(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("aaabcc"))
	f.Add(bytes.Repeat([]byte("x"), 255)) // boundaries are where bugs live
	f.Add(bytes.Repeat([]byte("x"), 256))
	f.Add([]byte{0, 0, 0, 255, 255})

	f.Fuzz(func(t *testing.T, in []byte) {
		enc := Encode(in)
		out, err := Decode(enc)
		if err != nil {
			t.Fatalf("Decode(Encode(%q)) failed: %v", in, err)
		}
		if !bytes.Equal(in, out) {
			t.Fatalf("round trip changed the data: in=%q out=%q", in, out)
		}
		if len(enc) > 2*len(in) { // worst case: every byte is its own run
			t.Errorf("encoded size %d exceeds twice the input size %d", len(enc), len(in))
		}
	})
}

// FuzzDecode feeds arbitrary (mostly invalid) bytes to the decoder. The
// properties: it never panics, and whatever it accepts is stable when it is
// re-encoded and decoded again.
func FuzzDecode(f *testing.F) {
	f.Add([]byte{3, 'a', 1, 'b'})
	f.Add([]byte{0, 'a'})
	f.Add([]byte{1})
	f.Add([]byte{255, 0, 255, 0})

	f.Fuzz(func(t *testing.T, enc []byte) {
		out, err := Decode(enc)
		if err != nil {
			return // rejecting bad input is fine; panicking is not
		}
		again, err := Decode(Encode(out))
		if err != nil || !bytes.Equal(out, again) {
			t.Fatalf("decoded data is not stable: first=%q again=%q err=%v", out, again, err)
		}
	})
}
