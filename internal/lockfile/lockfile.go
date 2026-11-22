// Package lockfile provides exclusive, crash-safe file replacement using
// Git-style sibling .lock files.
package lockfile

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var ErrLocked = errors.New("lock file already exists")

type Lock struct {
	path      string
	lockPath  string
	file      *os.File
	committed bool
}

func Acquire(path string, perm os.FileMode) (*Lock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("lockfile: create parent directory: %w", err)
	}
	lockPath := path + ".lock"
	file, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("%w: %s", ErrLocked, lockPath)
		}
		return nil, fmt.Errorf("lockfile: acquire %s: %w", lockPath, err)
	}
	return &Lock{path: path, lockPath: lockPath, file: file}, nil
}

func (l *Lock) Commit(data []byte) error {
	if l == nil || l.file == nil || l.committed {
		return errors.New("lockfile: lock is not active")
	}
	n, err := l.file.Write(data)
	if err != nil {
		return fmt.Errorf("lockfile: write: %w", err)
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	if err := l.file.Sync(); err != nil {
		return fmt.Errorf("lockfile: sync: %w", err)
	}
	if err := l.file.Close(); err != nil {
		l.file = nil
		return fmt.Errorf("lockfile: close: %w", err)
	}
	l.file = nil
	if err := os.Rename(l.lockPath, l.path); err != nil {
		return fmt.Errorf("lockfile: publish %s: %w", l.path, err)
	}
	l.committed = true
	return nil
}

func (l *Lock) Close() error {
	if l == nil {
		return nil
	}
	var closeErr error
	if l.file != nil {
		closeErr = l.file.Close()
		l.file = nil
	}
	if !l.committed {
		if err := os.Remove(l.lockPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return errors.Join(closeErr, fmt.Errorf("lockfile: remove %s: %w", l.lockPath, err))
		}
	}
	return closeErr
}
