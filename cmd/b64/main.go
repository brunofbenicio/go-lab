// Command b64 encodes and decodes Base64 from a file or standard input.
package main

import "os"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
