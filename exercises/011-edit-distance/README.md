# 011 - Edit distance

## Problem

The Levenshtein distance is the minimum number of insertions, deletions and
substitutions needed to turn one string into another.

- `Distance(a, b string) int` computes it (rune-based, using two rows of memory).
- `Similarity(a, b string) float64` maps it to a score from 0 to 1.
- `Suggest(word string, candidates []string, maxDist int) []string` returns the
  close candidates, closest first, for "did you mean ...?" messages.

## Examples

```go
levenshtein.Distance("kitten", "sitting")                   // 3
levenshtein.Suggest("comit", []string{"commit", "merge"}, 2) // ["commit"]
```

## Concepts

Dynamic programming with rolling rows, runes versus bytes, and stable sorting
with `slices.SortFunc`.

## Running the tests

```sh
go test -race ./exercises/011-edit-distance/
```
