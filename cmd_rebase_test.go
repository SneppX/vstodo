package main

import (
	"os"
	"path/filepath"
	"testing"

	"vstodo/internal/commit"
)

func TestRebaseReplaysLinearCommitOntoUpstream(t *testing.T) {
	a, root, oursID, upstreamID := makeMergeFixture(t,
		map[string]string{"left.txt": "base left\n", "right.txt": "base right\n"},
		map[string]string{"left.txt": "ours left\n", "right.txt": "base right\n"},
		map[string]string{"left.txt": "base left\n", "right.txt": "upstream right\n"},
	)
	if err := cmdRebase(a, []string{"feature"}); err != nil {
		t.Fatalf("rebase: %v", err)
	}
	head, err := a.Refs.Head()
	if err != nil || head == oursID || head == upstreamID {
		t.Fatalf("rebased HEAD = %s, err=%v; expected rewritten commit", head.Hex(), err)
	}
	obj, err := a.Store.Load(head)
	if err != nil {
		t.Fatalf("load rebased commit: %v", err)
	}
	parsed, err := commit.Parse(head, obj.Data)
	if err != nil {
		t.Fatalf("parse rebased commit: %v", err)
	}
	if len(parsed.Parents) != 1 || parsed.Parents[0] != upstreamID {
		t.Fatalf("rebased parents = %v, want [%s]", parsed.Parents, upstreamID.Hex())
	}
	for path, want := range map[string]string{"left.txt": "ours left\n", "right.txt": "upstream right\n"} {
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil || string(data) != want {
			t.Fatalf("%s = %q, err=%v; want %q", path, data, err, want)
		}
	}
}

func TestRebaseConflictLeavesRepositoryUnchanged(t *testing.T) {
	a, root, oursID, _ := makeMergeFixture(t,
		map[string]string{"file.txt": "base\nsecond\n"},
		map[string]string{"file.txt": "ours\nsecond\n"},
		map[string]string{"file.txt": "upstream\nsecond\n"},
	)
	if err := cmdRebase(a, []string{"feature"}); err == nil {
		t.Fatal("rebase unexpectedly accepted overlapping changes")
	}
	head, err := a.Refs.Head()
	if err != nil || head != oursID {
		t.Fatalf("HEAD changed after conflict: %s, err=%v", head.Hex(), err)
	}
	data, err := os.ReadFile(filepath.Join(root, "file.txt"))
	if err != nil || string(data) != "ours\nsecond\n" {
		t.Fatalf("worktree changed after conflict: data=%q err=%v", data, err)
	}
}
