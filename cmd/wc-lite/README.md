# wc-lite

A small version of `wc`: counts lines, words, characters and bytes.

## Usage

```sh
wc-lite [-l] [-w] [-m] [-c] [file ...]
```

| Flag | Meaning                 |
|------|-------------------------|
| `-l` | print the line count    |
| `-w` | print the word count    |
| `-m` | print the character count |
| `-c` | print the byte count    |

With no flags it prints lines, words and bytes. Columns always appear in the
order lines, words, characters, bytes. With no file (or the file `-`) it reads
standard input; with several files it adds a `total` line.

Exit status: `0` on success, `1` if a file could not be read, `2` for usage
errors.

## Examples

```sh
printf 'hello world\nfoo\n' | wc-lite        #  2  3 16 (columns share one width)
wc-lite -l notes.txt                          # 120 notes.txt
wc-lite -m -c accents.txt                     # characters and bytes differ for UTF-8
```

## Tests

```sh
go test -race ./cmd/wc-lite/
```
