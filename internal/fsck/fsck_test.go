package fsck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
	"vstodo/internal/tree"
)

func fixture(t *testing.T) *app.App {
	t.Helper()
	a, err := app.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	blobID, err := a.Store.WriteRaw(object.BlobType, []byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	treeData := (&tree.Tree{Entries: []tree.Entry{{Mode: "100644", Name: "hello.txt", ID: blobID}}}).Marshal()
	treeID, err := a.Store.WriteRaw(object.TreeType, treeData)
	if err != nil {
		t.Fatal(err)
	}
	commitData := commit.Build(treeID, nil, "VSTodo Test", "VSTodo Test", "initial")
	commitID, err := a.Store.WriteRaw(object.CommitType, commitData)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Refs.WriteRef("refs/heads/main", commitID); err != nil {
		t.Fatal(err)
	}
	if err := a.Refs.SetHead("ref: refs/heads/main"); err != nil {
		t.Fatal(err)
	}
	return a
}

func TestCheckValidRepository(t *testing.T) {
	a := fixture(t)
	report, err := Check(a)
	if err != nil {
		t.Fatal(err)
	}
	if report.Objects != 3 || report.Blobs != 1 || report.Trees != 1 || report.Commits != 1 || report.References != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestCheckAcceptsUnbornRepository(t *testing.T) {
	a, err := app.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	report, err := Check(a)
	if err != nil {
		t.Fatal(err)
	}
	if report.Objects != 0 || report.References != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestCheckRejectsCorruptObject(t *testing.T) {
	a := fixture(t)
	ids, err := a.Store.Objects()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(a.Store.ObjectsDir, ids[0].Hex()[:2], ids[0].Hex()[2:])
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not a zlib stream"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Check(a); err == nil || !strings.Contains(err.Error(), "object") {
		t.Fatalf("expected corrupt object error, got %v", err)
	}
}

func TestCheckRejectsMissingTreeEntry(t *testing.T) {
	a, err := app.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	missing, err := sha1.FromHex(strings.Repeat("f", 40))
	if err != nil {
		t.Fatal(err)
	}
	treeData := (&tree.Tree{Entries: []tree.Entry{{Mode: "100644", Name: "missing.txt", ID: missing}}}).Marshal()
	treeID, err := a.Store.WriteRaw(object.TreeType, treeData)
	if err != nil {
		t.Fatal(err)
	}
	commitData := commit.Build(treeID, nil, "VSTodo Test", "VSTodo Test", "broken")
	commitID, err := a.Store.WriteRaw(object.CommitType, commitData)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Refs.WriteRef("refs/heads/main", commitID); err != nil {
		t.Fatal(err)
	}
	if err := a.Refs.SetHead("ref: refs/heads/main"); err != nil {
		t.Fatal(err)
	}
	if _, err := Check(a); err == nil || !strings.Contains(err.Error(), "missing object") {
		t.Fatalf("expected missing tree entry error, got %v", err)
	}
}

func TestCheckValidatesRemoteTrackingReferences(t *testing.T) {
	a := fixture(t)
	branchID, err := a.Refs.ReadRef("refs/heads/main")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Refs.WriteRef("refs/remotes/origin/main", branchID); err != nil {
		t.Fatal(err)
	}
	report, err := Check(a)
	if err != nil {
		t.Fatal(err)
	}
	if report.References != 2 {
		t.Fatalf("reference count = %d, want 2", report.References)
	}
}

func TestCheckRejectsMissingRemoteTrackingTarget(t *testing.T) {
	a := fixture(t)
	missing := sha1.Digest{9, 8, 7}
	if err := a.Refs.WriteRef("refs/remotes/origin/broken", missing); err != nil {
		t.Fatal(err)
	}
	if _, err := Check(a); err == nil || !strings.Contains(err.Error(), "missing object") {
		t.Fatalf("expected missing remote-tracking target error, got %v", err)
	}
}
