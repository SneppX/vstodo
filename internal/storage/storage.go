// Package storage persists objects content-addressed by SHA-1.
// Loose objects live at .vstodo/objects/<2-hex>/<38-hex>, zlib-compressed.
package storage

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

// LooseStore persists objects as immutable loose files.
type LooseStore struct {
	ObjectsDir string
}

// NewLooseStore opens (creating) the loose object store.
func NewLooseStore(objectsDir string) *LooseStore {
	return &LooseStore{ObjectsDir: objectsDir}
}

// Store writes obj as an immutable object. Publication uses a same-directory
// temporary file and an atomic hard link, so readers never observe partial data
// and concurrent writers cannot replace an existing object.
func (s *LooseStore) Store(obj *object.Object) (sha1.Digest, error) {
	if obj == nil {
		return sha1.Zero, fmt.Errorf("storage: nil object")
	}
	id := object.ComputeID(obj.Type, obj.Data)
	compressed, err := object.Compress(obj.Type, obj.Data)
	if err != nil {
		return id, err
	}
	if err := s.storeCompressed(id, compressed, obj.Type, obj.Data); err != nil {
		return id, err
	}
	return id, nil
}

// storeCompressed atomically publishes validated compressed object bytes.
func (s *LooseStore) storeCompressed(id sha1.Digest, compressed []byte, typ object.Type, data []byte) error {
	path := s.objectPath(id)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("storage: create object directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".vstodo-object-*")
	if err != nil {
		return fmt.Errorf("storage: create temporary object: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(compressed); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("storage: write temporary object: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("storage: sync temporary object: %w", err)
	}
	if err := tmp.Chmod(0o444); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("storage: set object permissions: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("storage: close temporary object: %w", err)
	}

	if err := os.Link(tmpPath, path); err == nil {
		return nil
	} else if !os.IsExist(err) {
		return fmt.Errorf("storage: atomically publish object %s: %w", id.Hex(), err)
	}

	existing, err := s.Load(id)
	if err != nil {
		return fmt.Errorf("storage: existing object %s is invalid: %w", id.Hex(), err)
	}
	if existing.Type != typ || !bytes.Equal(existing.Data, data) {
		return fmt.Errorf("storage: object ID collision for %s", id.Hex())
	}
	return nil
}

// Load reads, decompresses, and verifies the object for id.
func (s *LooseStore) Load(id sha1.Digest) (*object.Object, error) {
	compressed, err := os.ReadFile(s.objectPath(id))
	if err != nil {
		return nil, err
	}
	raw, err := object.Decompress(compressed)
	if err != nil {
		return nil, fmt.Errorf("storage: decompress object %s: %w", id.Hex(), err)
	}
	typ, data, err := object.Unmarshal(raw)
	if err != nil {
		return nil, fmt.Errorf("storage: decode object %s: %w", id.Hex(), err)
	}
	actualID := object.ComputeID(typ, data)
	if actualID != id {
		return nil, fmt.Errorf("storage: object integrity check failed for %s (content hashes to %s)", id.Hex(), actualID.Hex())
	}
	return &object.Object{Type: typ, Data: data, ID: id}, nil
}

// Has reports whether id exists.
func (s *LooseStore) Has(id sha1.Digest) (bool, error) {
	_, err := os.Stat(s.objectPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// objectPath returns .vstodo/objects/<2>/<38>.
func (s *LooseStore) objectPath(id sha1.Digest) string {
	h := id.Hex()
	return filepath.Join(s.ObjectsDir, h[:2], h[2:])
}

// Objects enumerates validly named regular loose-object files in stable order.
// Filesystem traversal errors are returned rather than silently discarded.
func (s *LooseStore) Objects() ([]sha1.Digest, error) {
	var out []sha1.Digest
	err := filepath.WalkDir(s.ObjectsDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(s.ObjectsDir, path)
		if err != nil {
			return err
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) != 2 || len(parts[0]) != 2 || len(parts[1]) != 38 {
			return nil
		}
		id, err := sha1.FromHex(parts[0] + parts[1])
		if err != nil {
			return nil
		}
		out = append(out, id)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("storage: enumerate objects: %w", err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Hex() < out[j].Hex() })
	return out, nil
}

// ReadObject reads raw payload bytes for id (decompressed).
func (s *LooseStore) ReadObject(id sha1.Digest) ([]byte, error) {
	obj, err := s.Load(id)
	if err != nil {
		return nil, err
	}
	return obj.Data, nil
}

// ReadRaw returns the compressed on-disk bytes for id.
func (s *LooseStore) ReadRaw(id sha1.Digest) ([]byte, error) {
	return os.ReadFile(s.objectPath(id))
}

// StoreRaw validates compressed bytes against id before atomically storing them.
// Existing immutable objects are never overwritten.
func (s *LooseStore) StoreRaw(id sha1.Digest, compressed []byte) error {
	raw, err := object.Decompress(compressed)
	if err != nil {
		return fmt.Errorf("storage: invalid compressed object: %w", err)
	}
	typ, data, err := object.Unmarshal(raw)
	if err != nil {
		return fmt.Errorf("storage: invalid object payload: %w", err)
	}
	if actual := object.ComputeID(typ, data); actual != id {
		return fmt.Errorf("storage: object ID mismatch: requested %s, content hashes to %s", id.Hex(), actual.Hex())
	}
	return s.storeCompressed(id, compressed, typ, data)
}

// WriteRaw stores raw payload as type and returns digest.
func (s *LooseStore) WriteRaw(typ object.Type, data []byte) (sha1.Digest, error) {
	return s.Store(&object.Object{Type: typ, Data: data})
}
