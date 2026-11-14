package main

import (
	"os"
	"path/filepath"
	"testing"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/repo"
	"vstodo/internal/sha1"
	"vstodo/internal/storage"
)

func TestRepoInitFind(t *testing.T) {
	root := filepath.Join(t.TempDir(), "myrepo")
	r, err := repo.Init(root)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if r.Root != root {
		t.Fatalf("Repo.Root = %q, want %q", r.Root, root)
	}
	if r.GitDir != filepath.Join(root, ".vstodo") {
		t.Fatalf("Repo.GitDir = %q", r.GitDir)
	}
	if _, err := os.Stat(filepath.Join(r.GitDir, "HEAD")); err != nil {
		t.Fatalf("HEAD not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(r.GitDir, "objects")); err != nil {
		t.Fatalf("objects dir not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(r.GitDir, "refs/heads")); err != nil {
		t.Fatalf("refs/heads not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(r.GitDir, "config")); err != nil {
		t.Fatalf("config not created: %v", err)
	}

	found, err := repo.Find(root)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if found.Root != r.Root || found.GitDir != r.GitDir {
		t.Fatalf("Find returned different repo")
	}
}

func TestRepoPathGitPath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "r")
	r, _ := repo.Init(root)
	if r.Path("a", "b.txt") != filepath.Join(root, "a", "b.txt") {
		t.Fatalf("Path mismatch")
	}
	if r.GitPath("objects", "ab", "cd") != filepath.Join(r.GitDir, "objects", "ab", "cd") {
		t.Fatalf("GitPath mismatch")
	}
	if r.ObjectsDir() != filepath.Join(r.GitDir, "objects") {
		t.Fatalf("ObjectsDir mismatch")
	}
	if r.RefPath("heads", "main") != filepath.Join(r.GitDir, "refs", "heads", "main") {
		t.Fatalf("RefPath mismatch")
	}
	if r.HeadPath() != filepath.Join(r.GitDir, "HEAD") {
		t.Fatalf("HeadPath mismatch")
	}
}

func TestRepoNotFound(t *testing.T) {
	if _, err := repo.Find(filepath.Join(t.TempDir(), "notarepo")); err == nil {
		t.Fatalf("expected error for non-repo directory")
	}
}

func TestCommitParseAuthorCommitterTZ(t *testing.T) {
	data := []byte("tree 0000000000000000000000000000000000000000\nauthor Alice <alice@example.com> 1000000 +0000\ncommitter Bob <bob@example.com> 1000000 +0000\n\nhello\n")
	c, err := commit.Parse(sha1.Zero, data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if c.AuthorTS != 1000000 {
		t.Fatalf("AuthorTS = %d, want 1000000", c.AuthorTS)
	}
	if c.CommitterTS != 1000000 {
		t.Fatalf("CommitterTS = %d, want 1000000", c.CommitterTS)
	}
	if c.Message != "hello" {
		t.Fatalf("Message = %q, want %q", c.Message, "hello")
	}
}

func TestCommitParseMessageMultiline(t *testing.T) {
	data := []byte("tree 0000000000000000000000000000000000000000\nauthor A <a> 0 +0000\ncommitter B <b> 0 +0000\n\nfirst line\nsecond line\nthird line\n")
	c, err := commit.Parse(sha1.Zero, data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if c.Message != "first line\nsecond line\nthird line" {
		t.Fatalf("Message = %q", c.Message)
	}
}

func TestCommitIDMatchesStore(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewLooseStore(filepath.Join(dir, "objects"))
	cb := commit.Build(sha1.Zero, nil, "A <a>", "B <b>", "store test")
	id := object.HashCommit(cb)
	if _, err := store.Store(&object.Object{Type: object.CommitType, Data: cb}); err != nil {
		t.Fatalf("Store: %v", err)
	}
	loaded, err := store.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Type != object.CommitType {
		t.Fatalf("loaded type = %s", loaded.Type)
	}
	if !bytesEqual(loaded.Data, cb) {
		t.Fatalf("loaded data mismatch")
	}
}

func TestTreeToMapAndReadBlob(t *testing.T) {
	root := filepath.Join(t.TempDir(), "trepo")
	a, err := app.Init(root)
	if err != nil {
		t.Fatalf("app.Init: %v", err)
	}
	// write a blob directly
	blobID, err := a.Store.WriteRaw(object.BlobType, []byte("tree-to-map content"))
	if err != nil {
		t.Fatalf("WriteRaw: %v", err)
	}
	data, err := readBlob(a, blobID)
	if err != nil {
		t.Fatalf("readBlob: %v", err)
	}
	if string(data) != "tree-to-map content" {
		t.Fatalf("readBlob = %q", string(data))
	}
}

func TestBuildTreeFromIndex(t *testing.T) {
	root := filepath.Join(t.TempDir(), "btree")
	a, err := app.Init(root)
	if err != nil {
		t.Fatalf("app.Init: %v", err)
	}
	id1, err := a.Store.WriteRaw(object.BlobType, []byte("file one"))
	if err != nil {
		t.Fatalf("WriteRaw: %v", err)
	}
	id2, err := a.Store.WriteRaw(object.BlobType, []byte("file two"))
	if err != nil {
		t.Fatalf("WriteRaw: %v", err)
	}
	entries := map[string]sha1.Digest{
		"alpha.txt": id1,
		"beta.txt":  id2,
	}
	treeID, err := buildTreeFromIndex(a, entries, "")
	if err != nil {
		t.Fatalf("buildTreeFromIndex: %v", err)
	}
	if treeID == sha1.Zero {
		t.Fatalf("tree ID is zero")
	}
	m, err := treeToMap(a, treeID, "")
	if err != nil {
		t.Fatalf("treeToMap: %v", err)
	}
	if m["alpha.txt"] != id1 || m["beta.txt"] != id2 {
		t.Fatalf("treeToMap mismatch: %v", m)
	}
}

func TestWalkFiles(t *testing.T) {
	root := filepath.Join(t.TempDir(), "walkroot")
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".vstodo"), 0o755); err != nil {
		t.Fatalf("MkdirAll .vstodo: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".vstodo", "ignore.txt"), []byte("ignore"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	files, err := walkFiles(root)
	if err != nil {
		t.Fatalf("walkFiles: %v", err)
	}
	seen := map[string]bool{}
	for _, f := range files {
		seen[f] = true
	}
	if !seen["a.txt"] || !seen["sub/b.txt"] {
		t.Fatalf("walkFiles missing files: %v", files)
	}
	if seen["ignore.txt"] {
		t.Fatalf("walkFiles should skip .vstodo")
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
