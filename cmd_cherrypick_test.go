package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCherryPickCombinesNonConflictingChanges(t *testing.T) {
	a, root, _, pickID := makeMergeFixture(t,
		map[string]string{"left.txt": "base left\n", "right.txt": "base right\n"},
		map[string]string{"left.txt": "ours left\n", "right.txt": "base right\n"},
		map[string]string{"left.txt": "base left\n", "right.txt": "picked right\n"},
	)
	if err := cmdCherryPick(a, []string{pickID.Hex()}); err != nil {
		t.Fatalf("cherry-pick: %v", err)
	}
	for path, want := range map[string]string{"left.txt": "ours left\n", "right.txt": "picked right\n"} {
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil || string(data) != want {
			t.Fatalf("%s = %q, err=%v; want %q", path, data, err, want)
		}
	}
	head, err := a.Refs.Head()
	if err != nil {
		t.Fatalf("read HEAD: %v", err)
	}
	obj, err := a.Store.Load(head)
	if err != nil {
		t.Fatalf("load cherry-pick commit: %v", err)
	}
	parsed, err := commit.Parse(head, obj.Data)
	if err != nil {
		t.Fatalf("parse cherry-pick commit: %v", err)
	}
	if len(parsed.Parents) != 1 {
		t.Fatalf("cherry-pick parents = %v, want one parent", parsed.Parents)
	}
	if parsed.Message != "Cherry-pick: snapshot" {
		t.Fatalf("cherry-pick message = %q", parsed.Message)
	}
}

func TestCherryPickConflictLeavesRepositoryUnchanged(t *testing.T) {
	a, root, oursID, pickID := makeMergeFixture(t,
		map[string]string{"file.txt": "base\nsecond\n"},
		map[string]string{"file.txt": "ours\nsecond\n"},
		map[string]string{"file.txt": "picked\nsecond\n"},
	)
	if err := cmdCherryPick(a, []string{pickID.Hex()}); err == nil {
		t.Fatal("cherry-pick unexpectedly accepted conflicting changes")
	}
	head, err := a.Refs.Head()
	if err != nil || head != oursID {
		t.Fatalf("HEAD changed: %s, err=%v", head.Hex(), err)
	}
	data, err := os.ReadFile(filepath.Join(root, "file.txt"))
	if err != nil || string(data) != "ours\nsecond\n" {
		t.Fatalf("worktree changed: data=%q err=%v", data, err)
	}
}
