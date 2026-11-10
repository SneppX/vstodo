package benchmark_test

import (
	"testing"

	"vstodo/internal/object"
	"vstodo/internal/storage"
)

func benchStoreDir(b *testing.B) string {
	return b.TempDir() + "/objects"
}

// BenchmarkStoreStore measures writing objects to the loose store.
func BenchmarkStoreStore(b *testing.B) {
	dir := benchStoreDir(b)
	store := storage.NewLooseStore(dir)
	obj := &object.Object{Type: object.BlobType, Data: make([]byte, 1024)}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.Store(obj); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkStoreLoad measures reading objects from the loose store.
func BenchmarkStoreLoad(b *testing.B) {
	dir := benchStoreDir(b)
	store := storage.NewLooseStore(dir)
	obj := &object.Object{Type: object.BlobType, Data: make([]byte, 1024)}
	id, err := store.Store(obj)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.Load(id); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkStoreHas measures the Has membership check.
func BenchmarkStoreHas(b *testing.B) {
	dir := benchStoreDir(b)
	store := storage.NewLooseStore(dir)
	obj := &object.Object{Type: object.BlobType, Data: make([]byte, 1024)}
	id, err := store.Store(obj)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.Has(id); err != nil {
			b.Fatal(err)
		}
	}
}
