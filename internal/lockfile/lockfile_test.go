package lockfile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAcquireCommitReplacesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	lock, err := Acquire(path, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := lock.Commit([]byte("new state")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new state" {
		t.Fatalf("file content = %q, want new state", got)
	}
	if _, err := os.Stat(path + ".lock"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lock file remains after commit: %v", err)
	}
}

func TestAcquireIsExclusive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state")
	first, err := Acquire(path, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if _, err := Acquire(path, 0o644); !errors.Is(err, ErrLocked) {
		t.Fatalf("second Acquire error = %v, want ErrLocked", err)
	}
}

func TestCloseRemovesUncommittedLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state")
	lock, err := Acquire(path, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".lock"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("uncommitted lock file remains: %v", err)
	}
}
