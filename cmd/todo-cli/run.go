package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
)

const usage = `usage: todo-cli [-f file] <command> [arguments]

commands:
  add <title...>                 add a task
  list [-status all|pending|done] show tasks (default: all)
  done <id>                      mark a task as completed
  rm <id>                        remove a task
`

// app bundles what every command needs.
type app struct {
	path           string
	stdout, stderr io.Writer
}

// run is the testable entry point. It returns the process exit status:
// 0 on success, 1 for runtime failures (I/O, unknown task), 2 for usage errors.
func run(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("todo-cli", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(stderr, usage)
		fs.PrintDefaults()
	}
	path := fs.String("f", "todos.json", "path of the JSON file that stores the tasks")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() == 0 {
		fs.Usage()
		return 2
	}

	a := &app{path: *path, stdout: stdout, stderr: stderr}
	cmd, rest := fs.Arg(0), fs.Args()[1:]
	switch cmd {
	case "add":
		return a.add(rest)
	case "list", "ls":
		return a.list(rest)
	case "done":
		return a.done(rest)
	case "rm":
		return a.remove(rest)
	default:
		fmt.Fprintf(stderr, "todo-cli: unknown command %q\n", cmd)
		fs.Usage()
		return 2
	}
}

func (a *app) fail(code int, format string, args ...any) int {
	fmt.Fprintf(a.stderr, "todo-cli: "+format+"\n", args...)
	return code
}

// parseID reads the single <id> argument of done and rm.
func (a *app) parseID(cmd string, args []string) (int, bool) {
	if len(args) != 1 {
		a.fail(2, "%s needs exactly one task id", cmd)
		return 0, false
	}
	id, err := strconv.Atoi(args[0])
	if err != nil || id < 1 {
		a.fail(2, "invalid task id %q", args[0])
		return 0, false
	}
	return id, true
}

func (a *app) add(args []string) int {
	title := strings.TrimSpace(strings.Join(args, " "))
	if title == "" {
		return a.fail(2, "add needs a title")
	}
	tasks, err := load(a.path)
	if err != nil {
		return a.fail(1, "%v", err)
	}
	task := Task{ID: nextID(tasks), Title: title}
	if err := save(a.path, append(tasks, task)); err != nil {
		return a.fail(1, "%v", err)
	}
	fmt.Fprintf(a.stdout, "added #%d: %s\n", task.ID, task.Title)
	return 0
}

func (a *app) list(args []string) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(a.stderr)
	status := fs.String("status", "all", "which tasks to show: all, pending or done")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if !slices.Contains([]string{"all", "pending", "done"}, *status) {
		return a.fail(2, "invalid -status %q (use all, pending or done)", *status)
	}
	tasks, err := load(a.path)
	if err != nil {
		return a.fail(1, "%v", err)
	}
	shown := 0
	for _, t := range tasks {
		if (*status == "pending" && t.Done) || (*status == "done" && !t.Done) {
			continue
		}
		mark := " "
		if t.Done {
			mark = "x"
		}
		fmt.Fprintf(a.stdout, "[%s] %d %s\n", mark, t.ID, t.Title)
		shown++
	}
	if shown == 0 {
		fmt.Fprintln(a.stdout, "no tasks")
	}
	return 0
}

func (a *app) done(args []string) int {
	id, ok := a.parseID("done", args)
	if !ok {
		return 2
	}
	tasks, err := load(a.path)
	if err != nil {
		return a.fail(1, "%v", err)
	}
	i := indexOf(tasks, id)
	if i < 0 {
		return a.fail(1, "task %d not found", id)
	}
	tasks[i].Done = true
	if err := save(a.path, tasks); err != nil {
		return a.fail(1, "%v", err)
	}
	fmt.Fprintf(a.stdout, "completed #%d: %s\n", id, tasks[i].Title)
	return 0
}

func (a *app) remove(args []string) int {
	id, ok := a.parseID("rm", args)
	if !ok {
		return 2
	}
	tasks, err := load(a.path)
	if err != nil {
		return a.fail(1, "%v", err)
	}
	i := indexOf(tasks, id)
	if i < 0 {
		return a.fail(1, "task %d not found", id)
	}
	title := tasks[i].Title
	if err := save(a.path, slices.Delete(tasks, i, i+1)); err != nil {
		return a.fail(1, "%v", err)
	}
	fmt.Fprintf(a.stdout, "removed #%d: %s\n", id, title)
	return 0
}
