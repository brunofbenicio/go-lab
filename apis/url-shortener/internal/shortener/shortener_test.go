package shortener

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brunofbenicio/go-lab/apis/url-shortener/internal/base62"
	"github.com/brunofbenicio/go-lab/apis/url-shortener/internal/store"
)

var t0 = time.Date(2025, time.March, 1, 12, 0, 0, 0, time.UTC)

// fakeClock is a manually advanced clock.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock { return &fakeClock{t: t0} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// sequence returns a reader that yields the given values as big-endian
// uint64s, one per code draw. Reading past the end returns io.EOF.
func sequence(values ...uint64) io.Reader {
	var buf bytes.Buffer
	for _, v := range values {
		_ = binary.Write(&buf, binary.BigEndian, v)
	}
	return &buf
}

// countingReader yields the same 8 bytes forever and counts the draws.
type countingReader struct {
	value uint64
	draws int
}

func (r *countingReader) Read(p []byte) (int, error) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], r.value)
	r.draws++
	return copy(p, b[:]), nil
}

// failingStore wraps a Store and fails Create with a fixed error.
type failingStore struct {
	store.Store
	err error
}

func (f failingStore) Create(context.Context, store.Link) error { return f.err }

func newService(t *testing.T, opts ...Option) (*Service, *store.Memory, *fakeClock) {
	t.Helper()
	clock := newFakeClock()
	mem := store.NewMemory()
	opts = append([]Option{WithClock(clock.Now)}, opts...)
	svc, err := New(mem, opts...)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	return svc, mem, clock
}

func TestNewValidatesOptions(t *testing.T) {
	tests := []struct {
		name    string
		store   store.Store
		opts    []Option
		wantErr bool
	}{
		{name: "defaults", store: store.NewMemory()},
		{name: "nil store", store: nil, wantErr: true},
		{name: "minimum code length", store: store.NewMemory(), opts: []Option{WithCodeLength(MinCodeLength)}},
		{name: "maximum code length", store: store.NewMemory(), opts: []Option{WithCodeLength(MaxCodeLength)}},
		{name: "code length too short", store: store.NewMemory(), opts: []Option{WithCodeLength(MinCodeLength - 1)}, wantErr: true},
		{name: "code length too long", store: store.NewMemory(), opts: []Option{WithCodeLength(MaxCodeLength + 1)}, wantErr: true},
		{name: "zero attempts", store: store.NewMemory(), opts: []Option{WithMaxAttempts(0)}, wantErr: true},
		{name: "zero max TTL", store: store.NewMemory(), opts: []Option{WithMaxTTL(0)}, wantErr: true},
		{name: "nil clock and reader are ignored", store: store.NewMemory(), opts: []Option{WithClock(nil), WithRandom(nil)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := New(tt.store, tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Fatalf("New() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && svc == nil {
				t.Error("New() returned a nil service without an error")
			}
		})
	}
}

func TestShorten(t *testing.T) {
	const space7 = uint64(3521614606208) // 62^7

	tests := []struct {
		name       string
		draw       uint64
		in         ShortenInput
		wantCode   string
		wantExpiry time.Time
	}{
		{
			name:     "no expiry",
			draw:     0,
			in:       ShortenInput{URL: "https://example.com/page"},
			wantCode: "0000000",
		},
		{
			name:       "with expiry",
			draw:       61,
			in:         ShortenInput{URL: "https://example.com/page", TTL: time.Hour},
			wantCode:   "000000z",
			wantExpiry: t0.Add(time.Hour),
		},
		{
			name:     "draw wraps around the code space",
			draw:     space7 + 5,
			in:       ShortenInput{URL: "http://example.com"},
			wantCode: "0000005",
		},
		{
			name:     "surrounding whitespace is trimmed",
			draw:     62,
			in:       ShortenInput{URL: "  https://example.com/x \n"},
			wantCode: "0000010",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, mem, _ := newService(t, WithRandom(sequence(tt.draw)))

			link, err := svc.Shorten(context.Background(), tt.in)
			if err != nil {
				t.Fatalf("Shorten() failed: %v", err)
			}
			if link.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", link.Code, tt.wantCode)
			}
			if want := strings.TrimSpace(tt.in.URL); link.URL != want {
				t.Errorf("URL = %q, want %q", link.URL, want)
			}
			if !link.CreatedAt.Equal(t0) {
				t.Errorf("CreatedAt = %v, want %v", link.CreatedAt, t0)
			}
			if !link.ExpiresAt.Equal(tt.wantExpiry) {
				t.Errorf("ExpiresAt = %v, want %v", link.ExpiresAt, tt.wantExpiry)
			}
			if link.Hits != 0 {
				t.Errorf("Hits = %d, want 0", link.Hits)
			}

			stored, err := mem.Get(context.Background(), link.Code)
			if err != nil {
				t.Fatalf("link was not stored: %v", err)
			}
			if stored != link {
				t.Errorf("stored %+v differs from returned %+v", stored, link)
			}
		})
	}
}

func TestShortenValidatesURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		ok   bool
	}{
		{"https", "https://example.com", true},
		{"http with port, path and query", "http://example.com:8080/a/b?x=1&y=2#frag", true},
		{"uppercase scheme", "HTTPS://example.com", true},
		{"IPv6 host", "http://[2001:db8::1]:8080/", true},
		{"internationalized host", "https://münchen.example.com/straße", true},
		{"empty", "", false},
		{"blank", "   ", false},
		{"no scheme", "example.com/page", false},
		{"scheme-relative", "//example.com/page", false},
		{"ftp scheme", "ftp://example.com/file", false},
		{"javascript scheme", "javascript:alert(1)", false},
		{"data scheme", "data:text/html,hello", false},
		{"mailto", "mailto:someone@example.com", false},
		{"missing host", "https:///path", false},
		{"only a port", "http://:8080/", false},
		{"credentials", "https://user:secret@example.com/", false},
		{"username only", "https://user@example.com/", false},
		{"inner space", "https://example.com/a b", false},
		{"control character", "https://example.com/a\x00b", false},
		{"newline inside", "https://example.com/a\nb", false},
		{"malformed escape", "https://example.com/%zz", false},
		{"too long", "https://example.com/" + strings.Repeat("a", MaxURLLength), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, _ := newService(t, WithRandom(sequence(1)))
			_, err := svc.Shorten(context.Background(), ShortenInput{URL: tt.url})
			if tt.ok {
				if err != nil {
					t.Fatalf("Shorten(%q) failed: %v", tt.url, err)
				}
				return
			}
			if !errors.Is(err, ErrInvalidURL) {
				t.Errorf("Shorten(%q) error = %v, want ErrInvalidURL", tt.url, err)
			}
		})
	}
}

func TestShortenValidatesTTL(t *testing.T) {
	tests := []struct {
		name    string
		ttl     time.Duration
		wantErr bool
	}{
		{"zero means no expiry", 0, false},
		{"one second", time.Second, false},
		{"exactly the maximum", DefaultMaxTTL, false},
		{"negative", -time.Second, true},
		{"above the maximum", DefaultMaxTTL + time.Second, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, _ := newService(t, WithRandom(sequence(1)))
			_, err := svc.Shorten(context.Background(), ShortenInput{URL: "https://example.com", TTL: tt.ttl})
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidTTL) {
					t.Errorf("error = %v, want ErrInvalidTTL", err)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestShortenRetriesOnCollision(t *testing.T) {
	// The first and second draws are identical, so the second link collides
	// with the first and must fall through to the third draw.
	svc, mem, _ := newService(t, WithRandom(sequence(7, 7, 8)))
	ctx := context.Background()

	first, err := svc.Shorten(ctx, ShortenInput{URL: "https://example.com/1"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Shorten(ctx, ShortenInput{URL: "https://example.com/2"})
	if err != nil {
		t.Fatal(err)
	}

	if first.Code != "0000007" || second.Code != "0000008" {
		t.Errorf("codes = %q, %q; want 0000007, 0000008", first.Code, second.Code)
	}
	if mem.Len() != 2 {
		t.Errorf("store holds %d links, want 2", mem.Len())
	}
	got, err := mem.Get(ctx, first.Code)
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != "https://example.com/1" {
		t.Errorf("collision overwrote the first link: URL = %q", got.URL)
	}
}

func TestShortenGivesUpAfterMaxAttempts(t *testing.T) {
	const attempts = 3
	rnd := &countingReader{value: 42}
	svc, mem, _ := newService(t, WithRandom(rnd), WithMaxAttempts(attempts))
	ctx := context.Background()

	if _, err := svc.Shorten(ctx, ShortenInput{URL: "https://example.com/1"}); err != nil {
		t.Fatal(err)
	}
	rnd.draws = 0

	_, err := svc.Shorten(ctx, ShortenInput{URL: "https://example.com/2"})
	if !errors.Is(err, ErrCodeGeneration) {
		t.Fatalf("error = %v, want ErrCodeGeneration", err)
	}
	if rnd.draws != attempts {
		t.Errorf("drew %d codes, want %d", rnd.draws, attempts)
	}
	if mem.Len() != 1 {
		t.Errorf("store holds %d links, want 1", mem.Len())
	}
}

func TestShortenPropagatesFailures(t *testing.T) {
	boom := errors.New("disk on fire")

	t.Run("random source failure", func(t *testing.T) {
		svc, _, _ := newService(t, WithRandom(sequence()))
		_, err := svc.Shorten(context.Background(), ShortenInput{URL: "https://example.com"})
		if !errors.Is(err, io.EOF) {
			t.Errorf("error = %v, want it to wrap io.EOF", err)
		}
		if errors.Is(err, ErrCodeGeneration) {
			t.Error("a random source failure must not look like a collision streak")
		}
	})

	t.Run("store failure", func(t *testing.T) {
		svc, err := New(failingStore{Store: store.NewMemory(), err: boom}, WithRandom(sequence(1)))
		if err != nil {
			t.Fatal(err)
		}
		_, err = svc.Shorten(context.Background(), ShortenInput{URL: "https://example.com"})
		if !errors.Is(err, boom) {
			t.Errorf("error = %v, want %v", err, boom)
		}
	})
}

func TestShortenDefaultRandomProducesValidCodes(t *testing.T) {
	svc, _, _ := newService(t, WithCodeLength(9))

	const workers = 16
	codes := make(chan string, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			link, err := svc.Shorten(context.Background(), ShortenInput{URL: "https://example.com"})
			if err != nil {
				t.Errorf("Shorten() failed: %v", err)
				return
			}
			codes <- link.Code
		}()
	}
	wg.Wait()
	close(codes)

	seen := make(map[string]bool)
	for code := range codes {
		if len(code) != 9 || !base62.IsValid(code) {
			t.Errorf("code %q is not 9 base62 characters", code)
		}
		if seen[code] {
			t.Errorf("code %q was drawn twice", code)
		}
		seen[code] = true
	}
}

func TestResolve(t *testing.T) {
	svc, _, clock := newService(t, WithRandom(sequence(1, 2)))
	ctx := context.Background()

	forever, err := svc.Shorten(ctx, ShortenInput{URL: "https://example.com/forever"})
	if err != nil {
		t.Fatal(err)
	}
	temp, err := svc.Shorten(ctx, ShortenInput{URL: "https://example.com/temp", TTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("records hits and last access", func(t *testing.T) {
		clock.Advance(10 * time.Second)
		for want := int64(1); want <= 3; want++ {
			got, err := svc.Resolve(ctx, forever.Code)
			if err != nil {
				t.Fatalf("Resolve() failed: %v", err)
			}
			if got.URL != forever.URL || got.Hits != want {
				t.Errorf("Resolve() = %q with %d hits, want %q with %d", got.URL, got.Hits, forever.URL, want)
			}
		}
		stats, err := svc.Stats(ctx, forever.Code)
		if err != nil {
			t.Fatal(err)
		}
		if !stats.LastAccessAt.Equal(t0.Add(10 * time.Second)) {
			t.Errorf("LastAccessAt = %v, want %v", stats.LastAccessAt, t0.Add(10*time.Second))
		}
	})

	t.Run("unknown code", func(t *testing.T) {
		if _, err := svc.Resolve(ctx, "nope123"); !errors.Is(err, ErrNotFound) {
			t.Errorf("error = %v, want ErrNotFound", err)
		}
	})

	t.Run("expired link is refused and not counted", func(t *testing.T) {
		clock.Advance(time.Minute) // t0 + 70s, past temp's expiry at t0 + 60s
		if _, err := svc.Resolve(ctx, temp.Code); !errors.Is(err, ErrExpired) {
			t.Fatalf("error = %v, want ErrExpired", err)
		}
		stats, err := svc.Stats(ctx, temp.Code)
		if err != nil {
			t.Fatal(err)
		}
		if stats.Hits != 0 {
			t.Errorf("Hits = %d, want 0 for an expired link", stats.Hits)
		}
	})

	t.Run("link without expiry keeps working", func(t *testing.T) {
		clock.Advance(1000 * time.Hour)
		if _, err := svc.Resolve(ctx, forever.Code); err != nil {
			t.Errorf("Resolve() failed: %v", err)
		}
	})
}

func TestResolveExpiryBoundary(t *testing.T) {
	svc, _, clock := newService(t, WithRandom(sequence(1)))
	ctx := context.Background()
	link, err := svc.Shorten(ctx, ShortenInput{URL: "https://example.com", TTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}

	clock.Advance(time.Minute - time.Nanosecond)
	if _, err := svc.Resolve(ctx, link.Code); err != nil {
		t.Fatalf("link must still be valid one nanosecond before expiry: %v", err)
	}
	clock.Advance(time.Nanosecond)
	if _, err := svc.Resolve(ctx, link.Code); !errors.Is(err, ErrExpired) {
		t.Fatalf("error = %v, want ErrExpired exactly at expiry", err)
	}
}

func TestStats(t *testing.T) {
	svc, _, clock := newService(t, WithRandom(sequence(1, 2)))
	ctx := context.Background()
	open, err := svc.Shorten(ctx, ShortenInput{URL: "https://example.com/open"})
	if err != nil {
		t.Fatal(err)
	}
	temp, err := svc.Shorten(ctx, ShortenInput{URL: "https://example.com/temp", TTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Resolve(ctx, temp.Code); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		advance     time.Duration
		code        string
		wantHits    int64
		wantExpired bool
		wantErr     error
	}{
		{name: "fresh link without hits", code: open.Code},
		{name: "link with one hit", code: temp.Code, wantHits: 1},
		{name: "unknown code", code: "nope123", wantErr: ErrNotFound},
		{name: "expired link still has stats", advance: 2 * time.Hour, code: temp.Code, wantHits: 1, wantExpired: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clock.Advance(tt.advance)
			got, err := svc.Stats(ctx, tt.code)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				return
			}
			if got.Hits != tt.wantHits {
				t.Errorf("Hits = %d, want %d", got.Hits, tt.wantHits)
			}
			if got.Expired != tt.wantExpired {
				t.Errorf("Expired = %v, want %v", got.Expired, tt.wantExpired)
			}
		})
	}
}

func TestStatsDoesNotRecordHits(t *testing.T) {
	svc, _, _ := newService(t, WithRandom(sequence(1)))
	ctx := context.Background()
	link, err := svc.Shorten(ctx, ShortenInput{URL: "https://example.com"})
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if _, err := svc.Stats(ctx, link.Code); err != nil {
			t.Fatal(err)
		}
	}
	got, err := svc.Stats(ctx, link.Code)
	if err != nil {
		t.Fatal(err)
	}
	if got.Hits != 0 {
		t.Errorf("Hits = %d, want 0", got.Hits)
	}
}
