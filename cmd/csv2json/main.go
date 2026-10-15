// Command csv2json converts CSV data into a JSON array of objects.
package main

import "os"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
