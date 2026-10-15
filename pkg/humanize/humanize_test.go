package humanize

import (
	"math"
	"testing"
	"time"
)

func TestBytes(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{999, "999 B"},
		{1000, "1 kB"},
		{1500, "1.5 kB"},
		{999_949, "999.9 kB"},
		{999_999, "1 MB"}, // rounds up into the next unit
		{1_000_000, "1 MB"},
		{2_500_000_000, "2.5 GB"},
		{-1500, "-1.5 kB"},
		{math.MaxInt64, "9.2 EB"},
		{math.MinInt64, "-9.2 EB"},
	}
	for _, tt := range tests {
		if got := Bytes(tt.in); got != tt.want {
			t.Errorf("Bytes(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestIBytes(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1 KiB"},
		{1536, "1.5 KiB"},
		{1 << 20, "1 MiB"},
		{5 << 30, "5 GiB"},
		{-2048, "-2 KiB"},
		{math.MaxInt64, "8 EiB"},
	}
	for _, tt := range tests {
		if got := IBytes(tt.in); got != tt.want {
			t.Errorf("IBytes(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestDuration(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		{0, "0s"},
		{500 * time.Nanosecond, "500ns"},
		{250 * time.Millisecond, "250ms"},
		{time.Second, "1s"},
		{1500 * time.Millisecond, "1s"},
		{90 * time.Second, "1m 30s"},
		{3*time.Hour + 5*time.Second, "3h 5s"},
		{36 * time.Hour, "1d 12h"},
		{49*time.Hour + 30*time.Minute, "2d 1h 30m"},
		{-90 * time.Second, "-1m 30s"},
		{-250 * time.Millisecond, "-250ms"},
		{math.MaxInt64, "106751d 23h 47m 16s"},
		{math.MinInt64, "-106751d 23h 47m 16s"},
	}
	for _, tt := range tests {
		if got := Duration(tt.in); got != tt.want {
			t.Errorf("Duration(%v) = %q, want %q", int64(tt.in), got, tt.want)
		}
	}
}

func TestThousands(t *testing.T) {
	tests := []struct {
		in   int64
		sep  string
		want string
	}{
		{0, ",", "0"},
		{999, ",", "999"},
		{1000, ",", "1,000"},
		{1234567, ".", "1.234.567"},
		{-1234567, ".", "-1.234.567"},
		{100000, " ", "100 000"},
		{math.MinInt64, ",", "-9,223,372,036,854,775,808"},
	}
	for _, tt := range tests {
		if got := Thousands(tt.in, tt.sep); got != tt.want {
			t.Errorf("Thousands(%d, %q) = %q, want %q", tt.in, tt.sep, got, tt.want)
		}
	}
}
