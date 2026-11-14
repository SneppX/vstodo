package main

import "testing"

func TestBranchCreatesFromStartPoint(t *testing.T) {
	a, _, _, targetID, _ := makeMergeFixture(t,
		map[string]string{"base.txt": "base\n"},
		map[string]string{"ours.txt": "ours\n"},
		map[string]string{"theirs.txt": "theirs\n"},
	)
	if err := cmdBranch(a, []string{"release", "feature"}); err != nil {
		t.Fatalf("create branch: %v", err)
	}
	id, err := a.Refs.ReadRef("refs/heads/release")
	if err != nil || id != targetID {
		t.Fatalf("release ref = %s, err=%v; want %s", id.Hex(), err, targetID.Hex())
	}
}

func TestBranchSafeDeleteAndForceDelete(t *testing.T) {
	a, _, _, _, _ := makeMergeFixture(t,
		map[string]string{"base.txt": "base\n"},
		map[string]string{"ours.txt": "ours\n"},
		map[string]string{"theirs.txt": "theirs\n"},
	)
	if err := cmdBranch(a, []string{"-d", "feature"}); err == nil {
		t.Fatal("safe delete unexpectedly removed an unmerged branch")
	}
	if err := cmdBranch(a, []string{"-D", "feature"}); err != nil {
		t.Fatalf("force delete: %v", err)
	}
	if _, err := a.Refs.ReadRef("refs/heads/feature"); err == nil {
		t.Fatal("feature ref still exists")
	}
}

func TestBranchRenameUpdatesCurrentHead(t *testing.T) {
	a, _, _, mainID, _ := makeMergeFixture(t,
		map[string]string{"base.txt": "base\n"},
		map[string]string{"ours.txt": "ours\n"},
		map[string]string{"theirs.txt": "theirs\n"},
	)
	if err := cmdBranch(a, []string{"-m", "main", "trunk"}); err != nil {
		t.Fatalf("rename branch: %v", err)
	}
	if current, err := a.Refs.CurrentBranch(); err != nil || current != "trunk" {
		t.Fatalf("current branch = %q, err=%v; want trunk", current, err)
	}
	id, err := a.Refs.ReadRef("refs/heads/trunk")
	if err != nil || id != mainID {
		t.Fatalf("trunk ref = %s, err=%v; want %s", id.Hex(), err, mainID.Hex())
	}
	if _, err := a.Refs.ReadRef("refs/heads/main"); err == nil {
		t.Fatal("old main ref still exists")
	}
}
