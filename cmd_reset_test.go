package main

import (
	"os"
	"path/filepath"
	"testing"

	"vstodo/internal/sha1"
)

func TestResetSoftMovesHeadWithoutChangingIndexOrWorktree(t *testing.T) {
	a, root, targetID, _ := checkoutFixture(t)
	mainID, err := a.Refs.ReadRef("refs/heads/main")
	if err != nil {
		t.Fatalf("read main ref: %v", err)
	}
	before := make(map[string]sha1.Digest, len(a.Index.Entries()))
	for path, id := range a.Index.Entries() {
		before[path] = id
	}

	if err := cmdReset(a, []string{"--soft", "feature"}); err != nil {
		t.Fatalf("reset --soft: %v", err)
	}
	head, err := a.Refs.Head()
	if err != nil || head != targetID {
		t.Fatalf("HEAD = %s, err=%v; want %s", head.Hex(), err, targetID.Hex())
	}
	if branch, err := a.Refs.CurrentBranch(); err != nil || branch != "main" {
		t.Fatalf("branch = %q, err=%v; want main", branch, err)
	}
	if len(a.Index.Entries()) != len(before) {
		t.Fatalf("soft reset changed index size")
	}
	for path, id := range before {
		if a.Index.Get(path) != id {
			t.Fatalf("soft reset changed index entry %s", path)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, "keep.txt"))
	if err != nil || string(data) != "original\n" {
		t.Fatalf("soft reset changed worktree: data=%q err=%v", data, err)
	}
	if mainID == targetID {
		t.Fatal("fixture commits unexpectedly match")
	}
}

func TestResetMixedMovesHeadAndIndexButLeavesWorktree(t *testing.T) {
	a, root, _, target := checkoutFixture(t)
	if err := cmdCheckout(a, []string{"feature"}); err != nil {
		t.Fatalf("checkout feature: %v", err)
	}
	mainID, err := a.Refs.ReadRef("refs/heads/main")
	if err != nil {
		t.Fatalf("read main ref: %v", err)
	}

	if err := cmdReset(a, []string{"--mixed", "main"}); err != nil {
		t.Fatalf("reset --mixed: %v", err)
	}
	head, err := a.Refs.Head()
	if err != nil || head != mainID {
		t.Fatalf("HEAD = %s, err=%v; want %s", head.Hex(), err, mainID.Hex())
	}
	for path, content := range target {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil || string(data) != content {
			t.Fatalf("mixed reset changed worktree path %s: data=%q err=%v", path, data, err)
		}
	}
	if a.Index.Contains("new.txt") {
		t.Fatal("mixed reset retained a staged file absent from target")
	}
	if !a.Index.Contains("remove.txt") {
		t.Fatal("mixed reset did not restore target index snapshot")
	}
}

func TestResetHardRestoresTrackedSnapshot(t *testing.T) {
	a, root, _, _ := checkoutFixture(t)
	if err := cmdCheckout(a, []string{"feature"}); err != nil {
		t.Fatalf("checkout feature: %v", err)
	}
	mainID, err := a.Refs.ReadRef("refs/heads/main")
	if err != nil {
		t.Fatalf("read main ref: %v", err)
	}

	if err := cmdReset(a, []string{"--hard", "main"}); err != nil {
		t.Fatalf("reset --hard: %v", err)
	}
	head, err := a.Refs.Head()
	if err != nil || head != mainID {
		t.Fatalf("HEAD = %s, err=%v; want %s", head.Hex(), err, mainID.Hex())
	}
	for path, content := range map[string]string{
		"keep.txt":       "original\n",
		"remove.txt":     "remove me\n",
		"nested/old.txt": "old nested file\n",
	} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil || string(data) != content {
			t.Fatalf("hard reset did not restore %s: data=%q err=%v", path, data, err)
		}
	}
	for _, path := range []string{"new.txt", "nested"} {
		if path == "nested" {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, path)); err == nil {
			t.Fatalf("%s should have been removed", path)
		}
	}
	if a.Index.Contains("new.txt") || !a.Index.Contains("nested/old.txt") {
		t.Fatal("hard reset index does not match target snapshot")
	}
}

func TestResetHardRefusesToOverwriteUntrackedFiles(t *testing.T) {
	a, root, _, _ := checkoutFixture(t)
	if err := os.WriteFile(filepath.Join(root, "new.txt"), []byte("untracked data\n"), 0o644); err != nil {
		t.Fatalf("write untracked file: %v", err)
	}
	if err := cmdReset(a, []string{"--hard", "feature"}); err == nil {
		t.Fatal("reset --hard unexpectedly overwrote an untracked file")
	}
	if branch, err := a.Refs.CurrentBranch(); err != nil || branch != "main" {
		t.Fatalf("branch = %q, err=%v; want main", branch, err)
	}
	data, err := os.ReadFile(filepath.Join(root, "new.txt"))
	if err != nil || string(data) != "untracked data\n" {
		t.Fatalf("untracked file changed: data=%q err=%v", data, err)
	}
}
