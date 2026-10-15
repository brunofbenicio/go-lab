# Property-based tests with `testing/quick`

## What it solves

Example-based tests check the cases you thought of. A property test states a
*law* that must hold for every input ("deduplicating twice equals
deduplicating once") and the framework tries hundreds of random inputs,
reporting a counterexample when one breaks the law. It complements tables;
it does not replace them.

## When to use

- Pure functions with algebraic laws: idempotence, commutativity, round trips,
  "output is sorted", "length is preserved".
- Comparing a fast implementation with a slow but obviously correct one.
- Data structure invariants.

## What this example shows

- `quick.Check` with properties for `Dedup` and `Merge`.
- A seeded `*rand.Rand` in `quick.Config`, so runs are reproducible.
- **Custom generators** (`quick.Generator`): `smallInts` forces collisions and
  `sortedInts` satisfies `Merge`'s precondition. Without them the default
  generator makes properties pass vacuously or violate preconditions.
- `quick.CheckEqual` for differential testing against a reference.
- Checking the checker: a false property must yield a `*quick.CheckError`
  carrying the failing input.

## Tips

- Keep each property small and named after the law it states.
- `testing/quick` is frozen but perfectly serviceable; it does not shrink
  failing inputs, so generators that produce small values make failures readable.
- For arbitrary bytes and automatic input discovery, use native fuzzing
  (see the `fuzzing` example).

## Running

```sh
go test -v ./testing-examples/quick-properties/
```
