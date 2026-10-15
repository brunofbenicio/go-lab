package semver_test

import (
	"fmt"

	semver "github.com/brunofbenicio/go-lab/testing-examples/examples"
)

// The package is declared as semver_test (an external test package) so the
// examples use the API exactly as a user would, with the package qualifier.
// Example functions are compiled and executed by `go test`, and their output
// is compared with the "// Output:" comment. They also appear in godoc.

// The function named just "Example" documents the package as a whole.
func Example() {
	v, err := semver.Parse("v1.4.2")
	if err != nil {
		panic(err)
	}
	next, _ := v.Bump("minor")
	fmt.Println(v, "->", next)
	// Output: 1.4.2 -> 1.5.0
}

// ExampleParse documents the function Parse.
func ExampleParse() {
	v, _ := semver.Parse("2.0.0-rc1")
	fmt.Printf("major=%d pre=%s\n", v.Major, v.Pre)
	// Output: major=2 pre=rc1
}

// ExampleParse_invalid is a suffixed example: it still documents Parse, and
// the suffix (lower-case) distinguishes variations.
func ExampleParse_invalid() {
	_, err := semver.Parse("1.2")
	fmt.Println(err)
	// Output: semver: invalid version: "1.2" needs three numeric parts
}

// ExampleVersion_Compare documents the method Compare of the type Version.
func ExampleVersion_Compare() {
	release, _ := semver.Parse("1.0.0")
	candidate, _ := semver.Parse("1.0.0-rc1")
	newer, _ := semver.Parse("1.0.1")

	fmt.Println(release.Compare(candidate)) // a release is newer than its pre-release
	fmt.Println(release.Compare(release))
	fmt.Println(release.Compare(newer))
	// Output:
	// 1
	// 0
	// -1
}

func ExampleSort() {
	var versions []semver.Version
	for _, s := range []string{"1.10.0", "1.2.0", "1.2.0-beta", "0.9.9"} {
		v, _ := semver.Parse(s)
		versions = append(versions, v)
	}

	semver.Sort(versions)

	fmt.Println(versions)
	// Output: [0.9.9 1.2.0-beta 1.2.0 1.10.0]
}

// Ranging over a map visits keys in random order, so a plain "Output:" would
// be flaky. "Unordered output:" compares the lines as a set instead.
func ExampleGroupByMajor() {
	var versions []semver.Version
	for _, s := range []string{"1.0.0", "2.1.0", "1.5.3"} {
		v, _ := semver.Parse(s)
		versions = append(versions, v)
	}

	for major, group := range semver.GroupByMajor(versions) {
		fmt.Printf("major %d: %v\n", major, group)
	}
	// Unordered output:
	// major 1: [1.0.0 1.5.3]
	// major 2: [2.1.0]
}
