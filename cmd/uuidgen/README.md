# uuidgen

Prints random (version 4) UUIDs using the operating system's secure random
number source.

## Usage

```sh
uuidgen [-n count] [-upper] [-no-dashes]
```

| Flag         | Meaning                                  |
|--------------|------------------------------------------|
| `-n N`       | how many UUIDs to print, 1-1000 (default `1`) |
| `-upper`     | upper-case hex digits                    |
| `-no-dashes` | omit the dashes                          |

Exit status: `0` on success, `1` if random bytes could not be read, `2` for
usage errors. Nothing is printed when an error occurs.

## Examples

```sh
uuidgen
# 3f2b8c1e-6d4a-4b7e-9a15-0c8e2d7f41ab

uuidgen -n 3 -upper
uuidgen -no-dashes            # 32 hex digits, handy for file names
```

## Tests

The tests replace the random source with a fixed byte sequence to check the
exact output, and also check real output against the version 4 format.

```sh
go test -race ./cmd/uuidgen/
```
