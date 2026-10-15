package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"unicode"
)

// counts holds the totals for one input.
type counts struct {
	lines, words, chars, bytes int
}

func (c *counts) add(o counts) {
	c.lines += o.lines
	c.words += o.words
	c.chars += o.chars
	c.bytes += o.bytes
}

// selection says which columns to print.
type selection struct {
	lines, words, chars, bytes bool
}

// values returns the selected counts in the fixed order lines, words, chars,
// bytes.
func (s selection) values(c counts) []int {
	var out []int
	if s.lines {
		out = append(out, c.lines)
	}
	if s.words {
		out = append(out, c.words)
	}
	if s.chars {
		out = append(out, c.chars)
	}
	if s.bytes {
		out = append(out, c.bytes)
	}
	return out
}

// count reads r to the end. Lines are newline characters, words are runs of
// non-space characters, and invalid UTF-8 bytes count as one character each.
func count(r io.Reader) (counts, error) {
	var c counts
	br := bufio.NewReader(r)
	inWord := false
	for {
		ch, size, err := br.ReadRune()
		if errors.Is(err, io.EOF) {
			return c, nil
		}
		if err != nil {
			return c, err
		}
		c.bytes += size
		c.chars++
		if ch == '\n' {
			c.lines++
		}
		if unicode.IsSpace(ch) {
			inWord = false
		} else if !inWord {
			inWord = true
			c.words++
		}
	}
}

// countNamed counts the file called name, or stdin when name is "-".
func countNamed(name string, stdin io.Reader) (counts, error) {
	if name == "-" {
		return count(stdin)
	}
	f, err := os.Open(name)
	if err != nil {
		return counts{}, err
	}
	defer f.Close()
	c, err := count(f)
	if err != nil {
		return c, fmt.Errorf("%s: %w", name, err)
	}
	return c, nil
}

// run is the testable entry point. It returns the process exit status:
// 0 on success, 1 if some input could not be read, 2 for usage errors.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("wc-lite", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: wc-lite [-l] [-w] [-m] [-c] [file ...]")
		fmt.Fprintln(stderr, "With no file, or with the file -, standard input is read.")
		fs.PrintDefaults()
	}
	var sel selection
	fs.BoolVar(&sel.lines, "l", false, "print the line count")
	fs.BoolVar(&sel.words, "w", false, "print the word count")
	fs.BoolVar(&sel.chars, "m", false, "print the character count")
	fs.BoolVar(&sel.bytes, "c", false, "print the byte count")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if sel == (selection{}) {
		sel = selection{lines: true, words: true, bytes: true}
	}

	names := fs.Args()
	implicitStdin := len(names) == 0
	if implicitStdin {
		names = []string{"-"}
	}

	type result struct {
		name string
		c    counts
	}
	var results []result
	var total counts
	status := 0
	for _, name := range names {
		c, err := countNamed(name, stdin)
		if err != nil {
			fmt.Fprintf(stderr, "wc-lite: %v\n", err)
			status = 1
			continue
		}
		results = append(results, result{name, c})
		total.add(c)
	}
	if len(results) > 1 {
		results = append(results, result{"total", total})
	}

	// Right-align every column to the widest number so the output lines up.
	width := 1
	for _, r := range results {
		for _, v := range sel.values(r.c) {
			width = max(width, len(strconv.Itoa(v)))
		}
	}
	for _, r := range results {
		for i, v := range sel.values(r.c) {
			if i > 0 {
				fmt.Fprint(stdout, " ")
			}
			fmt.Fprintf(stdout, "%*d", width, v)
		}
		if !implicitStdin {
			fmt.Fprintf(stdout, " %s", r.name)
		}
		fmt.Fprintln(stdout)
	}
	return status
}
