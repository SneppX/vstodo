package gc

import (
	"os"
	"testing"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
	"vstodo/internal/tree"
)

func gcFixture(t *testing.T) (*app.App, sha1.Digest, sha1.Digest) {
	t.Helper()
	a, err := app.Init(t.TempDir())
	if err != nil {
		t.Fatalf("app.Init: %v", err)
	}
	blobID, err := a.Store.WriteRaw(object.BlobType, []byte("reachable content"))
	if err != nil {
		t.Fatalf("store blob: %v", err)
	}
	treeData := (&tree.Tree{Entries: []tree.Entry{{Mode: "100644", Name: "file.txt", ID: blobID}}}).Marshal()
	treeID, err := a.Store.WriteRaw(object.TreeType, treeData)
	if err != nil {
		t.Fatalf("store tree: %v", err)
	}
	commitData := commit.Build(treeID, nil, "GC Test", "GC Test", "root")
	commitID, err := a.Store.WriteRaw(object.CommitType, commitData)
	if err != nil {
		t.Fatalf("store commit: %v", err)
	}
	if err := a.Refs.WriteRef("refs/heads/main", commitID); err != nil {
		t.Fatalf("write ref: %v", err)
	}
	if err := a.Refs.SetHead("ref: refs/heads/main"); err != nil {
		t.Fatalf("set HEAD: %v", err)
	}
	danglingID, err := a.Store.WriteRaw(object.BlobType, []byte("unreachable content"))
	if err != nil {
		t.Fatalf("store dangling blob: %v", err)
	}
	return a, blobID, danglingID
}

func TestGCDryRunDoesNotDeleteObjects(t *testing.T) {
	a, reachableID, danglingID := gcFixture(t)
	if err := GCWithOptions(a, true); err != nil {
		t.Fatalf("GC dry run: %v", err)
	}
	for _, id := range []sha1.Digest{reachableID, danglingID} {
		has, err := a.Store.Has(id)
		if err != nil || !has {
			t.Fatalf("object %s missing after dry run; has=%v err=%v", id.Hex(), has, err)
		}
	}
}

func TestGCPrunesOnlyUnreachableLooseObjects(t *testing.T) {
	a, reachableID, danglingID := gcFixture(t)
	if err := GCWithOptions(a, false); err != nil {
		t.Fatalf("GC prune: %v", err)
	}
	for _, id := range []sha1.Digest{reachableID} {
		has, err := a.Store.Has(id)
		if err != nil || !has {
			t.Fatalf("reachable object %s missing; has=%v err=%v", id.Hex(), has, err)
		}
	}
	if has, err := a.Store.Has(danglingID); err != nil || has {
		t.Fatalf("dangling object remains; has=%v err=%v", has, err)
	}
}

func TestGCAbortsOnBrokenReferenceWithoutDeletingObjects(t *testing.T) {
	a, _, danglingID := gcFixture(t)
	missing := object.ComputeID(object.BlobType, []byte("missing"))
	if err := a.Refs.WriteRef("refs/heads/broken", missing); err != nil {
		t.Fatalf("write broken ref: %v", err)
	}
	if err := GCWithOptions(a, false); err == nil {
		t.Fatal("GC unexpectedly succeeded with a broken ref")
	}
	if has, err := a.Store.Has(danglingID); err != nil || !has {
		t.Fatalf("GC deleted objects despite mark failure; has=%v err=%v", has, err)
	}
	_ = os.ErrNotExist
}
