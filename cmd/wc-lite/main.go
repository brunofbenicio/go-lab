// Command wc-lite counts lines, words, characters and bytes, like a small
// version of wc(1).
package main

import "os"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
