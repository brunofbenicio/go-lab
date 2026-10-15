# todo-cli

A tiny task manager that keeps its list in a JSON file.

## Usage

```sh
todo-cli [-f file] <command> [arguments]
```

| Command                          | What it does                          |
|----------------------------------|---------------------------------------|
| `add <title...>`                 | add a task (the words are joined)     |
| `list [-status all\|pending\|done]` | show tasks (`ls` is an alias)      |
| `done <id>`                      | mark a task as completed              |
| `rm <id>`                        | remove a task                         |

The `-f` flag selects the storage file (default `todos.json` in the current
directory). A missing or empty file is treated as an empty list. The file is
replaced atomically on every change and looks like this:

```json
[
  {
    "id": 1,
    "title": "buy milk",
    "done": false
  }
]
```

Exit status: `0` on success, `1` for runtime failures (unreadable file, unknown
task id), `2` for usage errors.

## Examples

```sh
todo-cli add write the report        # added #1: write the report
todo-cli add -review notes           # everything after "add" is the title
todo-cli done 1                      # completed #1: write the report
todo-cli list -status pending
todo-cli -f ~/work.json list
```

## Tests

The tests keep every task file inside `t.TempDir()`.

```sh
go test -race ./cmd/todo-cli/
```
