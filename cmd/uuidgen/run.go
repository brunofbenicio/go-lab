package main

import (
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

// maxCount bounds -n so a typo cannot flood the terminal.
const maxCount = 1000

// randSource supplies the random bytes. It is a variable so tests can use a
// predictable reader.
var randSource io.Reader = rand.Reader

// newUUID reads 16 random bytes from r and returns them as a version 4 UUID in
// the canonical 8-4-4-4-12 lower-case form.
func newUUID(r io.Reader) (string, error) {
	var b [16]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return "", fmt.Errorf("reading random bytes: %w", err)
	}
	b[6] = b[6]&0x0f | 0x40 // version 4
	b[8] = b[8]&0x3f | 0x80 // RFC 4122 variant
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

// run is the testable entry point. It returns the process exit status:
// 0 on success, 1 if random bytes could not be read, 2 for usage errors.
func run(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("uuidgen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: uuidgen [-n count] [-upper] [-no-dashes]")
		fs.PrintDefaults()
	}
	n := fs.Int("n", 1, fmt.Sprintf("how many UUIDs to print (1-%d)", maxCount))
	upper := fs.Bool("upper", false, "print upper-case hex digits")
	noDashes := fs.Bool("no-dashes", false, "omit the dashes")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "uuidgen: unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	if *n < 1 || *n > maxCount {
		fmt.Fprintf(stderr, "uuidgen: -n must be between 1 and %d\n", maxCount)
		return 2
	}

	// Build everything first so a failure never leaves partial output behind.
	var out strings.Builder
	for i := 0; i < *n; i++ {
		id, err := newUUID(randSource)
		if err != nil {
			fmt.Fprintf(stderr, "uuidgen: %v\n", err)
			return 1
		}
		if *noDashes {
			id = strings.ReplaceAll(id, "-", "")
		}
		if *upper {
			id = strings.ToUpper(id)
		}
		out.WriteString(id)
		out.WriteByte('\n')
	}
	if _, err := io.WriteString(stdout, out.String()); err != nil {
		fmt.Fprintf(stderr, "uuidgen: writing output: %v\n", err)
		return 1
	}
	return 0
}
