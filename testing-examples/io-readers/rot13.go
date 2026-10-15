package frame

import "io"

// rot13Reader applies the ROT13 substitution to everything read from r. It is
// here as a second subject: a custom io.Reader that testing/iotest can verify.
type rot13Reader struct{ r io.Reader }

// NewRot13 wraps r so that letters are rotated by 13 places.
func NewRot13(r io.Reader) io.Reader { return rot13Reader{r} }

func (x rot13Reader) Read(p []byte) (int, error) {
	n, err := x.r.Read(p)
	for i, c := range p[:n] {
		switch {
		case c >= 'a' && c <= 'z':
			p[i] = 'a' + (c-'a'+13)%26
		case c >= 'A' && c <= 'Z':
			p[i] = 'A' + (c-'A'+13)%26
		}
	}
	return n, err
}
