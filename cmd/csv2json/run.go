package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"unicode/utf8"
)

// parseDelimiter accepts a single character, or the two characters \t for a
// tab, which is hard to type in a shell.
func parseDelimiter(s string) (rune, error) {
	if s == `\t` {
		return '\t', nil
	}
	r, size := utf8.DecodeRuneInString(s)
	if s == "" || size != len(s) || r == utf8.RuneError {
		return 0, fmt.Errorf("-d must be a single character, got %q", s)
	}
	if r == '"' || r == '\r' || r == '\n' {
		return 0, fmt.Errorf("-d cannot be %q", r)
	}
	return r, nil
}

// run is the testable entry point. It returns the process exit status:
// 0 on success, 1 for unreadable or malformed input, 2 for usage errors.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("csv2json", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: csv2json [-d delim] [-no-header] [-types] [-compact] [file]")
		fmt.Fprintln(stderr, "With no file, or with the file -, standard input is read.")
		fs.PrintDefaults()
	}
	delim := fs.String("d", ",", `field delimiter: one character, or \t for a tab`)
	noHeader := fs.Bool("no-header", false, "treat the first row as data and name columns col1, col2, ...")
	types := fs.Bool("types", false, "write numbers, true/false and empty cells as JSON numbers, booleans and null")
	compact := fs.Bool("compact", false, "print the JSON on a single line")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	comma, err := parseDelimiter(*delim)
	if err != nil {
		fmt.Fprintf(stderr, "csv2json: %v\n", err)
		return 2
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(stderr, "csv2json: at most one file may be given")
		return 2
	}

	in := stdin
	if fs.NArg() == 1 && fs.Arg(0) != "-" {
		f, err := os.Open(fs.Arg(0))
		if err != nil {
			fmt.Fprintf(stderr, "csv2json: %v\n", err)
			return 1
		}
		defer f.Close()
		in = f
	}

	out, err := convert(in, options{comma: comma, noHeader: *noHeader, types: *types, compact: *compact})
	if err != nil {
		fmt.Fprintf(stderr, "csv2json: %v\n", err)
		return 1
	}
	if _, err := fmt.Fprintf(stdout, "%s\n", out); err != nil {
		fmt.Fprintf(stderr, "csv2json: writing output: %v\n", err)
		return 1
	}
	return 0
}
