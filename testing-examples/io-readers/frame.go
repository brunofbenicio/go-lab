// Package frame implements length-prefixed messages over a byte stream: each
// frame is a 2-byte big-endian length followed by that many payload bytes.
// It is the kind of code that must cope with streams delivering data in odd
// chunks, which is what the tests exercise.
package frame

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// MaxPayload is the largest payload a frame can carry.
const MaxPayload = 1<<16 - 1

// ErrTooLarge is returned by Write when the payload exceeds MaxPayload.
var ErrTooLarge = errors.New("frame: payload too large")

// Write sends payload as a single frame.
func Write(w io.Writer, payload []byte) error {
	if len(payload) > MaxPayload {
		return fmt.Errorf("%w: %d bytes", ErrTooLarge, len(payload))
	}
	buf := make([]byte, 2, 2+len(payload))
	binary.BigEndian.PutUint16(buf, uint16(len(payload)))
	buf = append(buf, payload...)
	_, err := w.Write(buf) // a Writer must return an error on short writes
	return err
}

// Read returns the next frame. It returns io.EOF only when the stream ends
// cleanly between frames, and io.ErrUnexpectedEOF when it ends in the middle
// of one. Other read errors are wrapped and can be matched with errors.Is.
func Read(r io.Reader) ([]byte, error) {
	var header [2]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil, err // clean end, or a cut-off header
		}
		return nil, fmt.Errorf("frame: read header: %w", err)
	}
	payload := make([]byte, binary.BigEndian.Uint16(header[:]))
	if _, err := io.ReadFull(r, payload); err != nil {
		if err == io.EOF { // the header promised bytes that never came
			err = io.ErrUnexpectedEOF
		}
		if err == io.ErrUnexpectedEOF {
			return nil, err
		}
		return nil, fmt.Errorf("frame: read payload: %w", err)
	}
	return payload, nil
}
