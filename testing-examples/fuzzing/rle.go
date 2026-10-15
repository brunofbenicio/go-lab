// Package rle implements a tiny run-length encoding for byte slices. Each run
// becomes a pair of bytes: the run length (1-255) followed by the byte value.
//
// Encoders and decoders are classic fuzzing targets: the input space is huge,
// the invariants are simple, and off-by-one errors at run boundaries (such as
// a run of exactly 255 or 256 bytes) are easy to miss in hand-written tests.
package rle

import (
	"errors"
	"fmt"
)

// MaxDecoded is the largest output Decode will produce. It protects against
// "decompression bombs": 2 input bytes can expand to 255.
const MaxDecoded = 1 << 20

// Errors returned by Decode.
var (
	ErrCorrupt  = errors.New("rle: corrupt input")
	ErrTooLarge = errors.New("rle: decoded data too large")
)

// Encode compresses src. Runs longer than 255 are split into several pairs.
func Encode(src []byte) []byte {
	var out []byte
	for i := 0; i < len(src); {
		j := i + 1
		for j < len(src) && src[j] == src[i] && j-i < 255 {
			j++
		}
		out = append(out, byte(j-i), src[i])
		i = j
	}
	return out
}

// Decode reverses Encode.
func Decode(enc []byte) ([]byte, error) {
	if len(enc)%2 != 0 {
		return nil, fmt.Errorf("%w: odd length %d", ErrCorrupt, len(enc))
	}
	total := 0
	for i := 0; i < len(enc); i += 2 {
		if enc[i] == 0 {
			return nil, fmt.Errorf("%w: zero-length run at offset %d", ErrCorrupt, i)
		}
		total += int(enc[i])
	}
	if total > MaxDecoded {
		return nil, fmt.Errorf("%w: %d bytes", ErrTooLarge, total)
	}
	out := make([]byte, 0, total)
	for i := 0; i < len(enc); i += 2 {
		for range enc[i] {
			out = append(out, enc[i+1])
		}
	}
	return out, nil
}
