package password

import (
	"bytes"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// fastIterations keeps the tests quick; production code uses DefaultIterations.
const fastIterations = 1000

func newHasher(t *testing.T, opts ...Option) *Hasher {
	t.Helper()
	h, err := NewHasher(append([]Option{WithIterations(fastIterations)}, opts...)...)
	if err != nil {
		t.Fatalf("NewHasher() failed: %v", err)
	}
	return h
}

func TestDeriveKnownAnswers(t *testing.T) {
	// PBKDF2-HMAC-SHA-256 vectors; the first one is from RFC 7914, section 11
	// (truncated to 32 bytes), the others are the widely used "password"/"salt"
	// vectors.
	tests := []struct {
		name       string
		password   string
		salt       string
		iterations int
		want       string
	}{
		{"RFC 7914", "passwd", "salt", 1, "55ac046e56e3089fec1691c22544b605f94185216dde0465e68b9d57c20dacbc"},
		{"one iteration", "password", "salt", 1, "120fb6cffcf8b32c43e7225256c4f837a86548c92ccc35480805987cb70be17b"},
		{"two iterations", "password", "salt", 2, "ae4d0c95af6b46d32d0adff928f06dd02a303f8ef3c251dfd6e2d85a95474c43"},
		{"4096 iterations", "password", "salt", 4096, "c5e478d59288c841aa530db6845c4c8d962893a001ce4e11a4963873aa98134a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := derive(tt.password, []byte(tt.salt), tt.iterations)
			if err != nil {
				t.Fatal(err)
			}
			if hex.EncodeToString(got) != tt.want {
				t.Errorf("derive() = %x, want %s", got, tt.want)
			}
		})
	}
}

func TestHashGoldenValue(t *testing.T) {
	// Salt 00 01 02 ... 0f; the expected string was computed independently
	// with Python's hashlib.pbkdf2_hmac.
	salt := make([]byte, DefaultSaltLength)
	for i := range salt {
		salt[i] = byte(i)
	}
	h := newHasher(t, WithRandom(bytes.NewReader(salt)))

	got, err := h.Hash("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	const want = "$pbkdf2-sha256$i=1000$AAECAwQFBgcICQoLDA0ODw$ppsXnjrdPB4KryJ6DrOqKqhkWrhv7PbKAMF1Eml8cZ4"
	if got != want {
		t.Errorf("Hash() = %q, want %q", got, want)
	}
}

func TestHashFormatAndUniqueness(t *testing.T) {
	h := newHasher(t)
	format := regexp.MustCompile(`^\$pbkdf2-sha256\$i=1000\$[A-Za-z0-9+/]{22}\$[A-Za-z0-9+/]{43}$`)

	seen := make(map[string]bool)
	for range 5 {
		encoded, err := h.Hash("same password")
		if err != nil {
			t.Fatal(err)
		}
		if !format.MatchString(encoded) {
			t.Fatalf("hash %q does not match the expected format", encoded)
		}
		if seen[encoded] {
			t.Fatalf("hash %q was produced twice: salts must be random", encoded)
		}
		seen[encoded] = true
	}
}

func TestHashAndVerify(t *testing.T) {
	h := newHasher(t)

	tests := []struct {
		name     string
		password string
		attempt  string
		want     bool
	}{
		{"same password", "correct horse battery staple", "correct horse battery staple", true},
		{"wrong password", "correct horse battery staple", "Correct horse battery staple", false},
		{"prefix of the password", "correct horse", "correct", false},
		{"password with an extra character", "correct", "correct ", false},
		{"unicode password", "pässwörd-日本語-😀", "pässwörd-日本語-😀", true},
		{"unicode password, different normalization", "é", "é", false},
		{"embedded NUL byte", "pass\x00word", "pass\x00word", true},
		{"NUL byte is significant", "pass\x00word", "pass", false},
		{"longest accepted password", strings.Repeat("a", MaxPasswordLength), strings.Repeat("a", MaxPasswordLength), true},
		{"empty attempt", "correct horse", "", false},
		{"attempt above the length limit", "correct horse", strings.Repeat("a", MaxPasswordLength+1), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := h.Hash(tt.password)
			if err != nil {
				t.Fatalf("Hash() failed: %v", err)
			}
			got, err := h.Verify(tt.attempt, encoded)
			if err != nil {
				t.Fatalf("Verify() failed: %v", err)
			}
			if got != tt.want {
				t.Errorf("Verify(%q) = %v, want %v", tt.attempt, got, tt.want)
			}
		})
	}
}

func TestHashRejectsBadPasswords(t *testing.T) {
	h := newHasher(t)
	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{"empty", "", ErrEmptyPassword},
		{"too long", strings.Repeat("a", MaxPasswordLength+1), ErrPasswordTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := h.Hash(tt.password); !errors.Is(err, tt.wantErr) {
				t.Errorf("Hash() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestHashFailsWithoutRandomness(t *testing.T) {
	h := newHasher(t, WithRandom(bytes.NewReader([]byte{1, 2, 3}))) // fewer bytes than a salt
	if _, err := h.Hash("secret"); err == nil {
		t.Error("Hash() succeeded although the salt could not be read")
	}
}

func TestVerifyUsesParametersFromTheHash(t *testing.T) {
	cheap := newHasher(t, WithIterations(3), WithSaltLength(MinSaltLength))
	encoded, err := cheap.Hash("secret")
	if err != nil {
		t.Fatal(err)
	}

	// A hasher configured differently still verifies the older hash.
	strict := newHasher(t, WithIterations(5000), WithSaltLength(32))
	ok, err := strict.Verify("secret", encoded)
	if err != nil || !ok {
		t.Errorf("Verify() = %v, %v; want true, nil", ok, err)
	}
}

func TestVerifyRejectsInvalidHashes(t *testing.T) {
	h := newHasher(t)
	valid, err := h.Hash("secret")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(valid, "$") // ["", scheme, "i=1000", salt, key]

	join := func(p ...string) string { return strings.Join(p, "$") }
	tests := []struct {
		name    string
		encoded string
	}{
		{"empty", ""},
		{"plain text", "not a hash"},
		{"another scheme", join("", "bcrypt", parts[2], parts[3], parts[4])},
		{"missing key", join("", parts[1], parts[2], parts[3])},
		{"extra segment", valid + "$extra"},
		{"no leading dollar", strings.TrimPrefix(valid, "$")},
		{"missing iteration prefix", join("", parts[1], "1000", parts[3], parts[4])},
		{"iterations not a number", join("", parts[1], "i=many", parts[3], parts[4])},
		{"zero iterations", join("", parts[1], "i=0", parts[3], parts[4])},
		{"negative iterations", join("", parts[1], "i=-5", parts[3], parts[4])},
		{"iterations with a plus sign", join("", parts[1], "i=+1000", parts[3], parts[4])},
		{"iterations with leading zeros", join("", parts[1], "i=01000", parts[3], parts[4])},
		{"too many iterations", join("", parts[1], "i=10000001", parts[3], parts[4])},
		{"iterations overflow", join("", parts[1], "i=99999999999999999999", parts[3], parts[4])},
		{"salt is not base64", join("", parts[1], parts[2], "!!!", parts[4])},
		{"salt is empty", join("", parts[1], parts[2], "", parts[4])},
		{"salt with padding", join("", parts[1], parts[2], parts[3]+"==", parts[4])},
		{"key is not base64", join("", parts[1], parts[2], parts[3], "***")},
		{"key is empty", join("", parts[1], parts[2], parts[3], "")},
		{"key is too short", join("", parts[1], parts[2], parts[3], parts[4][:20])},
		{"key is too long", join("", parts[1], parts[2], parts[3], parts[4]+"AAAA")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := h.Verify("secret", tt.encoded)
			if !errors.Is(err, ErrInvalidHash) {
				t.Errorf("Verify() error = %v, want ErrInvalidHash", err)
			}
			if ok {
				t.Error("Verify() reported a match for an invalid hash")
			}
		})
	}
}

func TestNewHasherValidatesOptions(t *testing.T) {
	tests := []struct {
		name    string
		opts    []Option
		wantErr bool
	}{
		{"defaults", nil, false},
		{"one iteration", []Option{WithIterations(1)}, false},
		{"maximum iterations", []Option{WithIterations(MaxIterations)}, false},
		{"zero iterations", []Option{WithIterations(0)}, true},
		{"negative iterations", []Option{WithIterations(-1)}, true},
		{"too many iterations", []Option{WithIterations(MaxIterations + 1)}, true},
		{"minimum salt", []Option{WithSaltLength(MinSaltLength)}, false},
		{"salt too short", []Option{WithSaltLength(MinSaltLength - 1)}, true},
		{"nil random is ignored", []Option{WithRandom(nil)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, err := NewHasher(tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewHasher() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && h == nil {
				t.Error("NewHasher() returned nil without an error")
			}
		})
	}
}

func TestDefaultCostMeetsRecommendation(t *testing.T) {
	if DefaultIterations < 600_000 {
		t.Errorf("DefaultIterations = %d, below the OWASP recommendation of 600000", DefaultIterations)
	}
	h, err := NewHasher()
	if err != nil {
		t.Fatal(err)
	}
	if h.iterations != DefaultIterations || h.saltLength != DefaultSaltLength {
		t.Errorf("defaults not applied: %+v", h)
	}
}

func TestConcurrentUse(t *testing.T) {
	h := newHasher(t)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			encoded, err := h.Hash("shared password")
			if err != nil {
				t.Errorf("Hash() failed: %v", err)
				return
			}
			if ok, err := h.Verify("shared password", encoded); err != nil || !ok {
				t.Errorf("Verify() = %v, %v; want true, nil", ok, err)
			}
		}()
	}
	wg.Wait()
}
