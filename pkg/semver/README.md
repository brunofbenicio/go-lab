# semver

Parses, formats and compares semantic versions following semver.org 2.0.0:
`MAJOR.MINOR.PATCH` with optional pre-release and build metadata. A leading
`v` is accepted. Parsing is strict (no leading zeros, no empty identifiers) and
comparison follows the precedence rules of section 11, including pre-releases.

## Usage

```go
v, err := semver.Parse("v1.4.0-rc.1+build.5")
v.Compare(semver.MustParse("1.4.0")) // -1: a pre-release sorts before its release
v.IncPatch()                         // 1.4.1

slices.SortFunc(versions, semver.Version.Compare)
```

Build metadata is ignored when comparing.

## Tests

```sh
go test -race ./pkg/semver/
```
