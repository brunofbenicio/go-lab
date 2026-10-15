// Package caesar implements the Caesar cipher and a frequency-analysis
// attack that recovers the key from English ciphertext.
package caesar

import (
	"math"
	"strings"
)

// englishFreq holds the relative frequency (in percent) of the letters a..z
// in typical English text.
var englishFreq = [26]float64{
	8.167, 1.492, 2.782, 4.253, 12.702, 2.228, 2.015, 6.094, 6.966,
	0.153, 0.772, 4.025, 2.406, 6.749, 7.507, 1.929, 0.095, 5.987,
	6.327, 9.056, 2.758, 0.978, 2.360, 0.150, 1.974, 0.074,
}

// Shift moves every ASCII letter of s by k positions in the alphabet,
// preserving case. Other characters are left untouched. k may be negative or
// larger than 26.
func Shift(s string, k int) string {
	k = ((k % 26) + 26) % 26
	shift := rune(k)
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return 'a' + (r-'a'+shift)%26
		case r >= 'A' && r <= 'Z':
			return 'A' + (r-'A'+shift)%26
		}
		return r
	}, s)
}

// Encrypt encrypts plaintext with the given key.
func Encrypt(plaintext string, key int) string { return Shift(plaintext, key) }

// Decrypt reverses Encrypt.
func Decrypt(ciphertext string, key int) string { return Shift(ciphertext, -key) }

// Crack guesses the key used to encrypt an English text by choosing the shift
// whose letter distribution is closest (chi-squared) to English. It returns the
// key in 0..25 and the recovered plaintext. Short texts may yield wrong
// guesses; text without letters is returned unchanged with key 0.
func Crack(ciphertext string) (key int, plaintext string) {
	var counts [26]int
	total := 0
	for _, r := range ciphertext {
		switch {
		case r >= 'a' && r <= 'z':
			counts[r-'a']++
			total++
		case r >= 'A' && r <= 'Z':
			counts[r-'A']++
			total++
		}
	}
	if total == 0 {
		return 0, ciphertext
	}
	best := math.Inf(1)
	for k := 0; k < 26; k++ {
		var chi float64
		for i, c := range counts {
			// A cipher letter at index i came from plain letter i-k.
			expected := englishFreq[(i-k+26)%26] / 100 * float64(total)
			d := float64(c) - expected
			chi += d * d / expected
		}
		if chi < best {
			best, key = chi, k
		}
	}
	return key, Decrypt(ciphertext, key)
}
