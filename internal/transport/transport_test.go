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
	// write a blob and a commit manually
	blobID, err := a.Store.WriteRaw(object.BlobType, []byte("hello\n"))
	if err != nil {
		t.Fatalf("store blob: %v", err)
	}
	treeData := object.Marshal(object.TreeType, []byte("100644 file.txt\x00"))
	treeData = append(treeData, blobID.Bytes()...)
	treeID, err := a.Store.WriteRaw(object.TreeType, treeData)
	if err != nil {
		t.Fatalf("store tree: %v", err)
	}
	commitData := object.Marshal(object.CommitType, []byte("tree "+treeID.Hex()+"\n\nmessage\n"))
	commitID, err := a.Store.WriteRaw(object.CommitType, commitData)
	if err != nil {
		t.Fatalf("store commit: %v", err)
	}
	if err := a.Refs.WriteRef("refs/heads/main", commitID); err != nil { // main -> commit
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

	tp := NewLocalTransport()
	if err := tp.Clone(src, dstApp); err != nil {
		t.Fatalf("clone: %v", err)
	}

	// HEAD copied
	head, err := os.ReadFile(filepath.Join(dstApp.Repo.GitDir, "HEAD"))
	if err != nil {
		t.Fatalf("read dst HEAD: %v", err)
	}
	if string(head) != "ref: refs/heads/main\n" {
		t.Fatalf("bad dst HEAD: %q", string(head))
	}

	// refs copied
	refs, err := dstApp.Refs.ListRefs()
	if err != nil {
		t.Fatalf("list refs: %v", err)
	}
	if _, ok := refs["refs/heads/main"]; !ok {
		t.Fatalf("missing heads/main")
	}
	if _, ok := refs["refs/tags/v1.0"]; !ok {
		t.Fatalf("missing tags/v1.0")
	}

	// working tree checked out
	data, err := os.ReadFile(filepath.Join(dstApp.Repo.Root, "file.txt"))
	if err != nil {
		t.Fatalf("read dst file: %v", err)
	}
	if string(data) != "hello\n" {
		t.Fatalf("bad file content: %q", string(data))
	}

	// object copied
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
		t.Fatalf("no objects copied")
	}
}
