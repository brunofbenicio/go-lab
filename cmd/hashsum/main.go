// Command hashsum prints the MD5, SHA-1 or SHA-256 digest of files or
// standard input, in the same format as md5sum and sha256sum.
package main

import "os"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
