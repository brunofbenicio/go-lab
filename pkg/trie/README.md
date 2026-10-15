# trie

Generic prefix tree (`Trie[V]`) keyed by strings. It indexes by rune, so
accented and multi-byte text works, and it answers prefix questions such as
autocomplete efficiently.

## Usage

```go
t := trie.New[int]()
t.Put("golang", 1)
t.Put("gopher", 2)
t.KeysWithPrefix("go") // ["golang" "gopher"], sorted
t.HasPrefix("py")      // false
t.Delete("gopher")     // prunes branches that no longer lead to a key
```

Not safe for concurrent use.

## Tests

```sh
go test -race ./pkg/trie/
```
