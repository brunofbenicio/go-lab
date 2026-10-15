// Package bloom implements a Bloom filter: a compact probabilistic set that
// never reports a false negative but may report a false positive with a
// configurable probability.
//
// It is useful as a cheap pre-check before an expensive lookup (disk, network,
// database). A Filter is not safe for concurrent use.
package bloom

import (
	"hash/fnv"
	"math"
)

// Filter is a Bloom filter. Elements cannot be removed.
type Filter struct {
	bits []uint64
	m    uint64 // number of bits
	k    uint   // number of hash functions
}

// New creates a filter sized for about expected elements at a false positive
// rate of fpRate (for example 0.01 for 1%). It panics if fpRate is not in
// (0, 1).
func New(expected int, fpRate float64) *Filter {
	if !(fpRate > 0 && fpRate < 1) {
		panic("bloom: fpRate must be in (0, 1)")
	}
	n := float64(max(expected, 1))
	m := math.Ceil(-n * math.Log(fpRate) / (math.Ln2 * math.Ln2))
	k := math.Round(m / n * math.Ln2)
	return NewWithParams(uint64(m), uint(max(k, 1)))
}

// NewWithParams creates a filter with exactly m bits and k hash functions.
// Both are raised to at least 1.
func NewWithParams(m uint64, k uint) *Filter {
	m, k = max(m, 1), max(k, 1)
	return &Filter{bits: make([]uint64, (m+63)/64), m: m, k: k}
}

// Bits returns the filter size in bits.
func (f *Filter) Bits() uint64 { return f.m }

// Hashes returns the number of hash functions applied per element.
func (f *Filter) Hashes() uint { return f.k }

// Add inserts data into the filter.
func (f *Filter) Add(data []byte) {
	h1, h2 := hashes(data)
	for i := uint(0); i < f.k; i++ {
		pos := (h1 + uint64(i)*h2) % f.m
		f.bits[pos/64] |= 1 << (pos % 64)
	}
}

// AddString inserts s into the filter.
func (f *Filter) AddString(s string) { f.Add([]byte(s)) }

// MayContain reports whether data might have been added. A false result is
// definitive; a true result may be a false positive.
func (f *Filter) MayContain(data []byte) bool {
	h1, h2 := hashes(data)
	for i := uint(0); i < f.k; i++ {
		pos := (h1 + uint64(i)*h2) % f.m
		if f.bits[pos/64]&(1<<(pos%64)) == 0 {
			return false
		}
	}
	return true
}

// MayContainString is the string variant of MayContain.
func (f *Filter) MayContainString(s string) bool { return f.MayContain([]byte(s)) }

// hashes derives two independent 64-bit hashes from data; the i-th index is
// then h1 + i*h2 (double hashing). h2 is forced odd so that it is coprime with
// power-of-two sizes and never zero.
func hashes(data []byte) (h1, h2 uint64) {
	h := fnv.New64a()
	h.Write(data)
	h1 = h.Sum64()
	h.Write([]byte{0xff})
	return h1, h.Sum64() | 1
}
