// Command httpcheck requests URLs and reports whether each one answered with
// an acceptable HTTP status.
package main

import "os"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
