# csv2json

Converts CSV into a JSON array of objects, using the header row for the keys.
Keys keep the column order of the file.

## Usage

```sh
csv2json [-d delim] [-no-header] [-types] [-compact] [file]
```

| Flag         | Meaning                                                          |
|--------------|------------------------------------------------------------------|
| `-d delim`   | field delimiter: one character, or `\t` for a tab (default `,`)  |
| `-no-header` | the first row is data; columns are named `col1`, `col2`, ...     |
| `-types`     | write numbers, `true`/`false` and empty cells as JSON numbers, booleans and `null` |
| `-compact`   | print everything on one line                                     |

By default every value is a string. With `-types`, only text that is already a
valid JSON number is converted, so values such as `007` or `+5` stay strings and
numbers keep their exact digits. With no file, or with `-`, standard input is
read.

Exit status: `0` on success, `1` for unreadable or malformed CSV (duplicate or
empty column names, rows with the wrong number of fields), `2` for usage errors.

## Examples

```sh
printf 'name,age\nAda,36\n' | csv2json -types -compact
# [{"name":"Ada","age":36}]

csv2json -d ';' export.csv > export.json
csv2json -no-header -d '\t' data.tsv
```

## Tests

```sh
go test -race ./cmd/csv2json/
```
