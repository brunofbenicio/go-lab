# 003 - Anagram groups

## Problem

Given a list of words, group the ones that are anagrams of each other.

- `Key(word string) string` returns a canonical signature: letters and digits,
  lower-cased and sorted. Spaces and punctuation are ignored.
- `AreAnagrams(a, b string) bool` compares two words (identical words do not
  count).
- `Group(words []string) [][]string` returns the groups in order of first
  appearance, keeping the original spelling and order inside each group.

## Examples

```go
anagram.Group([]string{"eat", "tea", "tan", "ate", "nat", "bat"})
// [["eat" "tea" "ate"] ["tan" "nat"] ["bat"]]
```

## Concepts

Using a canonical key as a map index, rune handling for Unicode text, and
deterministic output order (no map iteration in the result).

## Running the tests

```sh
go test -race ./exercises/003-anagram-groups/
```
