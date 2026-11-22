// Package storage provides durable loose and pack-backed object storage.
package storage

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"vstodo/internal/object"
	"vstodo/internal/packfile"
	"vstodo/internal/sha1"
)

// PackStore keeps newly written objects loose and stores received packs
// immutably under objects/pack. Pack files are published before their indexes;
// an incomplete pair is therefore ignored by readers.
type PackStore struct {
	Loose    *LooseStore
	PacksDir string
	mu       sync.Mutex
	cache    map[string]*packfile.Pack
}

// NewPackStore creates a pack-aware store around an existing loose store.
func NewPackStore(loose *LooseStore, packsDir string) *PackStore {
	return &PackStore{Loose: loose, PacksDir: packsDir, cache: make(map[string]*packfile.Pack)}
}

// Store preserves the normal append-only loose-object write path.
func (s *PackStore) Store(obj *object.Object) (sha1.Digest, error) {
	if s == nil || s.Loose == nil {
		return sha1.Zero, errors.New("storage: pack store has no loose store")
	}
	return s.Loose.Store(obj)
}

// StorePack validates and durably publishes a Git pack and its v2 index.
// The checksum-derived filename makes pack publication idempotent.
func (s *PackStore) StorePack(data []byte) (int, error) {
	if s == nil || s.Loose == nil {
		return 0, errors.New("storage: pack store has no loose store")
	}
	pack, err := packfile.ReadPack(data)
	if err != nil {
		return 0, fmt.Errorf("storage: validate pack: %w", err)
	}
	idxBytes, err := packfile.WriteIndex(packfile.BuildIndexFromPack(pack))
	if err != nil {
		return 0, fmt.Errorf("storage: build pack index: %w", err)
	}
	if err := os.MkdirAll(s.PacksDir, 0o755); err != nil {
		return 0, fmt.Errorf("storage: create pack directory: %w", err)
	}
	stem := "pack-" + pack.Checksum.Hex()
	packPath := filepath.Join(s.PacksDir, stem+".pack")
	idxPath := filepath.Join(s.PacksDir, stem+".idx")
	if err := publishImmutable(packPath, data); err != nil {
		return 0, fmt.Errorf("storage: publish pack: %w", err)
	}
	if err := publishImmutable(idxPath, idxBytes); err != nil {
		return 0, fmt.Errorf("storage: publish pack index: %w", err)
	}
	s.mu.Lock()
	delete(s.cache, packPath)
	s.mu.Unlock()
	return len(pack.Objects), nil
}

// Load reads an object from loose storage first, then from an indexed pack.
func (s *PackStore) Load(id sha1.Digest) (*object.Object, error) {
	if s == nil || s.Loose == nil {
		return nil, errors.New("storage: pack store has no loose store")
	}
	obj, err := s.Loose.Load(id)
	if err == nil || !errors.Is(err, os.ErrNotExist) {
		return obj, err
	}
	pack, err := s.findPack(id)
	if err != nil {
		return nil, err
	}
	if pack == nil {
		return nil, os.ErrNotExist
	}
	packed, ok := pack.Lookup(id)
	if !ok {
		return nil, os.ErrNotExist
	}
	return &object.Object{Type: packed.Type, Data: append([]byte(nil), packed.Data...), ID: id}, nil
}

// Has checks loose storage and then pack indexes without decoding pack contents.
func (s *PackStore) Has(id sha1.Digest) (bool, error) {
	if s == nil || s.Loose == nil {
		return false, errors.New("storage: pack store has no loose store")
	}
	ok, err := s.Loose.Has(id)
	if err != nil || ok {
		return ok, err
	}
	path, _, err := s.findPackIndex(id)
	return path != "", err
}

func (s *PackStore) findPack(id sha1.Digest) (*packfile.Pack, error) {
	idxPath, idx, err := s.findPackIndex(id)
	if err != nil || idxPath == "" {
		return nil, err
	}
	packPath := strings.TrimSuffix(idxPath, ".idx") + ".pack"
	s.mu.Lock()
	if cached := s.cache[packPath]; cached != nil {
		s.mu.Unlock()
		return cached, nil
	}
	s.mu.Unlock()
	data, err := os.ReadFile(packPath)
	if err != nil {
		return nil, fmt.Errorf("storage: read indexed pack %s: %w", packPath, err)
	}
	pack, err := packfile.ReadPack(data)
	if err != nil {
		return nil, fmt.Errorf("storage: decode indexed pack %s: %w", packPath, err)
	}
	if pack.Checksum != idx.PackChecksum {
		return nil, fmt.Errorf("storage: pack checksum does not match index %s", idxPath)
	}
	if _, ok := pack.Lookup(id); !ok {
		return nil, fmt.Errorf("storage: index %s references an object missing from its pack", idxPath)
	}
	s.mu.Lock()
	if existing := s.cache[packPath]; existing != nil {
		pack = existing
	} else {
		s.cache[packPath] = pack
	}
	s.mu.Unlock()
	return pack, nil
}

func (s *PackStore) findPackIndex(id sha1.Digest) (string, *packfile.PackIndex, error) {
	entries, err := os.ReadDir(s.PacksDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil, nil
		}
		return "", nil, fmt.Errorf("storage: enumerate pack indexes: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".idx") {
			continue
		}
		path := filepath.Join(s.PacksDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return "", nil, fmt.Errorf("storage: read pack index %s: %w", path, err)
		}
		idx, err := packfile.ReadIndex(data)
		if err != nil {
			return "", nil, fmt.Errorf("storage: invalid pack index %s: %w", path, err)
		}
		if _, ok := idx.Find(id); ok {
			return path, idx, nil
		}
	}
	return "", nil, nil
}

// publishImmutable writes a file using a synced temporary and atomic hard link.
// Existing content-addressed files are accepted only when their bytes match.
func publishImmutable(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".vstodo-pack-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o444); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Link(tmpPath, path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrExist) {
		return err
	}
	existing, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(existing, data) {
		return fmt.Errorf("storage: immutable file collision at %s", path)
	}
	return nil
}
