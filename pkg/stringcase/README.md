# stringcase

Converts identifiers between `camelCase`, `PascalCase`, `snake_case`,
`kebab-case` and `SCREAMING_SNAKE_CASE`. Word detection understands separators,
case changes and acronyms, so `HTTPServer` is `HTTP` + `Server`, and works with
Unicode letters.

## Usage

```go
stringcase.Snake("HTTPServerConfig")  // "http_server_config"
stringcase.Kebab("userAccountID")     // "user-account-id"
stringcase.Camel("order_line_item")   // "orderLineItem"
stringcase.Words("parseHTTPResponse") // ["parse" "HTTP" "Response"]
```

Digits stay attached to the word before them (`user2Name` is `user2` +
`Name`).

## Tests

```sh
go test -race ./pkg/stringcase/
```
