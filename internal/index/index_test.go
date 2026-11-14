package index

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"vstodo/internal/sha1"
)

func TestIndexSavesGitDIRCVersion2(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index")
	idx := NewIndex(path)
	id := sha1.Sum([]byte("object id"))
	idx.Add("src/main.go", id)
	idx.AddMode("bin/tool", id, 0100755)
	if err := idx.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data[:4]) != "DIRC" {
		t.Fatalf("signature = %q, want DIRC", data[:4])
	}
	if got := data[7]; got != 2 {
		t.Fatalf("version low byte = %d, want 2", got)
	}
	loaded := NewIndex(path)
	if err := loaded.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Get("src/main.go") != id {
		t.Fatal("blob ID did not round-trip")
	}
	if loaded.Mode("bin/tool") != 0100755 {
		t.Fatalf("executable mode = %o, want 100755", loaded.Mode("bin/tool"))
	}
}

func TestIndexLoadsLegacyTextAndMigratesOnSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index")
	id := sha1.Sum([]byte("legacy"))
	legacy := []byte(id.Hex() + "\tlegacy.txt\n")
	if err := os.WriteFile(path, legacy, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	idx := NewIndex(path)
	if err := idx.Load(); err != nil {
		t.Fatalf("Load legacy: %v", err)
	}
	if idx.Get("legacy.txt") != id {
		t.Fatal("legacy entry was not loaded")
	}
	if err := idx.Save(); err != nil {
		t.Fatalf("Save migrated index: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile migrated index: %v", err)
	}
	if !bytes.HasPrefix(data, []byte("DIRC")) {
		t.Fatal("legacy index was not migrated to DIRC")
	}
}

func TestIndexRejectsChecksumCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index")
	idx := NewIndex(path)
	idx.Add("file.txt", sha1.Sum([]byte("file")))
	if err := idx.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	data[12] ^= 0xff
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("corrupt index: %v", err)
	}
	if err := NewIndex(path).Load(); err == nil {
		t.Fatal("Load accepted corrupted checksum")
	}
}
