package frame

import (
	"bytes"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
	"testing/iotest"
)

// encode builds a stream containing one frame per payload.
func encode(t *testing.T, payloads ...string) []byte {
	t.Helper()
	var buf bytes.Buffer // an in-memory io.Writer
	for _, p := range payloads {
		if err := Write(&buf, []byte(p)); err != nil {
			t.Fatalf("Write(%q): %v", p, err)
		}
	}
	return buf.Bytes()
}

// readAll reads frames until a clean EOF.
func readAll(r io.Reader) ([]string, error) {
	var out []string
	for {
		p, err := Read(r)
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return out, err
		}
		out = append(out, string(p))
	}
}

func TestRoundTrip(t *testing.T) {
	stream := encode(t, "hello", "", "world")

	got, err := readAll(bytes.NewReader(stream))
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}
	if want := []string{"hello", "", "world"}; !slices.Equal(got, want) {
		t.Errorf("frames = %q, want %q", got, want)
	}
}

// The same logic must work no matter how the stream chunks its data.
// iotest wraps a reader to simulate the awkward behaviours of real ones.
func TestReadHandlesAwkwardReaders(t *testing.T) {
	stream := encode(t, "alpha", "beta")

	wrappers := map[string]func(io.Reader) io.Reader{
		"one byte at a time": iotest.OneByteReader,
		"half of the buffer": iotest.HalfReader,
		"data with EOF":      iotest.DataErrReader, // last bytes arrive together with io.EOF
	}
	for name, wrap := range wrappers {
		t.Run(name, func(t *testing.T) {
			got, err := readAll(wrap(bytes.NewReader(stream)))
			if err != nil {
				t.Fatalf("readAll: %v", err)
			}
			if want := []string{"alpha", "beta"}; !slices.Equal(got, want) {
				t.Errorf("frames = %q, want %q", got, want)
			}
		})
	}
}

func TestReadTruncatedStreams(t *testing.T) {
	full := encode(t, "payload")

	tests := []struct {
		name   string
		stream []byte
		want   error
	}{
		{"empty stream is a clean end", nil, io.EOF},
		{"cut inside the header", full[:1], io.ErrUnexpectedEOF},
		{"header without payload", full[:2], io.ErrUnexpectedEOF},
		{"cut inside the payload", full[:5], io.ErrUnexpectedEOF},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Read(bytes.NewReader(tt.stream)); !errors.Is(err, tt.want) {
				t.Errorf("Read error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestReadPropagatesReaderErrors(t *testing.T) {
	boom := errors.New("disk on fire")

	t.Run("failing from the start", func(t *testing.T) {
		_, err := Read(iotest.ErrReader(boom))
		if !errors.Is(err, boom) {
			t.Errorf("Read error = %v, want %v", err, boom)
		}
	})

	t.Run("timeout after the header", func(t *testing.T) {
		// TimeoutReader fails on its second Read: the header arrives, the payload times out.
		_, err := Read(iotest.TimeoutReader(bytes.NewReader(encode(t, "payload"))))
		if !errors.Is(err, iotest.ErrTimeout) {
			t.Errorf("Read error = %v, want iotest.ErrTimeout", err)
		}
	})
}

// failingWriter accepts n bytes and then fails, like a closed connection.
type failingWriter struct{ left int }

func (w *failingWriter) Write(p []byte) (int, error) {
	if len(p) > w.left {
		n := w.left
		w.left = 0
		return n, io.ErrShortWrite
	}
	w.left -= len(p)
	return len(p), nil
}

func TestWriteErrors(t *testing.T) {
	t.Run("payload too large", func(t *testing.T) {
		err := Write(io.Discard, make([]byte, MaxPayload+1))
		if !errors.Is(err, ErrTooLarge) {
			t.Errorf("Write error = %v, want ErrTooLarge", err)
		}
	})
	t.Run("largest payload fits", func(t *testing.T) {
		if err := Write(io.Discard, make([]byte, MaxPayload)); err != nil {
			t.Errorf("Write error = %v, want nil", err)
		}
	})
	t.Run("writer fails", func(t *testing.T) {
		err := Write(&failingWriter{left: 3}, []byte("payload"))
		if !errors.Is(err, io.ErrShortWrite) {
			t.Errorf("Write error = %v, want io.ErrShortWrite", err)
		}
	})
}

// iotest.TestReader checks that a custom Reader obeys the io.Reader contract
// (correct counts, EOF behaviour, consistency across read sizes) and that its
// whole output equals the expected content.
func TestRot13Reader(t *testing.T) {
	const plain, rotated = "Hello, World! 123", "Uryyb, Jbeyq! 123"

	r := NewRot13(strings.NewReader(plain))
	if err := iotest.TestReader(r, []byte(rotated)); err != nil {
		t.Fatalf("iotest.TestReader: %v", err)
	}

	// Applying ROT13 twice returns the original text.
	back, err := io.ReadAll(NewRot13(NewRot13(strings.NewReader(plain))))
	if err != nil || string(back) != plain {
		t.Errorf("double ROT13 = %q, %v; want %q", back, err, plain)
	}
}
