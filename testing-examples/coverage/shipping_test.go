package shipping

import (
	"errors"
	"testing"
)

func TestFee(t *testing.T) {
	tests := []struct {
		name    string
		weight  int
		zone    Zone
		express bool
		want    int64
		wantErr error
	}{
		// Validation.
		{name: "zero weight", weight: 0, zone: ZoneLocal, wantErr: ErrInvalidWeight},
		{name: "negative weight", weight: -1, zone: ZoneLocal, wantErr: ErrInvalidWeight},
		{name: "above the maximum", weight: 30_001, zone: ZoneLocal, wantErr: ErrInvalidWeight},
		{name: "unknown zone", weight: 1000, zone: Zone(9), wantErr: ErrUnknownZone},

		// Free shipping and its boundaries.
		{name: "light local parcel is free", weight: 500, zone: ZoneLocal, want: 0},
		{name: "just above the free limit", weight: 501, zone: ZoneLocal, want: 500},
		{name: "express is never free", weight: 500, zone: ZoneLocal, express: true, want: 750},

		// Base prices per zone, first kilogram included.
		{name: "regional 1 kg", weight: 1000, zone: ZoneRegional, want: 900},
		{name: "international 1 kg", weight: 1000, zone: ZoneInternational, want: 2500},

		// Weight tiers: started kilograms beyond the first.
		{name: "1 kg + 1 g is two kilograms", weight: 1001, zone: ZoneRegional, want: 1000},
		{name: "local 3.5 kg", weight: 3500, zone: ZoneLocal, want: 800},
		{name: "international 3.5 kg", weight: 3500, zone: ZoneInternational, want: 3700},
		{name: "heaviest allowed", weight: 30_000, zone: ZoneRegional, want: 3800},

		// Express surcharge.
		{name: "express regional", weight: 1000, zone: ZoneRegional, express: true, want: 1350},
		{name: "express international 3.5 kg", weight: 3500, zone: ZoneInternational, express: true, want: 5550},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Fee(tt.weight, tt.zone, tt.express)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Fee() error = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Fee() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestZoneString(t *testing.T) {
	for zone, want := range map[Zone]string{
		ZoneLocal:         "local",
		ZoneRegional:      "regional",
		ZoneInternational: "international",
		Zone(7):           "zone(7)",
	} {
		if got := zone.String(); got != want {
			t.Errorf("Zone(%d).String() = %q, want %q", int(zone), got, want)
		}
	}
}
