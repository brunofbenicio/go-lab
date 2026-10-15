package main

import (
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// pickEncoding chooses the alphabet and padding for the given options.
func pickEncoding(urlSafe, raw bool) *base64.Encoding {
	switch {
	case urlSafe && raw:
		return base64.RawURLEncoding
	case urlSafe:
		return base64.URLEncoding
	case raw:
		return base64.RawStdEncoding
	default:
		return base64.StdEncoding
	}
}

// dropWhitespace removes spaces and line breaks, which may appear when Base64
// text was wrapped by another tool.
func dropWhitespace(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\r', '\n':
			return -1
		}
		return r
	}, s)
}

// run is the testable entry point. It returns the process exit status:
// 0 on success, 1 for unreadable or invalid input, 2 for usage errors.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("b64", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: b64 [-d] [-url] [-raw] [file]")
		fmt.Fprintln(stderr, "With no file, or with the file -, standard input is read.")
		fs.PrintDefaults()
	}
	decode := fs.Bool("d", false, "decode instead of encode")
	urlSafe := fs.Bool("url", false, "use the URL-safe alphabet (- and _ instead of + and /)")
	raw := fs.Bool("raw", false, "no '=' padding (omit it when encoding, do not expect it when decoding)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	operands := fs.Args()
	if len(operands) > 1 {
		fmt.Fprintln(stderr, "b64: at most one file may be given")
		return 2
	}

	in := stdin
	if len(operands) == 1 && operands[0] != "-" {
		f, err := os.Open(operands[0])
		if err != nil {
			fmt.Fprintf(stderr, "b64: %v\n", err)
			return 1
		}
		defer f.Close()
		in = f
	}
	data, err := io.ReadAll(in)
	if err != nil {
		fmt.Fprintf(stderr, "b64: reading input: %v\n", err)
		return 1
	}

	enc := pickEncoding(*urlSafe, *raw)
	var out []byte
	if *decode {
		out, err = enc.DecodeString(dropWhitespace(string(data)))
		if err != nil {
			fmt.Fprintf(stderr, "b64: invalid base64 input: %v\n", err)
			return 1
		}
	} else {
		out = append([]byte(enc.EncodeToString(data)), '\n')
	}
	if _, err := stdout.Write(out); err != nil {
		fmt.Fprintf(stderr, "b64: writing output: %v\n", err)
		return 1
	}
	return 0
}
