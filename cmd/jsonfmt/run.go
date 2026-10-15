package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// maxIndent bounds -indent to something sensible.
const maxIndent = 16

// run is the testable entry point. It returns the process exit status:
// 0 on success, 1 for unreadable or invalid input, 2 for usage errors.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("jsonfmt", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: jsonfmt [-m] [-indent N] [file]")
		fmt.Fprintln(stderr, "With no file, or with the file -, standard input is read.")
		fs.PrintDefaults()
	}
	minify := fs.Bool("m", false, "minify instead of pretty-printing")
	indent := fs.Int("indent", 2, "spaces per indentation level when pretty-printing (0-16)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *indent < 0 || *indent > maxIndent {
		fmt.Fprintf(stderr, "jsonfmt: -indent must be between 0 and %d\n", maxIndent)
		return 2
	}
	operands := fs.Args()
	if len(operands) > 1 {
		fmt.Fprintln(stderr, "jsonfmt: at most one file may be given")
		return 2
	}

	in := stdin
	if len(operands) == 1 && operands[0] != "-" {
		f, err := os.Open(operands[0])
		if err != nil {
			fmt.Fprintf(stderr, "jsonfmt: %v\n", err)
			return 1
		}
		defer f.Close()
		in = f
	}
	data, err := io.ReadAll(in)
	if err != nil {
		fmt.Fprintf(stderr, "jsonfmt: reading input: %v\n", err)
		return 1
	}
	// Surrounding whitespace is dropped so the output ends in exactly one newline.
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		fmt.Fprintln(stderr, "jsonfmt: empty input")
		return 1
	}

	// Compact and Indent work on the raw text, so object keys keep their order.
	var out bytes.Buffer
	if *minify {
		err = json.Compact(&out, data)
	} else {
		err = json.Indent(&out, data, "", strings.Repeat(" ", *indent))
	}
	if err != nil {
		fmt.Fprintf(stderr, "jsonfmt: invalid JSON: %v\n", err)
		return 1
	}
	out.WriteByte('\n')
	if _, err := stdout.Write(out.Bytes()); err != nil {
		fmt.Fprintf(stderr, "jsonfmt: writing output: %v\n", err)
		return 1
	}
	return 0
}
