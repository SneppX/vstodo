package main

import (
	"os"
	"path/filepath"
	"testing"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/sha1"
)

func makeMergeFixture(t *testing.T, baseFiles, oursFiles, theirsFiles map[string]string) (*app.App, string, sha1.Digest, sha1.Digest) {
	t.Helper()
	root := t.TempDir()
	a, err := app.Init(root)
	if err != nil {
		t.Fatalf("app.Init: %v", err)
	}
	baseEntries := storeCheckoutSnapshot(t, a, baseFiles, nil)
	baseID := storeCheckoutCommit(t, a, baseEntries, nil)
	oursEntries := storeCheckoutSnapshot(t, a, oursFiles, nil)
	oursID := storeCheckoutCommit(t, a, oursEntries, []sha1.Digest{baseID})
	theirsEntries := storeCheckoutSnapshot(t, a, theirsFiles, nil)
	theirsID := storeCheckoutCommit(t, a, theirsEntries, []sha1.Digest{baseID})

	for path, content := range oursFiles {
		writeCheckoutTestFile(t, a, path, content)
	}
	if err := a.Refs.WriteRef("refs/heads/main", oursID); err != nil {
		t.Fatalf("write main: %v", err)
	}
	if err := a.Refs.WriteRef("refs/heads/feature", theirsID); err != nil {
		t.Fatalf("write feature: %v", err)
	}
	if err := a.Refs.SetHead("ref: refs/heads/main"); err != nil {
		t.Fatalf("set HEAD: %v", err)
	}
	for path, id := range oursEntries {
		a.Index.Add(path, id)
	}
	if err := a.SaveIndex(); err != nil {
		t.Fatalf("save index: %v", err)
	}
	return a, root, oursID, theirsID
}

func TestMergeCombinesChangesAcrossFiles(t *testing.T) {
	a, root, oursID, theirsID := makeMergeFixture(t,
		map[string]string{"left.txt": "base left\n", "right.txt": "base right\n"},
		map[string]string{"left.txt": "ours left\n", "right.txt": "base right\n"},
		map[string]string{"left.txt": "base left\n", "right.txt": "theirs right\n"},
	)
	if err := cmdMerge(a, []string{"feature"}); err != nil {
		t.Fatalf("merge: %v", err)
	}
	for path, want := range map[string]string{"left.txt": "ours left\n", "right.txt": "theirs right\n"} {
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil || string(data) != want {
			t.Fatalf("%s = %q, err=%v; want %q", path, data, err, want)
		}
	}
	head, err := a.Refs.Head()
	if err != nil || head == oursID || head == theirsID {
		t.Fatalf("merge HEAD = %s, err=%v; expected a new merge commit", head.Hex(), err)
	}
	obj, err := a.Store.Load(head)
	if err != nil {
		t.Fatalf("load merge commit: %v", err)
	}
	parsed, err := commit.Parse(head, obj.Data)
	if err != nil {
		t.Fatalf("parse merge commit: %v", err)
	}
	if len(parsed.Parents) != 2 || parsed.Parents[0] != oursID || parsed.Parents[1] != theirsID {
		t.Fatalf("merge parents = %v, want [%s %s]", parsed.Parents, oursID.Hex(), theirsID.Hex())
	}
}

func TestMergeCombinesNonOverlappingEditsInOneFile(t *testing.T) {
	a, root, _, _ := makeMergeFixture(t,
		map[string]string{"file.txt": "one\ntwo\nthree\n"},
		map[string]string{"file.txt": "ONE\ntwo\nthree\n"},
		map[string]string{"file.txt": "one\ntwo\nTHREE\n"},
	)
	if err := cmdMerge(a, []string{"feature"}); err != nil {
		t.Fatalf("merge: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "file.txt"))
	if err != nil {
		t.Fatalf("read merged file: %v", err)
	}
	if string(data) != "ONE\ntwo\nTHREE\n" {
		t.Fatalf("merged content = %q", data)
	}
}

func TestMergeConflictLeavesRepositoryUnchanged(t *testing.T) {
	a, root, oursID, _ := makeMergeFixture(t,
		map[string]string{"file.txt": "base\nsecond\n"},
		map[string]string{"file.txt": "ours\nsecond\n"},
		map[string]string{"file.txt": "theirs\nsecond\n"},
	)
	if err := cmdMerge(a, []string{"feature"}); err == nil {
		t.Fatal("merge unexpectedly accepted overlapping edits")
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
