# Code coverage with `-cover` and `-coverprofile`

## What it solves

Coverage answers "which lines did my tests never execute?". It does not prove
the tests are good (a line can run without being checked), but an uncovered
branch is a guaranteed blind spot, so the report is a cheap map of where to
write the next test.

## When to use

- After writing tests for a function with many branches, to find forgotten ones.
- In CI as a trend or a floor, never as a goal in itself.
- To explore an unfamiliar codebase: what do the existing tests actually touch?

## What this example shows

`Fee` has validation, three zones, weight tiers, an express surcharge and a
free-shipping rule. The table-driven test has one row per branch and boundary
(500 g versus 501 g, 30,000 g versus 30,001 g), reaching 100% statement coverage.
Try deleting rows and watch the numbers drop.

## Commands

```sh
# Quick percentage
go test -cover ./testing-examples/coverage/

# Save a profile, then inspect it
go test -coverprofile=coverage.out ./testing-examples/coverage/
go tool cover -func=coverage.out          # per function, with the total
go tool cover -html=coverage.out          # opens an annotated source view
go tool cover -html=coverage.out -o coverage.html   # write to a file instead

# Modes: set (default), count (how many times), atomic (safe with -race)
go test -covermode=count -coverprofile=coverage.out ./testing-examples/coverage/

# Count code in other packages that the tests exercise indirectly
go test -coverpkg=./... -coverprofile=coverage.out ./...
```

`-race` implies `-covermode=atomic`. The repository's `.gitignore` already
excludes `coverage.*` and `*.out`, so profiles are never committed.

Since Go 1.20 you can also collect coverage from a running **binary**:
`go build -cover -o app .`, then `GOCOVERDIR=/tmp/cov ./app` and
`go tool covdata textfmt -i=/tmp/cov -o profile.txt`.

## Tips

- 100% is not the goal; boundaries and error paths matter more than the number.
- Read the HTML report: red blocks show exactly which branch was never taken.
