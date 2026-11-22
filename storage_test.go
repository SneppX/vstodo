package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sync"
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
		t.Fatal("Store returned zero digest")
	}

	ok, err := store.Has(id)
	if err != nil || !ok {
		t.Fatalf("Has = %v, %v; want true, nil", ok, err)
	}
	loaded, err := store.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Type != obj.Type || !bytes.Equal(loaded.Data, obj.Data) || loaded.ID != id {
		t.Fatalf("loaded object = %+v, want type %s and original data", loaded, obj.Type)
	}

	ok, err = store.Has(sha1.Zero)
	if err != nil || ok {
		t.Fatalf("Has(zero) = %v, %v; want false, nil", ok, err)
	}
	raw, err := store.ReadObject(id)
	if err != nil || !bytes.Equal(raw, obj.Data) {
		t.Fatalf("ReadObject = %q, %v", raw, err)
	}

	id2, err := store.WriteRaw(object.BlobType, []byte("raw payload"))
	if err != nil {
		t.Fatalf("WriteRaw: %v", err)
	}
	if ok, err := store.Has(id2); err != nil || !ok {
		t.Fatalf("Has(WriteRaw) = %v, %v", ok, err)
	}
}

func TestStoragePathLayout(t *testing.T) {
	store := storage.NewLooseStore(filepath.Join(t.TempDir(), "objects"))
	id, err := store.WriteRaw(object.BlobType, []byte("path layout"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(store.ObjectsDir, id.Hex()[:2], id.Hex()[2:])
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("object not stored at expected path %s: %v", path, err)
	}
}

func TestStorageConcurrentSameObject(t *testing.T) {
	store := storage.NewLooseStore(filepath.Join(t.TempDir(), "objects"))
	obj := &object.Object{Type: object.BlobType, Data: []byte("same object from concurrent writers")}
	const writers = 32

	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.Store(obj)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("concurrent Store: %v", err)
		}
	}

	id := object.HashBlob(obj.Data)
	loaded, err := store.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(loaded.Data, obj.Data) {
		t.Fatalf("loaded data = %q", loaded.Data)
	}
	ids, err := store.Objects()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != id {
		t.Fatalf("Objects = %v, want only %s", ids, id.Hex())
	}
}

func TestStoreRawRejectsMismatchedIDWithoutChangingObject(t *testing.T) {
	store := storage.NewLooseStore(filepath.Join(t.TempDir(), "objects"))
	original := []byte("original immutable payload")
	id, err := store.WriteRaw(object.BlobType, original)
	if err != nil {
		t.Fatal(err)
	}
	tampered, err := object.Compress(object.BlobType, []byte("tampered payload"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.StoreRaw(id, tampered); err == nil {
		t.Fatal("StoreRaw accepted bytes with a mismatched object ID")
	}
	loaded, err := store.Load(id)
	if err != nil {
		t.Fatalf("original object was damaged: %v", err)
	}
	if !bytes.Equal(loaded.Data, original) {
		t.Fatalf("object changed to %q", loaded.Data)
	}
}

func TestStoreRawRejectsCorruptCompression(t *testing.T) {
	store := storage.NewLooseStore(filepath.Join(t.TempDir(), "objects"))
	id := object.HashBlob([]byte("anything"))
	if err := store.StoreRaw(id, []byte("not zlib")); err == nil {
		t.Fatal("StoreRaw accepted corrupt compression")
	}
}

func TestStorageObjectsReturnsWalkErrors(t *testing.T) {
	store := storage.NewLooseStore(filepath.Join(t.TempDir(), "missing"))
	_, err := store.Objects()
	if err == nil {
		t.Fatal("Objects should report a missing object directory")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Objects error = %v, want os.ErrNotExist", err)
	}
}

func TestStorageStoreIsIdempotent(t *testing.T) {
	store := storage.NewLooseStore(filepath.Join(t.TempDir(), "objects"))
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
