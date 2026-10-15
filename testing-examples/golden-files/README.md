# Golden files with `testdata` and `-update`

## What it solves

When the output is large or visual (a rendered table, emitted source code, HTML,
JSON), writing the expected text inside the test is noisy and hard to review.
A *golden file* keeps the expected output in `testdata/`; the test compares
against it, and a flag regenerates it when the change is intentional.

## When to use

- Formatters, renderers, templates, code generators, CLI help text.
- Output where the exact bytes matter and a diff in code review is the best check.

For small values, assert directly in the test: it is clearer.

## What this example shows

- A `-update` flag defined with the `flag` package in the test file.
- `assertGolden`, which derives the file name from `t.Name()`, so each subtest
  gets `testdata/TestRender_<case>.golden` without any bookkeeping.
- A helpful failure message that prints both outputs and how to fix the golden file.
- Unicode and empty-input cases, each with its own golden file.

## Workflow

```sh
go test ./testing-examples/golden-files/                 # compare
go test ./testing-examples/golden-files/ -update         # rewrite goldens
git diff testing-examples/golden-files/testdata/         # review what changed
```

Never run `-update` blindly: a golden file accepted without reading the diff
turns the test into a rubber stamp. Go ignores directories named `testdata`
when building, so the files do not affect the package.
