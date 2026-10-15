package semver_test

import (
	"fmt"
	"slices"

	"github.com/brunofbenicio/go-lab/pkg/semver"
)

func Example() {
	current := semver.MustParse("v1.9.0")
	candidate, _ := semver.Parse("1.10.0-rc.1")

	fmt.Println(candidate.Compare(current)) // 1.10.0-rc.1 is newer than 1.9.0
	fmt.Println(current.IncMinor())

	versions := []semver.Version{
		semver.MustParse("1.0.0"), semver.MustParse("1.0.0-alpha"), semver.MustParse("0.9.0"),
	}
	slices.SortFunc(versions, semver.Version.Compare)
	fmt.Println(versions)
	// Output:
	// 1
	// 1.10.0
	// [0.9.0 1.0.0-alpha 1.0.0]
}
