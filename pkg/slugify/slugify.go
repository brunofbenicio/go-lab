// Package slugify turns arbitrary text into URL-friendly slugs: lowercase
// ASCII letters and digits separated by a single separator. Latin accents
// (including Portuguese ones) are transliterated, so "Ação e Reação" becomes
// "acao-e-reacao".
package slugify

import (
	"strings"
	"unicode"
)

var transliterate = strings.NewReplacer(
	"à", "a", "á", "a", "â", "a", "ã", "a", "ä", "a", "å", "a", "æ", "ae",
	"ç", "c", "è", "e", "é", "e", "ê", "e", "ë", "e",
	"ì", "i", "í", "i", "î", "i", "ï", "i", "ñ", "n",
	"ò", "o", "ó", "o", "ô", "o", "õ", "o", "ö", "o", "ø", "o", "œ", "oe",
	"ù", "u", "ú", "u", "û", "u", "ü", "u", "ý", "y", "ÿ", "y", "ß", "ss",
)

type options struct {
	sep    string
	maxLen int
}

// Option customizes Make.
type Option func(*options)

// WithSeparator sets the word separator (default "-"). It should be ASCII.
func WithSeparator(sep string) Option { return func(o *options) { o.sep = sep } }

// WithMaxLength limits the slug to n bytes, cutting at a word boundary when
// possible and never leaving a trailing separator. n <= 0 means no limit.
func WithMaxLength(n int) Option { return func(o *options) { o.maxLen = n } }

// Make converts s into a slug. Characters that are neither Latin letters nor
// digits (spaces, punctuation, symbols, other scripts) act as separators;
// apostrophes are dropped, so "don't" becomes "dont". Combining marks are
// discarded, which also handles decomposed (NFD) accents. The result is empty
// if s has nothing to keep.
func Make(s string, opts ...Option) string {
	o := options{sep: "-"}
	for _, opt := range opts {
		opt(&o)
	}

	var b strings.Builder
	pendingSep := false
	for _, r := range transliterate.Replace(strings.ToLower(s)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			if pendingSep && b.Len() > 0 {
				b.WriteString(o.sep)
			}
			pendingSep = false
			b.WriteRune(r)
		case r == '\'' || r == '’' || unicode.Is(unicode.Mn, r):
			// dropped without splitting the word
		default:
			pendingSep = true
		}
	}
	return cut(b.String(), o.sep, o.maxLen)
}

// cut shortens slug to at most n bytes at a separator boundary when it can.
func cut(slug, sep string, n int) string {
	if n <= 0 || len(slug) <= n {
		return slug
	}
	head := slug[:n]
	if !strings.HasPrefix(slug[n:], sep) { // the cut falls inside a word
		if i := strings.LastIndex(head, sep); i > 0 {
			head = head[:i]
		}
	}
	return strings.TrimSuffix(head, sep)
}
