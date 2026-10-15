package main

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"
)

// algorithms lists the supported digests. MD5 and SHA-1 are kept for checking
// legacy checksums only; they are not collision resistant.
var algorithms = map[string]func() hash.Hash{
	"md5":    md5.New,
	"sha1":   sha1.New,
	"sha256": sha256.New,
}

// sumNamed hashes the file called name, or stdin when name is "-".
func sumNamed(h hash.Hash, name string, stdin io.Reader) (string, error) {
	in := stdin
	if name != "-" {
		f, err := os.Open(name)
		if err != nil {
			return "", err
		}
		defer f.Close()
		in = f
	}
	if _, err := io.Copy(h, in); err != nil {
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// run is the testable entry point. It returns the process exit status:
// 0 on success, 1 if some input could not be read, 2 for usage errors.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("hashsum", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: hashsum [-a md5|sha1|sha256] [file ...]")
		fmt.Fprintln(stderr, "With no file, or with the file -, standard input is read.")
		fs.PrintDefaults()
	}
	algo := fs.String("a", "sha256", "algorithm: md5, sha1 or sha256")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	newHash, ok := algorithms[strings.ToLower(*algo)]
	if !ok {
		fmt.Fprintf(stderr, "hashsum: unknown algorithm %q (use md5, sha1 or sha256)\n", *algo)
		return 2
	}

	names := fs.Args()
	if len(names) == 0 {
		names = []string{"-"}
	}
	status := 0
	for _, name := range names {
		sum, err := sumNamed(newHash(), name, stdin)
		if err != nil {
			fmt.Fprintf(stderr, "hashsum: %v\n", err)
			status = 1
			continue
		}
		fmt.Fprintf(stdout, "%s  %s\n", sum, name)
	}
	return status
}
