// Package transport provides local repository transport: clone and fetch
// between local filesystems, no network required.
package transport

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"vstodo/internal/app"
	"vstodo/internal/ref"
	"vstodo/internal/sha1"
	"vstodo/internal/storage"
)

// Transport copies objects and refs between repositories.
type Transport interface {
	// Clone initializes dst, copies all objects from src, copies all refs
	// (heads + tags) and HEAD, then checks out HEAD in dst.
	Clone(src, dst *app.App) error
	// Fetch copies missing objects from src into dst and updates refs.
	Fetch(src, dst *app.App) error
}

// LocalTransport implements Transport using only filesystem operations.
type LocalTransport struct{}

// NewLocalTransport returns a local-only transport.
func NewLocalTransport() *LocalTransport { return &LocalTransport{} }

// Clone copies src into a newly initialized dst.
func (t *LocalTransport) Clone(src, dst *app.App) error {
	if err := copyObjects(src.Store, dst.Store); err != nil {
		return fmt.Errorf("clone: copy objects: %w", err)
	}
	if err := copyRefs(src.Refs, dst.Refs); err != nil {
		return fmt.Errorf("clone: copy refs: %w", err)
	}
	if err := copyHEAD(src.Refs, dst.Refs); err != nil {
		return fmt.Errorf("clone: copy HEAD: %w", err)
	}
	return checkoutHEAD(dst)
}

// Fetch copies missing objects and updates refs.
func (t *LocalTransport) Fetch(src, dst *app.App) error {
	if err := fetchMissingObjects(src.Store, dst.Store); err != nil {
		return fmt.Errorf("fetch: copy missing objects: %w", err)
	}
	if err := copyRefs(src.Refs, dst.Refs); err != nil {
		return fmt.Errorf("fetch: copy refs: %w", err)
	}
	return nil
}

// copyObjects reads every object from src and stores it in dst (skipping
// objects dst already has). Objects are copied as raw compressed files so
// the content-addressed ids are preserved exactly.
func copyObjects(src, dst *storage.LooseStore) error {
	ids, err := src.Objects()
	if err != nil {
		return err
	}
	for _, id := range ids {
		has, err := dst.Has(id)
		if err != nil {
			return err
		}
		if has {
			continue
		}
		raw, err := src.ReadRaw(id)
		if err != nil {
			return err
		}
		if err := dst.StoreRaw(id, raw); err != nil {
			return err
		}
	}
	return nil
}

// fetchMissingObjects copies objects present in src but missing from dst.
func fetchMissingObjects(src, dst *storage.LooseStore) error {
	ids, err := src.Objects()
	if err != nil {
		return err
	}
	for _, id := range ids {
		has, err := dst.Has(id)
		if err != nil {
			return err
		}
		if has {
			continue
		}
		raw, err := src.ReadRaw(id)
		if err != nil {
			return err
		}
		if err := dst.StoreRaw(id, raw); err != nil {
			return err
		}
	}
	return nil
}

// copyRefs copies all heads and tags refs from src into dst.
func copyRefs(src, dst *ref.Store) error {
	refs, err := src.ListRefs()
	if err != nil {
		return err
	}
	for name, id := range refs {
		if err := dst.WriteRef(name, id); err != nil {
			return err
		}
	}
	return nil
}

// copyHEAD copies the HEAD ref from src into dst.
func copyHEAD(src, dst *ref.Store) error {
	headPath := filepath.Join(src.GitDir, "HEAD")
	b, err := os.ReadFile(headPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	dstHEAD := filepath.Join(dst.GitDir, "HEAD")
	if err := os.MkdirAll(filepath.Dir(dstHEAD), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dstHEAD, b, 0o644)
}

// checkoutHEAD checks out HEAD in dst so the working tree matches.
func checkoutHEAD(a *app.App) error {
	headID, err := a.Refs.Head()
	if err != nil {
		return fmt.Errorf("checkout HEAD: %w", err)
	}
	if headID == sha1.Zero {
		return nil
	}
	obj, err := a.Store.Load(headID)
	if err != nil {
		return fmt.Errorf("checkout HEAD: load commit: %w", err)
	}
	return writeTreeToWorktree(a, obj.Data)
}

// writeTreeToWorktree recursively writes the tree at headID into dst's
// working tree, mirroring checkout semantics.
func writeTreeToWorktree(a *app.App, data []byte) error {
	treeID, err := objectTreeID(a, data)
	if err != nil {
		return err
	}
	return writeTreeRecursive(a, treeID, "")
}

func objectTreeID(a *app.App, commitData []byte) (sha1.Digest, error) {
	// parse "tree <hex>" from commit header.
	idx := -1
	for i := 0; i+4 < len(commitData); i++ {
		if commitData[i] == 't' && commitData[i+1] == 'r' && commitData[i+2] == 'e' && commitData[i+3] == 'e' && commitData[i+4] == ' ' {
			idx = i
			break
		}
	}
	if idx < 0 {
		return sha1.Zero, errors.New("transport: commit missing tree header")
	}
	start := idx + 5
	end := start
	for end < len(commitData) && commitData[end] != '\n' {
		end++
	}
	if end-start != 40 {
		return sha1.Zero, errors.New("transport: bad tree id length")
	}
	d, err := sha1.FromHex(string(commitData[start:end]))
	if err != nil {
		return sha1.Zero, err
	}
	return d, nil
}

func writeTreeRecursive(a *app.App, treeID sha1.Digest, prefix string) error {
	obj, err := a.Store.Load(treeID)
	if err != nil {
		return err
	}
	// raw tree: entries "<mode> <name>\0<20-byte id>"
	rest := obj.Data
	for len(rest) > 0 {
		nul := -1
		for i := 0; i < len(rest); i++ {
			if rest[i] == 0 {
				nul = i
				break
			}
		}
		if nul < 0 {
			return errors.New("transport: malformed tree entry")
		}
		meta := rest[:nul]
		rest = rest[nul+1:]
		if len(rest) < 20 {
			return errors.New("transport: truncated tree hash")
		}
		idBytes := rest[:20]
		rest = rest[20:]
		sp := -1
		for i := 0; i < len(meta); i++ {
			if meta[i] == ' ' {
				sp = i
				break
			}
		}
		if sp < 0 {
			return errors.New("transport: malformed tree meta")
		}
		mode := string(meta[:sp])
		name := string(meta[sp+1:])
		var id sha1.Digest
		copy(id[:], idBytes)

		fullPath := filepath.Join(prefix, name)
		if mode == "040000" {
			if err := os.MkdirAll(filepath.Join(a.Repo.Root, fullPath), 0o755); err != nil {
				return err
			}
			if err := writeTreeRecursive(a, id, fullPath); err != nil {
				return err
			}
			continue
		}
		blobObj, err := a.Store.Load(id)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(a.Repo.Root, fullPath), blobObj.Data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// compile-time checks
var _ Transport = (*LocalTransport)(nil)
