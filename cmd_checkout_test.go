package main

import (
	"os"
	"path/filepath"
	"testing"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

func checkoutFixture(t *testing.T) (*app.App, string, sha1.Digest, map[string]string) {
	t.Helper()
	root := t.TempDir()
	a, err := app.Init(root)
	if err != nil {
		t.Fatalf("app.Init: %v", err)
	}

	initial := map[string]string{
		"keep.txt":       "original\n",
		"remove.txt":     "remove me\n",
		"nested/old.txt": "old nested file\n",
	}
	initialEntries := storeCheckoutSnapshot(t, a, initial, nil)
	initialID := storeCheckoutCommit(t, a, initialEntries, nil)
	for path, content := range initial {
		writeCheckoutTestFile(t, a, path, content)
	}
	if err := a.Refs.WriteRef("refs/heads/main", initialID); err != nil {
		t.Fatalf("write main ref: %v", err)
	}
	if err := a.Refs.SetHead("ref: refs/heads/main"); err != nil {
		t.Fatalf("set HEAD: %v", err)
	}
	for path, id := range initialEntries {
		a.Index.Add(path, id)
	}
	if err := a.SaveIndex(); err != nil {
		t.Fatalf("save initial index: %v", err)
	}

	target := map[string]string{
		"keep.txt": "updated\n",
		"nested":   "directory replaced by a file\n",
		"new.txt":  "new file\n",
	}
	targetEntries := storeCheckoutSnapshot(t, a, target, []sha1.Digest{initialID})
	targetID := storeCheckoutCommit(t, a, targetEntries, []sha1.Digest{initialID})
	if err := a.Refs.WriteRef("refs/heads/feature", targetID); err != nil {
		t.Fatalf("write feature ref: %v", err)
	}
	return a, root, targetID, target
}

func storeCheckoutSnapshot(t *testing.T, a *app.App, files map[string]string, parents []sha1.Digest) map[string]sha1.Digest {
	t.Helper()
	entries := make(map[string]sha1.Digest, len(files))
	for path, content := range files {
		id, err := a.Store.WriteRaw(object.BlobType, []byte(content))
		if err != nil {
			t.Fatalf("store blob %s: %v", path, err)
		}
		entries[path] = id
	}
	return entries
}

func storeCheckoutCommit(t *testing.T, a *app.App, entries map[string]sha1.Digest, parents []sha1.Digest) sha1.Digest {
	t.Helper()
	treeID, err := buildTreeFromIndex(a, entries, "")
	if err != nil {
		t.Fatalf("build tree: %v", err)
	}
	raw := commit.Build(treeID, parents, "Checkout Test", "Checkout Test", "snapshot")
	id, err := a.Store.WriteRaw(object.CommitType, raw)
	if err != nil {
		t.Fatalf("store commit: %v", err)
	}
	return id
}

func writeCheckoutTestFile(t *testing.T, a *app.App, rel, content string) {
	t.Helper()
	full, err := worktreePath(a, rel)
	if err != nil {
		t.Fatalf("worktreePath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestCheckoutSwitchesSnapshotAndUpdatesIndex(t *testing.T) {
	a, root, targetID, target := checkoutFixture(t)
	if err := cmdCheckout(a, []string{"feature"}); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	for path, want := range target {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if string(data) != want {
			t.Fatalf("%s = %q, want %q", path, data, want)
		}
	}
	for _, removed := range []string{"remove.txt", "nested/old.txt"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(removed))); !os.IsNotExist(err) {
			t.Fatalf("%s should have been removed, stat error = %v", removed, err)
		}
	}
	if branch, err := a.Refs.CurrentBranch(); err != nil || branch != "feature" {
		t.Fatalf("current branch = %q, err=%v; want feature", branch, err)
	}
	head, err := a.Refs.Head()
	if err != nil || head != targetID {
		t.Fatalf("HEAD = %s, err=%v; want %s", head.Hex(), err, targetID.Hex())
	}
	if len(a.Index.Entries()) != len(target) {
		t.Fatalf("index has %d entries, want %d", len(a.Index.Entries()), len(target))
	}
	for path := range target {
		if !a.Index.Contains(path) {
			t.Fatalf("index is missing %s", path)
		}
	}
}

func TestCheckoutRefusesToOverwriteLocalChanges(t *testing.T) {
	a, root, _, _ := checkoutFixture(t)
	writeCheckoutTestFile(t, a, "keep.txt", "local edit\n")

	if err := cmdCheckout(a, []string{"feature"}); err == nil {
		t.Fatal("checkout unexpectedly overwrote a local modification")
	}
	if branch, err := a.Refs.CurrentBranch(); err != nil || branch != "main" {
		t.Fatalf("current branch = %q, err=%v; want main", branch, err)
	}
	data, err := os.ReadFile(filepath.Join(root, "keep.txt"))
	if err != nil {
		t.Fatalf("read locally modified file: %v", err)
	}
	if string(data) != "local edit\n" {
		t.Fatalf("local modification was lost: %q", data)
	}
}

func TestCheckoutRefusesUntrackedFiles(t *testing.T) {
	a, root, _, _ := checkoutFixture(t)
	if err := os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("keep me\n"), 0o644); err != nil {
		t.Fatalf("write untracked file: %v", err)
	}
	if err := cmdCheckout(a, []string{"feature"}); err == nil {
		t.Fatal("checkout unexpectedly proceeded with an untracked file")
	}
	if branch, err := a.Refs.CurrentBranch(); err != nil || branch != "main" {
		t.Fatalf("current branch = %q, err=%v; want main", branch, err)
	}
}
