# Table-driven tests

## What it solves

Testing the same function with many inputs usually leads to copy-pasted test
functions. A table-driven test keeps the scenarios as data (one row per case)
and the assertions in a single loop, so adding a case is a one-line change.

## When to use

- Pure or nearly pure functions with many input/output combinations.
- Error paths: list the invalid inputs next to the valid ones.
- Anywhere you catch yourself copying a test and changing two values.

## What this example shows

- `Parse` tested with a slice of structs (`name`, `input`, `want`, `wantErr`).
- `Format` tested with a `map[string]struct{...}`, where the key is the name.
- One `t.Run` per row, so a failure names the exact case.
- Errors compared with `errors.Is`, not by message text.

## Running

```sh
go test ./testing-examples/table-driven/
go test -run 'TestParse/IEC' -v ./testing-examples/table-driven/   # a single row
```
