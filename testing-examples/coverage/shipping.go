// Package shipping computes parcel shipping fees. The rules have enough
// branches (validation, zones, weight tiers, express, free shipping) to make
// the coverage report interesting to read.
package shipping

import (
	"errors"
	"fmt"
)

// Zone is a destination region.
type Zone int

// Supported zones.
const (
	ZoneLocal Zone = iota
	ZoneRegional
	ZoneInternational
)

// String returns the zone name.
func (z Zone) String() string {
	switch z {
	case ZoneLocal:
		return "local"
	case ZoneRegional:
		return "regional"
	case ZoneInternational:
		return "international"
	default:
		return fmt.Sprintf("zone(%d)", int(z))
	}
}

// Errors returned by Fee.
var (
	ErrInvalidWeight = errors.New("shipping: weight must be between 1 g and 30 kg")
	ErrUnknownZone   = errors.New("shipping: unknown zone")
)

const (
	maxWeight       = 30_000 // grams
	freeUnderWeight = 500    // local, standard parcels up to this weight ship free
)

// Fee returns the shipping fee in cents.
//
// The base price depends on the zone. Each started kilogram above the first
// adds a surcharge (higher for international parcels). Express delivery adds
// 50%. Light local parcels sent with standard delivery are free.
func Fee(weightGrams int, zone Zone, express bool) (int64, error) {
	if weightGrams <= 0 || weightGrams > maxWeight {
		return 0, ErrInvalidWeight
	}

	var base, perKg int64
	switch zone {
	case ZoneLocal:
		base, perKg = 500, 100
		if !express && weightGrams <= freeUnderWeight {
			return 0, nil
		}
	case ZoneRegional:
		base, perKg = 900, 100
	case ZoneInternational:
		base, perKg = 2500, 400
	default:
		return 0, fmt.Errorf("%w: %v", ErrUnknownZone, zone)
	}

	fee := base
	if extraKg := (weightGrams - 1) / 1000; extraKg > 0 { // started kilograms beyond the first
		fee += int64(extraKg) * perKg
	}
	if express {
		fee += fee / 2
	}
	return fee, nil
}
