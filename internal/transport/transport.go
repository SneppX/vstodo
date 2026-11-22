// Package transport provides local repository transport: clone and fetch
// between local filesystems, no network required.
package transport

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vstodo/internal/app"
	"vstodo/internal/object"
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
func copyObjects(src, dst *storage.PackStore) error {
	ids, err := src.AllObjects()
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
func fetchMissingObjects(src, dst *storage.PackStore) error {
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
	if obj.Type != object.CommitType {
		return fmt.Errorf("checkout HEAD: object %s is %s, not a commit", headID.Hex(), obj.Type)
	}
	return writeTreeToWorktree(a, obj.Data)
}

type worktreeFile struct {
	path string
	data []byte
}

func writeTreeToWorktree(a *app.App, commitData []byte) error {
	treeID, err := objectTreeID(a, commitData)
	if err != nil {
		return err
	}
	files := make([]worktreeFile, 0)
	seen := make(map[string]bool)
	if err := collectTreeFiles(a, treeID, nil, &files, seen); err != nil {
		return err
	}

	root, err := filepath.EvalSymlinks(a.Repo.Root)
	if err != nil {
		return fmt.Errorf("transport: resolve worktree root: %w", err)
	}
	for _, file := range files {
		parts := strings.Split(file.path, "/")
		parent, err := ensureSafeDirectory(root, parts[:len(parts)-1])
		if err != nil {
			return err
		}
		target := filepath.Join(parent, parts[len(parts)-1])
		info, err := os.Lstat(target)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("transport: refusing to write through symlink %q", file.path)
			}
			if info.IsDir() {
				return fmt.Errorf("transport: file path %q is an existing directory", file.path)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("transport: inspect %q: %w", file.path, err)
		}
		if err := os.WriteFile(target, file.data, 0o644); err != nil {
			return fmt.Errorf("transport: write %q: %w", file.path, err)
		}
	}
	return nil
}

func objectTreeID(_ *app.App, commitData []byte) (sha1.Digest, error) {
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

func collectTreeFiles(a *app.App, treeID sha1.Digest, prefix []string, files *[]worktreeFile, seen map[string]bool) error {
	obj, err := a.Store.Load(treeID)
	if err != nil {
		return err
	}
	if obj.Type != object.TreeType {
		return fmt.Errorf("transport: tree entry %s points to %s, not a tree", treeID.Hex(), obj.Type)
	}
	rest := obj.Data
	for len(rest) > 0 {
		nul := strings.IndexByte(string(rest), 0)
		if nul < 0 {
			return errors.New("transport: malformed tree entry")
		}
		meta := rest[:nul]
		rest = rest[nul+1:]
		if len(rest) < 20 {
			return errors.New("transport: truncated tree hash")
		}
		id, err := sha1.FromBytes(rest[:20])
		if err != nil {
			return err
		}
		rest = rest[20:]
		sp := strings.IndexByte(string(meta), ' ')
		if sp <= 0 || sp == len(meta)-1 {
			return errors.New("transport: malformed tree meta")
		}
		mode, name := string(meta[:sp]), string(meta[sp+1:])
		if err := validateTreeName(name, prefix, a.Repo.GitDir); err != nil {
			return err
		}
		parts := append(append([]string(nil), prefix...), name)
		path := strings.Join(parts, "/")
		isDirectory := mode == "40000" || mode == "040000"
		if mode != "100644" && mode != "100755" && mode != "120000" && !isDirectory {
			return fmt.Errorf("transport: unsupported tree mode %q for %q", mode, path)
		}
		if _, exists := seen[path]; exists {
			return fmt.Errorf("transport: duplicate tree path %q", path)
		}
		seen[path] = true
		if isDirectory {
			if err := collectTreeFiles(a, id, parts, files, seen); err != nil {
				return err
			}
			continue
		}
		blob, err := a.Store.Load(id)
		if err != nil {
			return fmt.Errorf("transport: load blob %q: %w", path, err)
		}
		if blob.Type != object.BlobType {
			return fmt.Errorf("transport: file entry %q points to %s, not a blob", path, blob.Type)
		}
		*files = append(*files, worktreeFile{path: path, data: blob.Data})
	}
	return nil
}

func validateTreeName(name string, prefix []string, gitDir string) error {
	if name == "" || name == "." || name == ".." || filepath.IsAbs(name) ||
		strings.ContainsAny(name, "/\\:") || filepath.VolumeName(name) != "" {
		return fmt.Errorf("transport: unsafe tree entry name %q", name)
	}
	if len(prefix) == 0 {
		metadataName := filepath.Base(gitDir)
		if strings.EqualFold(name, metadataName) || strings.EqualFold(name, ".vstodo") {
			return fmt.Errorf("transport: tree entry %q targets repository metadata", name)
		}
	}
	return nil
}

// ensureSafeDirectory creates missing directories one component at a time and
// rejects symlinks or non-directory components instead of following them.
func ensureSafeDirectory(root string, components []string) (string, error) {
	current := root
	for _, component := range components {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			if err := os.Mkdir(current, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
				return "", fmt.Errorf("transport: create directory %q: %w", current, err)
			}
			info, err = os.Lstat(current)
		}
		if err != nil {
			return "", fmt.Errorf("transport: inspect directory %q: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", fmt.Errorf("transport: refusing unsafe directory component %q", current)
		}
	}
	return current, nil
}

// compile-time checks
var _ Transport = (*LocalTransport)(nil)
