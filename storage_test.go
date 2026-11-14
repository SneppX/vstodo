package main

import (
	"os"
	"path/filepath"
	"testing"

	"vstodo/internal/object"
	"vstodo/internal/sha1"
	"vstodo/internal/storage"
)

func TestStorageRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewLooseStore(filepath.Join(dir, "objects"))

	obj := &object.Object{Type: object.BlobType, Data: []byte("loose store test data")}
	id, err := store.Store(obj)
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if id == sha1.Zero {
		t.Fatalf("Store returned zero digest")
	}

	ok, err := store.Has(id)
	if err != nil {
		t.Fatalf("Has: %v", err)
	}
	if !ok {
		t.Fatalf("Has should be true after Store")
	}

	loaded, err := store.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Type != object.BlobType {
		t.Fatalf("loaded type = %s, want %s", loaded.Type, object.BlobType)
	}
	if string(loaded.Data) != "loose store test data" {
		t.Fatalf("loaded data = %q", string(loaded.Data))
	}
	if loaded.ID != id {
		t.Fatalf("loaded.ID = %s, want %s", loaded.ID.Hex(), id.Hex())
	}

	ok, err = store.Has(sha1.Zero)
	if err != nil {
		t.Fatalf("Has(zero): %v", err)
	}
	if ok {
		t.Fatalf("Has(zero) should be false")
	}

	// ReadObject / WriteRaw convenience
	raw, err := store.ReadObject(id)
	if err != nil {
		t.Fatalf("ReadObject: %v", err)
	}
	if string(raw) != "loose store test data" {
		t.Fatalf("ReadObject = %q", string(raw))
	}
	id2, err := store.WriteRaw(object.BlobType, []byte("raw payload"))
	if err != nil {
		t.Fatalf("WriteRaw: %v", err)
	}
	if ok, _ := store.Has(id2); !ok {
		t.Fatalf("WriteRaw object not stored")
	}
}

func TestStoragePathLayout(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewLooseStore(filepath.Join(dir, "objects"))
	obj := &object.Object{Type: object.BlobType, Data: []byte("path layout")}
	id, _ := store.Store(obj)
	path := filepath.Join(dir, "objects", id.Hex()[:2], id.Hex()[2:])
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("object not stored at expected path %s: %v", path, err)
	}
}

func TestStorageConcurrentIDs(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewLooseStore(filepath.Join(dir, "objects"))
	for i := 0; i < 50; i++ {
		data := []byte{byte(i)}
		obj := &object.Object{Type: object.BlobType, Data: data}
		if _, err := store.Store(obj); err != nil {
			t.Fatalf("Store(%d): %v", i, err)
		}
	}
	// Verify all retrievable
	for i := 0; i < 50; i++ {
		id := object.HashBlob([]byte{byte(i)})
		loaded, err := store.Load(id)
		if err != nil {
			t.Fatalf("Load(%d): %v", i, err)
		}
		if string(loaded.Data) != string([]byte{byte(i)}) {
			t.Fatalf("data mismatch for %d", i)
		}
	}
}

func TestStorageRejectsObjectWithMismatchedContentHash(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewLooseStore(filepath.Join(dir, "objects"))
	id, err := store.Store(&object.Object{Type: object.BlobType, Data: []byte("original")})
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	corrupt, err := object.Compress(object.BlobType, []byte("tampered"))
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}
	if err := store.StoreRaw(id, corrupt); err != nil {
		t.Fatalf("StoreRaw: %v", err)
	}
	if _, err := store.Load(id); err == nil {
		t.Fatal("Load succeeded for an object whose content does not match its object ID")
	}
}

func TestStorageStoreIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewLooseStore(filepath.Join(dir, "objects"))
	obj := &object.Object{Type: object.BlobType, Data: []byte("same immutable object")}
	first, err := store.Store(obj)
	if err != nil {
		t.Fatalf("first Store: %v", err)
	}
	second, err := store.Store(obj)
	if err != nil {
		t.Fatalf("second Store: %v", err)
	}
	if second != first {
		t.Fatalf("second Store ID = %s, want %s", second.Hex(), first.Hex())
	}
}
