# jsonfmt

Pretty-prints or minifies JSON. Object keys keep their original order, and
invalid JSON is reported with a non-zero exit status, so it also works as a
quick validator.

## Usage

```sh
jsonfmt [-m] [-indent N] [file]
```

| Flag        | Meaning                                              |
|-------------|------------------------------------------------------|
| `-m`        | minify instead of pretty-printing                    |
| `-indent N` | spaces per indentation level, 0-16 (default `2`)     |

With no file, or with `-`, standard input is read.

Exit status: `0` on success, `1` for unreadable or invalid JSON, `2` for usage
errors.

## Examples

```sh
echo '{"b":1,"a":[1,2]}' | jsonfmt
# {
#   "b": 1,
#   "a": [
#     1,
#     2
#   ]
# }

jsonfmt -m config.json            # one compact line
jsonfmt -indent 4 config.json     # four-space indentation
echo '{oops}' | jsonfmt           # jsonfmt: invalid JSON: ... (exit 1)
```

## Tests

```sh
go test -race ./cmd/jsonfmt/
```
