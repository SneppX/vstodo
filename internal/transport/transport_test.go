package transport

import (
	"os"
	"path/filepath"
	"testing"

	"vstodo/internal/app"
	"vstodo/internal/object"
)

func makeRepo(t *testing.T, root string) *app.App {
	t.Helper()
	a, err := app.Init(root)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	blobID, err := a.Store.WriteRaw(object.BlobType, []byte("hello\n"))
	if err != nil {
		t.Fatalf("store blob: %v", err)
	}
	treeData := append([]byte("100644 file.txt\x00"), blobID.Bytes()...)
	treeID, err := a.Store.WriteRaw(object.TreeType, treeData)
	if err != nil {
		t.Fatalf("store tree: %v", err)
	}
	commitData := []byte("tree " + treeID.Hex() + "\n\nmessage\n")
	commitID, err := a.Store.WriteRaw(object.CommitType, commitData)
	if err != nil {
		t.Fatalf("store commit: %v", err)
	}
	if err := a.Refs.WriteRef("refs/heads/main", commitID); err != nil {
		t.Fatalf("write ref: %v", err)
	}
	if err := a.Refs.WriteRef("refs/tags/v1.0", commitID); err != nil {
		t.Fatalf("write tag: %v", err)
	}
	if err := a.Refs.SetHead("ref: refs/heads/main"); err != nil {
		t.Fatalf("set head: %v", err)
	}
	return a
}

func TestLocalTransportClone(t *testing.T) {
	srcRoot := filepath.Join(t.TempDir(), "src")
	dstRoot := filepath.Join(t.TempDir(), "dst")
	src := makeRepo(t, srcRoot)
	dstApp, err := app.Init(dstRoot)
	if err != nil {
		t.Fatalf("init dst: %v", err)
	}
	if err := NewLocalTransport().Clone(src, dstApp); err != nil {
		t.Fatalf("clone: %v", err)
	}
	head, err := os.ReadFile(filepath.Join(dstApp.Repo.GitDir, "HEAD"))
	if err != nil {
		t.Fatalf("read dst HEAD: %v", err)
	}
	if string(head) != "ref: refs/heads/main\n" {
		t.Fatalf("bad dst HEAD: %q", string(head))
	}
	refs, err := dstApp.Refs.ListRefs()
	if err != nil {
		t.Fatalf("list refs: %v", err)
	}
	if _, ok := refs["refs/heads/main"]; !ok {
		t.Fatal("missing heads/main")
	}
	if _, ok := refs["refs/tags/v1.0"]; !ok {
		t.Fatal("missing tags/v1.0")
	}
	data, err := os.ReadFile(filepath.Join(dstApp.Repo.Root, "file.txt"))
	if err != nil {
		t.Fatalf("read dst file: %v", err)
	}
	if string(data) != "hello\n" {
		t.Fatalf("bad file content: %q", string(data))
	}
	ids, _ := src.Store.Objects()
	var found bool
	for _, id := range ids {
		has, _ := dstApp.Store.Has(id)
		if has {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("no objects copied")
	}
}

func writeTreeFixture(t *testing.T, a *app.App, mode, name string, childData []byte, childType object.Type) error {
	t.Helper()
	childID, err := a.Store.WriteRaw(childType, childData)
	if err != nil {
		t.Fatalf("write child object: %v", err)
	}
	entry := append([]byte(mode+" "+name+"\x00"), childID.Bytes()...)
	treeID, err := a.Store.WriteRaw(object.TreeType, entry)
	if err != nil {
		t.Fatalf("write tree: %v", err)
	}
	commit := []byte("tree " + treeID.Hex() + "\n\nfixture\n")
	return writeTreeToWorktree(a, commit)
}

func TestWriteTreeAcceptsGitDirectoryMode(t *testing.T) {
	a, err := app.Init(filepath.Join(t.TempDir(), "repo"))
	if err != nil {
		t.Fatal(err)
	}
	blobID, err := a.Store.WriteRaw(object.BlobType, []byte("nested\n"))
	if err != nil {
		t.Fatal(err)
	}
	innerEntry := append([]byte("100644 file.txt\x00"), blobID.Bytes()...)
	if err := writeTreeFixture(t, a, "40000", "docs", innerEntry, object.TreeType); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(a.Repo.Root, "docs", "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "nested\n" {
		t.Fatalf("content = %q", data)
	}
}

func TestWriteTreeRejectsTraversal(t *testing.T) {
	a, err := app.Init(filepath.Join(t.TempDir(), "repo"))
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(filepath.Dir(a.Repo.Root), "escape.txt")
	err = writeTreeFixture(t, a, "100644", "../escape.txt", []byte("owned"), object.BlobType)
	if err == nil {
		t.Fatal("expected traversal path to be rejected")
	}
	if _, statErr := os.Stat(outside); !os.IsNotExist(statErr) {
		t.Fatalf("outside file exists or stat failed unexpectedly: %v", statErr)
	}
}

func TestWriteTreeRejectsRepositoryMetadata(t *testing.T) {
	a, err := app.Init(filepath.Join(t.TempDir(), "repo"))
	if err != nil {
		t.Fatal(err)
	}
	metadataEntry := append([]byte("100644 config\x00"), []byte("not a real blob")...)
	err = writeTreeFixture(t, a, "40000", ".vstodo", metadataEntry, object.TreeType)
	if err == nil {
		t.Fatal("expected repository metadata path to be rejected")
	}
	if _, statErr := os.Stat(filepath.Join(a.Repo.GitDir, "config")); statErr != nil {
		t.Fatalf("repository config was changed or removed: %v", statErr)
	}
}

func TestWriteTreeRejectsSymlinkParent(t *testing.T) {
	a, err := app.Init(filepath.Join(t.TempDir(), "repo"))
	if err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(a.Repo.Root, "redirect")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	blobID, err := a.Store.WriteRaw(object.BlobType, []byte("outside"))
	if err != nil {
		t.Fatal(err)
	}
	inner := append([]byte("100644 file.txt\x00"), blobID.Bytes()...)
	err = writeTreeFixture(t, a, "40000", "redirect", inner, object.TreeType)
	if err == nil {
		t.Fatal("expected symlink parent to be rejected")
	}
	if _, statErr := os.Stat(filepath.Join(outside, "file.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("outside file exists or stat failed unexpectedly: %v", statErr)
	}
}
