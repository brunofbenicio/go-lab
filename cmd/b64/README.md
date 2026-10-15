# b64

Encodes and decodes Base64 from a file or standard input.

## Usage

```sh
b64 [-d] [-url] [-raw] [file]
```

| Flag   | Meaning                                                        |
|--------|----------------------------------------------------------------|
| `-d`   | decode instead of encode                                       |
| `-url` | URL-safe alphabet (`-` and `_` instead of `+` and `/`)         |
| `-raw` | no `=` padding (omitted when encoding, not expected when decoding) |

Encoded output ends with a newline. When decoding, spaces and line breaks in
the input are ignored and the decoded bytes are written as they are. With no
file, or with `-`, standard input is read.

Exit status: `0` on success, `1` for unreadable or invalid input, `2` for usage
errors.

## Examples

```sh
printf 'hello' | b64                  # aGVsbG8=
printf 'aGVsbG8=' | b64 -d            # hello
printf '\xfb\xff' | b64 -url -raw     # -_8
b64 -d encoded.txt > original.bin
```

## Tests

```sh
go test -race ./cmd/b64/
```
