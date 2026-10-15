# validator

Validates structs using rules declared in `validate` tags, collecting every
violation instead of stopping at the first one.

## Usage

```go
type SignUp struct {
    Name  string `validate:"required,min=3,max=40"`
    Email string `validate:"required,email"`
    Age   int    `validate:"min=18"`
    Plan  string `validate:"oneof=free pro team"`
}

if err := validator.Validate(req); err != nil {
    var errs validator.Errors
    errors.As(err, &errs) // each FieldError has Field, Rule and Param
}
```

Rules: `required`, `min=N`, `max=N` (rune count, collection length or numeric
value), `oneof=a b c` and `email`. An empty field that is not `required` skips
its other rules. Nested structs are validated recursively, and an unknown rule
panics, since that is a bug in the tags.

## Tests

```sh
go test -race ./pkg/validator/
```
