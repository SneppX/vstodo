package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"vstodo/internal/index"
	"vstodo/internal/ref"
	"vstodo/internal/sha1"
)

func TestIndexRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index")
	idx := index.NewIndex(path)

	idx.Add("main.go", sha1.Digest{1})
	idx.Add("util.go", sha1.Digest{2})
	idx.Add("sub/dir/file.txt", sha1.Digest{3})
	if err := idx.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if idx.Get("main.go") != (sha1.Digest{1}) {
		t.Fatalf("Get(main.go) mismatch")
	}
	if idx.Get("util.go") != (sha1.Digest{2}) {
		t.Fatalf("Get(util.go) mismatch")
	}
	if idx.Get("missing") != sha1.Zero {
		t.Fatalf("Get(missing) should be Zero")
	}
	if !idx.Contains("sub/dir/file.txt") {
		t.Fatalf("Contains should be true")
	}

	// Reload from disk
	idx2 := index.NewIndex(path)
	if err := idx2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if idx2.Get("main.go") != (sha1.Digest{1}) {
		t.Fatalf("reloaded Get(main.go) mismatch")
	}
	if idx2.Get("sub/dir/file.txt") != (sha1.Digest{3}) {
		t.Fatalf("reloaded Get(sub/dir/file.txt) mismatch")
	}
	if !idx2.Contains("util.go") {
		t.Fatalf("reloaded Contains(util.go) should be true")
	}

	// Remove
	idx.Remove("util.go")
	if idx.Contains("util.go") {
		t.Fatalf("Contains(util.go) should be false after Remove")
	}
	if err := idx.Save(); err != nil {
		t.Fatalf("Save after remove: %v", err)
	}
	idx3 := index.NewIndex(path)
	if err := idx3.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if idx3.Contains("util.go") {
		t.Fatalf("reloaded Contains(util.go) should be false after Remove+Save")
	}
}

func TestIndexLoadMissingFile(t *testing.T) {
	idx := index.NewIndex(filepath.Join(t.TempDir(), "nonexistent.index"))
	if err := idx.Load(); err != nil {
		t.Fatalf("Load of missing file should not error: %v", err)
	}
	if idx.Contains("anything") {
		t.Fatalf("empty index should not contain anything")
	}
}

func TestRefReadWrite(t *testing.T) {
	dir := t.TempDir()
	store := ref.NewStore(dir)

	d := sha1.Digest{0xab, 0xcd}
	if err := store.WriteRef("refs/heads/main", d); err != nil {
		t.Fatalf("WriteRef: %v", err)
	}
	got, err := store.ReadRef("refs/heads/main")
	if err != nil {
		t.Fatalf("ReadRef: %v", err)
	}
	if got != d {
		t.Fatalf("ReadRef = %s, want %s", got.Hex(), d.Hex())
	}
}

func TestRefHeadSetHead(t *testing.T) {
	dir := t.TempDir()
	store := ref.NewStore(dir)

	if err := store.SetHead("ref: refs/heads/main"); err != nil {
		t.Fatalf("SetHead symbolic: %v", err)
	}
	branch, err := store.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch != "main" {
		t.Fatalf("CurrentBranch = %q, want %q", branch, "main")
	}

	d := sha1.Digest{1}
	if err := store.WriteRef("HEAD", d); err != nil {
		t.Fatalf("WriteRef HEAD: %v", err)
	}
	head, err := store.Head()
	if err != nil {
		t.Fatalf("Head: %v", err)
	}
	if head != d {
		t.Fatalf("Head = %s, want %s", head.Hex(), d.Hex())
	}
}

func TestRefListRefs(t *testing.T) {
	dir := t.TempDir()
	store := ref.NewStore(dir)
	d1 := sha1.Digest{1}
	d2 := sha1.Digest{2}
	if err := store.WriteRef("refs/heads/feature", d1); err != nil {
		t.Fatalf("WriteRef feature: %v", err)
	}
	if err := store.WriteRef("refs/tags/v1.0", d2); err != nil {
		t.Fatalf("WriteRef tag: %v", err)
	}
	refs, err := store.ListRefs()
	if err != nil {
		t.Fatalf("ListRefs: %v", err)
	}
	if refs["refs/heads/feature"] != d1 {
		t.Fatalf("ListRefs missing feature")
	}
	if refs["refs/tags/v1.0"] != d2 {
		t.Fatalf("ListRefs missing tag v1.0")
	}
}

func TestRefCurrentBranchDetached(t *testing.T) {
	dir := t.TempDir()
	store := ref.NewStore(dir)
	d := sha1.Digest{5}
	if err := store.WriteRef("HEAD", d); err != nil {
		t.Fatalf("WriteRef HEAD: %v", err)
	}
	branch, err := store.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if branch != "" {
		t.Fatalf("detached HEAD should return empty branch, got %q", branch)
	}
}

func TestRefErrNotFound(t *testing.T) {
	dir := t.TempDir()
	store := ref.NewStore(dir)
	if _, err := store.ReadRef("refs/heads/nonexistent"); !errors.Is(err, ref.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRefLockPreventsConcurrentWriter(t *testing.T) {
	dir := t.TempDir()
	store := ref.NewStore(dir)
	first := sha1.Digest{1}
	second := sha1.Digest{2}
	if err := store.WriteRef("refs/heads/main", first); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(dir, "refs", "heads", "main.lock")
	if err := os.WriteFile(lockPath, []byte("held"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteRef("refs/heads/main", second); !errors.Is(err, ref.ErrLockExists) {
		t.Fatalf("WriteRef error = %v, want ErrLockExists", err)
	}
	got, err := store.ReadRef("refs/heads/main")
	if err != nil {
		t.Fatal(err)
	}
	if got != first {
		t.Fatalf("ref changed while lock was held: got %s, want %s", got.Hex(), first.Hex())
	}
}

func TestRefRejectsPathTraversal(t *testing.T) {
	store := ref.NewStore(t.TempDir())
	if err := store.WriteRef("../outside", sha1.Digest{1}); err == nil {
		t.Fatal("WriteRef accepted path traversal")
	}
	if _, err := store.ReadRef("../../outside"); err == nil {
		t.Fatal("ReadRef accepted path traversal")
	}
	if err := store.WriteRefSymbolic("HEAD", "../outside"); err == nil {
		t.Fatal("WriteRefSymbolic accepted path traversal")
	}
}

func TestRefDetectsSymbolicCycles(t *testing.T) {
	store := ref.NewStore(t.TempDir())
	if err := store.WriteRefSymbolic("HEAD", "refs/heads/main"); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteRefSymbolic("refs/heads/main", "HEAD"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadRef("HEAD"); err == nil {
		t.Fatal("ReadRef accepted a symbolic reference cycle")
	}
}

func TestDeleteRefHonorsLock(t *testing.T) {
	dir := t.TempDir()
	store := ref.NewStore(dir)
	id := sha1.Digest{9}
	if err := store.WriteRef("refs/heads/main", id); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(dir, "refs", "heads", "main.lock")
	if err := os.WriteFile(lockPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteRef("refs/heads/main"); !errors.Is(err, ref.ErrLockExists) {
		t.Fatalf("DeleteRef error = %v, want ErrLockExists", err)
	}
	got, err := store.ReadRef("refs/heads/main")
	if err != nil || got != id {
		t.Fatalf("ref after rejected deletion = %s, %v", got.Hex(), err)
	}
}

func TestDeleteRefReportsMissingReference(t *testing.T) {
	store := ref.NewStore(t.TempDir())
	if err := store.DeleteRef("refs/heads/missing"); !errors.Is(err, ref.ErrNotFound) {
		t.Fatalf("DeleteRef error = %v, want ErrNotFound", err)
	}
}

func TestRefUpdateUsesExpectedValue(t *testing.T) {
	store := ref.NewStore(t.TempDir())
	first := sha1.Digest{1}
	second := sha1.Digest{2}
	third := sha1.Digest{3}

	if err := store.UpdateRef("refs/heads/main", nil, first); err != nil {
		t.Fatalf("create with nil expectation: %v", err)
	}
	if err := store.UpdateRef("refs/heads/main", &first, second); err != nil {
		t.Fatalf("update matching value: %v", err)
	}
	if err := store.UpdateRef("refs/heads/main", &first, third); !errors.Is(err, ref.ErrReferenceChanged) {
		t.Fatalf("stale update error = %v, want ErrReferenceChanged", err)
	}
	got, err := store.ReadRef("refs/heads/main")
	if err != nil || got != second {
		t.Fatalf("ref after stale update = %s, %v; want %s", got.Hex(), err, second.Hex())
	}
}

func TestRefUpdateRejectsSymbolicReference(t *testing.T) {
	store := ref.NewStore(t.TempDir())
	if err := store.WriteRefSymbolic("HEAD", "refs/heads/main"); err != nil {
		t.Fatal(err)
	}
	expected := sha1.Digest{1}
	if err := store.UpdateRef("HEAD", &expected, sha1.Digest{2}); err == nil {
		t.Fatal("UpdateRef accepted a symbolic reference")
	}
}
