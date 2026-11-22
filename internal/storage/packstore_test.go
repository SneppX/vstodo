package storage

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"vstodo/internal/object"
	"vstodo/internal/packfile"
)

func TestPackStorePersistsAndLoadsPackedObjects(t *testing.T) {
	root := t.TempDir()
	loose := NewLooseStore(filepath.Join(root, "objects"))
	packsDir := filepath.Join(root, "objects", "pack")
	want := &object.Object{Type: object.BlobType, Data: []byte("durable packed object")}
	writer := packfile.NewPackWriter(want)
	packBytes, _, err := writer.WritePack()
	if err != nil {
		t.Fatal(err)
	}
	store := NewPackStore(loose, packsDir)
	count, err := store.StorePack(packBytes)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("stored object count = %d, want 1", count)
	}
	id := object.ComputeID(want.Type, want.Data)
	if _, err := os.Stat(loose.objectPath(id)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("packed object unexpectedly expanded into loose storage: %v", err)
	}
	ok, err := store.Has(id)
	if err != nil || !ok {
		t.Fatalf("Has(%s) = %v, %v", id.Hex(), ok, err)
	}
	looseIDs, err := store.Objects()
	if err != nil {
		t.Fatal(err)
	}
	if len(looseIDs) != 0 {
		t.Fatalf("loose object count = %d, want 0", len(looseIDs))
	}
	allIDs, err := store.AllObjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(allIDs) != 1 || allIDs[0] != id {
		t.Fatalf("AllObjects() = %v, want [%s]", allIDs, id.Hex())
	}

	// A new store instance proves that reads are backed by durable files,
	// rather than only by the first instance's in-memory cache.
	reopened := NewPackStore(NewLooseStore(filepath.Join(root, "objects")), packsDir)
	got, err := reopened.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != want.Type || !bytes.Equal(got.Data, want.Data) || got.ID != id {
		t.Fatalf("loaded object = %#v, want type %q and data %q", got, want.Type, want.Data)
	}
}

func TestPackStoreRejectsInvalidPackWithoutPublishingFiles(t *testing.T) {
	root := t.TempDir()
	packsDir := filepath.Join(root, "objects", "pack")
	store := NewPackStore(NewLooseStore(filepath.Join(root, "objects")), packsDir)
	if _, err := store.StorePack([]byte("not a pack")); err == nil {
		t.Fatal("StorePack accepted invalid input")
	}
	if _, err := os.Stat(packsDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("invalid pack created pack directory: %v", err)
	}
}

func TestPackStoreHasReturnsFalseForMissingObject(t *testing.T) {
	root := t.TempDir()
	store := NewPackStore(NewLooseStore(filepath.Join(root, "objects")), filepath.Join(root, "objects", "pack"))
	ok, err := store.Has(object.ComputeID(object.BlobType, []byte("missing")))
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("Has returned true for missing object")
	}
}

func TestPackStoreDoesNotRetainPacksAboveCacheBudget(t *testing.T) {
	root := t.TempDir()
	packsDir := filepath.Join(root, "objects", "pack")
	want := &object.Object{Type: object.BlobType, Data: []byte("larger than cache budget")}
	writer := packfile.NewPackWriter(want)
	packBytes, _, err := writer.WritePack()
	if err != nil {
		t.Fatal(err)
	}
	store := NewPackStore(NewLooseStore(filepath.Join(root, "objects")), packsDir)
	store.maxCacheBytes = 1
	if _, err := store.StorePack(packBytes); err != nil {
		t.Fatal(err)
	}
	id := object.ComputeID(want.Type, want.Data)
	if _, err := store.Load(id); err != nil {
		t.Fatal(err)
	}
	if len(store.cache) != 0 {
		t.Fatalf("decoded pack cache retained %d entries above its byte budget", len(store.cache))
	}
}
