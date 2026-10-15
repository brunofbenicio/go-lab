// Package rotate provides a size-based rotating file writer, in the spirit of
// log rotation: when the current file would exceed a limit it becomes
// name.1, the previous name.1 becomes name.2, and so on.
package rotate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Writer appends to dir/name and rotates it when it grows past maxSize.
// It is safe for concurrent use.
type Writer struct {
	mu      sync.Mutex
	path    string
	maxSize int64
	keep    int // number of rotated backups to retain
	file    *os.File
	size    int64
}

// New opens (or creates) dir/name for appending. keep is how many backups
// (name.1 ... name.keep) are retained; zero discards the old data on rotation.
func New(dir, name string, maxSize int64, keep int) (*Writer, error) {
	w := &Writer{path: filepath.Join(dir, name), maxSize: maxSize, keep: keep}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *Writer) open() error {
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	w.file, w.size = f, info.Size()
	return nil
}

// Write appends p, rotating first if p would push the file over the limit.
// A single write is never split across two files.
func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return 0, os.ErrClosed
	}
	if w.size > 0 && w.size+int64(len(p)) > w.maxSize {
		if err := w.rotate(); err != nil {
			return 0, fmt.Errorf("rotate: %w", err)
		}
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *Writer) rotate() error {
	if err := w.file.Close(); err != nil {
		return err
	}
	if w.keep == 0 {
		if err := os.Remove(w.path); err != nil {
			return err
		}
		return w.open()
	}
	// Shift name.(i) to name.(i+1), dropping the oldest backup.
	for i := w.keep; i >= 1; i-- {
		from := w.path
		if i > 1 {
			from = fmt.Sprintf("%s.%d", w.path, i-1)
		}
		to := fmt.Sprintf("%s.%d", w.path, i)
		if err := os.Rename(from, to); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return w.open()
}

// Close closes the current file. Calling it more than once is safe.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}
