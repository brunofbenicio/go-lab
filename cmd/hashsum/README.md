# hashsum

Prints the MD5, SHA-1 or SHA-256 digest of files or standard input, in the same
`<hex>  <name>` format used by `md5sum` and `sha256sum`.

## Usage

```sh
hashsum [-a md5|sha1|sha256] [file ...]
```

| Flag       | Meaning                                      |
|------------|----------------------------------------------|
| `-a name`  | algorithm: `md5`, `sha1` or `sha256` (default `sha256`) |

With no file, or with `-`, standard input is read and the name shown is `-`.
If a file cannot be read, an error is printed, the remaining files are still
processed, and the exit status is `1`. Usage errors exit with `2`.

MD5 and SHA-1 are only meant for checking legacy checksums; use SHA-256 for
anything that must resist tampering.

## Examples

```sh
printf abc | hashsum
# ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad  -

hashsum -a md5 release.tar.gz notes.txt
```

## Tests

```sh
go test -race ./cmd/hashsum/
```
