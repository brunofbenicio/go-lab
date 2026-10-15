// Command minigrep prints the lines of its input that match a regular
// expression, a tiny version of grep.
//
//	minigrep [-i] [-n] [-c] PATTERN [FILE...]
//
// With no FILE (or with "-") it reads standard input. The exit status is 0 if
// any line matched, 1 if none did and 2 if an error occurred.
//
// main only wires the real process streams into run, which holds all the
// logic. That split is what makes the program testable: tests call run with
// in-memory readers and writers instead of spawning a process.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
)

const usage = "usage: minigrep [-i] [-n] [-c] PATTERN [FILE...]"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// options are the parsed command-line switches.
type options struct {
	lineNumbers bool
	count       bool
	prefix      bool // print the file name before each result
}

// run executes the program and returns its exit status. It never touches the
// global os.Args, os.Stdin, os.Stdout, os.Stderr or os.Exit.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	// A private FlagSet (instead of the global flag.CommandLine) keeps runs
	// independent from each other, and ContinueOnError returns errors rather
	// than calling os.Exit.
	fs := flag.NewFlagSet("minigrep", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, usage)
		fs.PrintDefaults()
	}
	ignoreCase := fs.Bool("i", false, "ignore case")
	lineNumbers := fs.Bool("n", false, "prefix each line with its line number")
	count := fs.Bool("c", false, "print only the number of matching lines")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2 // the flag package already explained the problem on stderr
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "minigrep: missing PATTERN")
		fs.Usage()
		return 2
	}

	pattern := fs.Arg(0)
	if *ignoreCase {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		fmt.Fprintf(stderr, "minigrep: invalid pattern: %v\n", err)
		return 2
	}

	files := fs.Args()[1:]
	opts := options{lineNumbers: *lineNumbers, count: *count, prefix: len(files) > 1}
	if len(files) == 0 {
		files = []string{"-"}
	}

	matched, failed := false, false
	for _, name := range files {
		n, err := searchInput(name, stdin, stdout, re, opts)
		if err != nil {
			fmt.Fprintf(stderr, "minigrep: %v\n", err)
			failed = true
			continue
		}
		matched = matched || n > 0
	}
	switch {
	case failed:
		return 2
	case matched:
		return 0
	}
	return 1
}

// searchInput opens name ("-" means stdin) and searches it.
func searchInput(name string, stdin io.Reader, stdout io.Writer, re *regexp.Regexp, opts options) (int, error) {
	r := stdin
	if name != "-" {
		f, err := os.Open(name)
		if err != nil {
			return 0, err
		}
		defer f.Close()
		r = f
	}
	return search(r, name, stdout, re, opts)
}

// search prints the matching lines of r and returns how many there were.
func search(r io.Reader, name string, stdout io.Writer, re *regexp.Regexp, opts options) (int, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)

	matches := 0
	for line := 1; sc.Scan(); line++ {
		if !re.Match(sc.Bytes()) {
			continue
		}
		matches++
		if opts.count {
			continue
		}
		if opts.prefix {
			fmt.Fprintf(stdout, "%s:", name)
		}
		if opts.lineNumbers {
			fmt.Fprintf(stdout, "%d:", line)
		}
		fmt.Fprintf(stdout, "%s\n", sc.Text())
	}
	if err := sc.Err(); err != nil {
		return matches, fmt.Errorf("%s: %w", name, err)
	}
	if opts.count {
		if opts.prefix {
			fmt.Fprintf(stdout, "%s:", name)
		}
		fmt.Fprintln(stdout, matches)
	}
	return matches, nil
}
