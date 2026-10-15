package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Task is one entry of the to-do list.
type Task struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

// load reads the task list from path. A missing or empty file is an empty
// list, so the first "add" can create it.
func load(path string) ([]Task, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}
	var tasks []Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return tasks, nil
}

// save writes the task list to path. It writes a temporary file in the same
// directory and renames it, so a crash never leaves a half-written list.
func save(path string, tasks []Task) error {
	if tasks == nil {
		tasks = []Task{} // encode as [] rather than null
	}
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(path), ".todo-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op once the rename succeeded
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// nextID returns an ID one greater than the largest in use.
func nextID(tasks []Task) int {
	maxID := 0
	for _, t := range tasks {
		maxID = max(maxID, t.ID)
	}
	return maxID + 1
}

// indexOf returns the position of the task with the given ID, or -1.
func indexOf(tasks []Task, id int) int {
	for i, t := range tasks {
		if t.ID == id {
			return i
		}
	}
	return -1
}
