package signature

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// testSecret is an obviously fake key used only by these tests.
var testSecret = []byte("whsec_test_only_not_a_real_secret")

func TestSignKnownAnswers(t *testing.T) {
	// The first two cases are the HMAC-SHA-256 test vectors 2 and 1 of RFC 4231.
	// The third was computed independently with Python's hmac module.
	tests := []struct {
		name    string
		secret  []byte
		message []byte
		want    string
	}{
		{
			name:    "RFC 4231 test case 2",
			secret:  []byte("Jefe"),
			message: []byte("what do ya want for nothing?"),
			want:    "sha256=5bdcc146bf60754e6a042426089575c75a003f089d2739839dec58b964ec3843",
		},
		{
			name:    "RFC 4231 test case 1",
			secret:  bytes.Repeat([]byte{0x0b}, 20),
			message: []byte("Hi There"),
			want:    "sha256=b0344c61d8db38535ca8afceaf0bf12b881dc200c9833da726e9376c2e32cff7",
		},
		{
			name:    "a webhook delivery",
			secret:  testSecret,
			message: Message("evt_001", "1740830400", []byte(`{"type":"order.created","data":{"id":42}}`)),
			want:    "sha256=40694aff0c74a1ed4e8ad03fca5bd67aee0eb3559662c6f92cf607e10e6bf097",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Sign(tt.secret, tt.message); got != tt.want {
				t.Errorf("Sign() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMessage(t *testing.T) {
	tests := []struct {
		name   string
		id, ts string
		body   []byte
		want   string
	}{
		{name: "regular", id: "evt_1", ts: "1740830400", body: []byte(`{"a":1}`), want: `evt_1.1740830400.{"a":1}`},
		{name: "empty body", id: "evt_1", ts: "1740830400", body: nil, want: "evt_1.1740830400."},
		{name: "body with dots and newlines", id: "e", ts: "1", body: []byte("a.b\n.c"), want: "e.1.a.b\n.c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := string(Message(tt.id, tt.ts, tt.body)); got != tt.want {
				t.Errorf("Message() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVerify(t *testing.T) {
	msg := Message("evt_001", "1740830400", []byte(`{"type":"order.created"}`))
	valid := Sign(testSecret, msg)
	hexPart := strings.TrimPrefix(valid, Prefix)

	tests := []struct {
		name    string
		secret  []byte
		message []byte
		header  string
		wantErr error
	}{
		{"valid", testSecret, msg, valid, nil},
		{"valid with surrounding whitespace", testSecret, msg, "  " + valid + "\n", nil},
		{"valid with uppercase hex", testSecret, msg, Prefix + strings.ToUpper(hexPart), nil},

		{"wrong secret", []byte("another-fake-secret"), msg, valid, ErrMismatch},
		{"body changed", testSecret, Message("evt_001", "1740830400", []byte(`{"type":"order.deleted"}`)), valid, ErrMismatch},
		{"delivery id changed", testSecret, Message("evt_002", "1740830400", []byte(`{"type":"order.created"}`)), valid, ErrMismatch},
		{"timestamp changed", testSecret, Message("evt_001", "1740830401", []byte(`{"type":"order.created"}`)), valid, ErrMismatch},
		{"last digit changed", testSecret, msg, valid[:len(valid)-1] + flip(valid[len(valid)-1]), ErrMismatch},
		{"all zero digest", testSecret, msg, Prefix + strings.Repeat("0", 64), ErrMismatch},

		{"empty header", testSecret, msg, "", ErrMissing},
		{"blank header", testSecret, msg, "   ", ErrMissing},
		{"no prefix", testSecret, msg, hexPart, ErrMalformed},
		{"other algorithm", testSecret, msg, "sha1=" + hexPart, ErrMalformed},
		{"uppercase prefix", testSecret, msg, "SHA256=" + hexPart, ErrMalformed},
		{"prefix only", testSecret, msg, Prefix, ErrMalformed},
		{"digest too short", testSecret, msg, valid[:len(valid)-2], ErrMalformed},
		{"digest too long", testSecret, msg, valid + "00", ErrMalformed},
		{"not hexadecimal", testSecret, msg, Prefix + strings.Repeat("z", 64), ErrMalformed},
		{"two signatures", testSecret, msg, valid + " " + valid, ErrMalformed},

		{"empty secret", nil, msg, Sign(nil, msg), ErrEmptySecret},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Verify(tt.secret, tt.message, tt.header); !errors.Is(err, tt.wantErr) {
				t.Errorf("Verify() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// flip returns a different hexadecimal digit than c.
func flip(c byte) string {
	if c == '0' {
		return "1"
	}
	return "0"
}

func TestSignAndVerifyRoundTrip(t *testing.T) {
	bodies := [][]byte{
		nil,
		[]byte(`{}`),
		[]byte(`{"type":"user.created","data":{"name":"Zoë","emoji":"😀"}}`),
		bytes.Repeat([]byte("x"), 1<<20),
		{0x00, 0xff, 0x10, '\n', '.'},
	}
	for _, body := range bodies {
		msg := Message("evt_rt", "1740830400", body)
		if err := Verify(testSecret, msg, Sign(testSecret, msg)); err != nil {
			t.Errorf("round trip failed for a %d byte body: %v", len(body), err)
		}
	}
}

func TestSignatureLooksLikeSha256Hex(t *testing.T) {
	sig := Sign(testSecret, []byte("anything"))
	if !strings.HasPrefix(sig, "sha256=") || len(sig) != len("sha256=")+64 {
		t.Errorf("Sign() = %q, want sha256= followed by 64 hex digits", sig)
	}
	if sig != strings.ToLower(sig) {
		t.Errorf("Sign() = %q, want lowercase hex", sig)
	}
}
