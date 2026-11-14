package main

import (
	"os"
	"path/filepath"
	"testing"

	"vstodo/internal/object"
)

func TestStashPushResetsWorktreeAndPopRestoresChanges(t *testing.T) {
	a, root, _, _ := checkoutFixture(t)
	if err := os.WriteFile(filepath.Join(root, "keep.txt"), []byte("local edit\n"), 0o644); err != nil {
		t.Fatalf("edit tracked file: %v", err)
	}
	if err := os.Remove(filepath.Join(root, "remove.txt")); err != nil {
		t.Fatalf("delete tracked file: %v", err)
	}
	if err := cmdStash(a, []string{"push", "-m", "work in progress"}); err != nil {
		t.Fatalf("stash push: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "keep.txt"))
	if err != nil || string(data) != "original\n" {
		t.Fatalf("push did not reset keep.txt: data=%q err=%v", data, err)
	}
	if _, err := os.Stat(filepath.Join(root, "remove.txt")); err != nil {
		t.Fatalf("push did not restore deleted tracked file: %v", err)
	}
	if err := cmdStash(a, []string{"pop"}); err != nil {
		t.Fatalf("stash pop: %v", err)
	}
	data, err = os.ReadFile(filepath.Join(root, "keep.txt"))
	if err != nil || string(data) != "local edit\n" {
		t.Fatalf("pop did not restore modification: data=%q err=%v", data, err)
	}
	if _, err := os.Stat(filepath.Join(root, "remove.txt")); err == nil {
		t.Fatal("pop did not restore tracked deletion")
	}
	if a.Index.Get("keep.txt") != object.HashBlob([]byte("original\n")) {
		t.Fatal("stash pop changed the original staged index state")
	}
}

func TestStashIncludeUntrackedAndRestore(t *testing.T) {
	a, root, _, _ := checkoutFixture(t)
	untracked := filepath.Join(root, "new-untracked.txt")
	if err := os.WriteFile(untracked, []byte("untracked\n"), 0o644); err != nil {
		t.Fatalf("create untracked: %v", err)
	}
	if err := cmdStash(a, []string{"push", "-u", "-m", "with untracked"}); err != nil {
		t.Fatalf("stash push -u: %v", err)
	}
	if _, err := os.Stat(untracked); !os.IsNotExist(err) {
		t.Fatalf("untracked file remains after push, err=%v", err)
	}
	if err := cmdStash(a, []string{"pop"}); err != nil {
		t.Fatalf("stash pop: %v", err)
	}
	data, err := os.ReadFile(untracked)
	if err != nil || string(data) != "untracked\n" {
		t.Fatalf("untracked file not restored: data=%q err=%v", data, err)
	}
}

func TestStashLogRetainsMultipleEntries(t *testing.T) {
	a, root, _, _ := checkoutFixture(t)
	if err := os.WriteFile(filepath.Join(root, "keep.txt"), []byte("first\n"), 0o644); err != nil {
		t.Fatalf("write first change: %v", err)
	}
	if err := cmdStash(a, []string{"push"}); err != nil {
		t.Fatalf("first stash: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "keep.txt"), []byte("second\n"), 0o644); err != nil {
		t.Fatalf("write second change: %v", err)
	}
	if err := cmdStash(a, []string{"push"}); err != nil {
		t.Fatalf("second stash: %v", err)
	}
	data, err := os.ReadFile(stashLogPath(a))
	if err != nil {
		t.Fatalf("read stash log: %v", err)
	}
	if len(stashLogEntries(data)) != 2 {
		t.Fatalf("stash log entries = %d, want 2", len(stashLogEntries(data)))
	}
}
