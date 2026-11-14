// Package storage persists objects content-addressed by SHA-1.
// Loose objects live at .vstodo/objects/<2-hex>/<38-hex>, zlib-compressed.
package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

// LooseStore persists objects as loose files.
type LooseStore struct {
	ObjectsDir string
}

// NewLooseStore opens (creating) the loose object store.
func NewLooseStore(objectsDir string) *LooseStore {
	return &LooseStore{ObjectsDir: objectsDir}
}

// Store writes obj (compressed) to disk.
func (s *LooseStore) Store(obj *object.Object) (sha1.Digest, error) {
	if obj == nil {
		return sha1.Zero, fmt.Errorf("storage: nil object")
	}
	id := object.ComputeID(obj.Type, obj.Data)
	compressed, err := object.Compress(obj.Type, obj.Data)
	if err != nil {
		return id, err
	}
	path := s.objectPath(id)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return id, err
	}
	if err := os.WriteFile(path, compressed, 0o444); err != nil {
		return id, err
	}
	return id, nil
}

// Load reads and decompresses the object for id.
func (s *LooseStore) Load(id sha1.Digest) (*object.Object, error) {
	path := s.objectPath(id)
	compressed, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	raw, err := object.Decompress(compressed)
	if err != nil {
		return nil, err
	}
	typ, data, err := object.Unmarshal(raw)
	if err != nil {
		return nil, err
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

// Objects enumerates every object id present in the loose store.
// Useful for copying a store wholesale (e.g. local clone).
func (s *LooseStore) Objects() ([]sha1.Digest, error) {
	var out []sha1.Digest
	_ = filepath.WalkDir(s.ObjectsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if len(d.Name()) != 38 {
			return nil
		}
		rel, err := filepath.Rel(s.ObjectsDir, path)
		if err != nil {
			return err
		}
		// rel is "<2><sep><38>"; remove separators to form the 40-hex hash.
		h := ""
		for _, r := range rel {
			if r != '\\' && r != '/' {
				h += string(r)
			}
		}
		if len(h) != 40 {
			return nil
		}
		id, err := sha1.FromHex(h)
		if err != nil {
			return nil
		}
		out = append(out, id)
		return nil
	})
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

// StoreRaw writes compressed on-disk bytes for id directly, preserving the
// content-addressed path without re-hashing.
func (s *LooseStore) StoreRaw(id sha1.Digest, compressed []byte) error {
	path := s.objectPath(id)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.Chmod(path, 0o644); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.WriteFile(path, compressed, 0o644); err != nil {
		return err
	}
	return os.Chmod(path, 0o444)
}

// WriteRaw stores raw payload as type and returns digest.
func (s *LooseStore) WriteRaw(typ object.Type, data []byte) (sha1.Digest, error) {
	obj := &object.Object{Type: typ, Data: data}
	return s.Store(obj)
}
