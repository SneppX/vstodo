// Package ref manages HEAD, branches, and tags:
// .vstodo/refs/heads, .vstodo/refs/tags, and .vstodo/HEAD.
package ref

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vstodo/internal/sha1"
)

// ErrNotFound is returned when a ref does not exist.
var ErrNotFound = errors.New("ref not found")

// ErrLockExists indicates another writer owns the reference lock.
var ErrLockExists = errors.New("reference lock exists")

// Store persists refs under a gitDir.
type Store struct {
	GitDir string
}

// NewStore opens the ref store under gitDir.
func NewStore(gitDir string) *Store { return &Store{GitDir: gitDir} }

// validateRefName rejects path traversal and invalid ref path components.
func validateRefName(name string) error {
	if name == "" || name == "@" || filepath.IsAbs(name) ||
		strings.ContainsAny(name, "\x00\\") || strings.Contains(name, "//") ||
		strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") ||
		strings.Contains(name, "..") || strings.Contains(name, "@{") ||
		strings.ContainsAny(name, " ~^:?*[") {
		return fmt.Errorf("ref: invalid reference name %q", name)
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("ref: invalid control character in reference name %q", name)
		}
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || strings.HasPrefix(part, ".") ||
			strings.HasSuffix(part, ".") || strings.HasSuffix(part, ".lock") {
			return fmt.Errorf("ref: invalid reference name %q", name)
		}
	}
	return nil
}

func (s *Store) refPath(refName string) (string, error) {
	if err := validateRefName(refName); err != nil {
		return "", err
	}
	return filepath.Join(s.GitDir, filepath.FromSlash(refName)), nil
}

// ReadRef resolves a direct or symbolic reference to an object digest.
func (s *Store) ReadRef(refName string) (sha1.Digest, error) {
	return s.readRef(refName, make(map[string]bool), 0)
}

func (s *Store) readRef(refName string, seen map[string]bool, depth int) (sha1.Digest, error) {
	if depth > 8 {
		return sha1.Zero, fmt.Errorf("ref: symbolic reference depth exceeded at %s", refName)
	}
	path, err := s.refPath(refName)
	if err != nil {
		return sha1.Zero, err
	}
	if seen[refName] {
		return sha1.Zero, fmt.Errorf("ref: symbolic reference cycle at %s", refName)
	}
	seen[refName] = true

	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return sha1.Zero, fmt.Errorf("%w: %s", ErrNotFound, refName)
		}
		return sha1.Zero, err
	}
	if !info.Mode().IsRegular() {
		return sha1.Zero, fmt.Errorf("ref: %s is not a regular file", refName)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return sha1.Zero, err
	}
	txt := strings.TrimSpace(string(b))
	if strings.HasPrefix(txt, "ref: ") {
		target := strings.TrimPrefix(txt, "ref: ")
		if err := validateRefName(target); err != nil {
			return sha1.Zero, fmt.Errorf("ref: invalid symbolic target in %s: %w", refName, err)
		}
		return s.readRef(target, seen, depth+1)
	}
	d, err := sha1.FromHex(txt)
	if err != nil {
		return sha1.Zero, fmt.Errorf("ref: malformed %s", refName)
	}
	return d, nil
}

// acquireLock creates the Git-compatible <ref>.lock file exclusively.
func (s *Store) acquireLock(refName string) (path, lockPath string, file *os.File, err error) {
	path, err = s.refPath(refName)
	if err != nil {
		return "", "", nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", "", nil, fmt.Errorf("ref: create parent directory: %w", err)
	}
	lockPath = path + ".lock"
	file, err = os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return "", "", nil, fmt.Errorf("%w: %s", ErrLockExists, refName)
		}
		return "", "", nil, fmt.Errorf("ref: acquire lock for %s: %w", refName, err)
	}
	return path, lockPath, file, nil
}

// publishLock writes, syncs, closes, and atomically publishes a locked ref.
func publishLock(path, lockPath string, file *os.File, content []byte) error {
	defer os.Remove(lockPath)
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		return fmt.Errorf("ref: write lock file: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("ref: sync lock file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("ref: close lock file: %w", err)
	}
	if err := os.Rename(lockPath, path); err != nil {
		return fmt.Errorf("ref: publish reference: %w", err)
	}
	return nil
}

// WriteRef atomically sets refName to digest (non-symbolic).
func (s *Store) WriteRef(refName string, d sha1.Digest) error {
	path, lockPath, file, err := s.acquireLock(refName)
	if err != nil {
		return err
	}
	return publishLock(path, lockPath, file, []byte(d.Hex()+"\n"))
}

// WriteRefSymbolic atomically makes refName point to another ref.
func (s *Store) WriteRefSymbolic(refName, target string) error {
	if err := validateRefName(target); err != nil {
		return fmt.Errorf("ref: invalid symbolic target: %w", err)
	}
	path, lockPath, file, err := s.acquireLock(refName)
	if err != nil {
		return err
	}
	return publishLock(path, lockPath, file, []byte("ref: "+target+"\n"))
}

// Head returns the digest HEAD points to, resolving symbolic refs.
func (s *Store) Head() (sha1.Digest, error) {
	return s.ReadRef("HEAD")
}

// SetHead sets HEAD to a digest or symbolic target.
func (s *Store) SetHead(target string) error {
	if strings.HasPrefix(target, "ref: ") {
		return s.WriteRefSymbolic("HEAD", strings.TrimPrefix(target, "ref: "))
	}
	d, err := sha1.FromHex(target)
	if err != nil {
		return err
	}
	return s.WriteRef("HEAD", d)
}

// ListRefs lists all refs under refs/heads and refs/tags.
func (s *Store) ListRefs() (map[string]sha1.Digest, error) {
	out := make(map[string]sha1.Digest)
	for _, ns := range []string{"refs/heads", "refs/tags"} {
		root := filepath.Join(s.GitDir, ns)
		if err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			name := filepath.ToSlash(filepath.Join(ns, rel))
			d, err := s.ReadRef(name)
			if err != nil {
				return err
			}
			out[name] = d
			return nil
		}); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	return out, nil
}

// DeleteRef removes refName while holding its lock.
func (s *Store) DeleteRef(refName string) error {
	path, lockPath, file, err := s.acquireLock(refName)
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(lockPath)
		return fmt.Errorf("ref: close deletion lock: %w", err)
	}
	defer os.Remove(lockPath)
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: %s", ErrNotFound, refName)
		}
		return fmt.Errorf("ref: delete %s: %w", refName, err)
	}
	return nil
}

// CurrentBranch returns the short branch name HEAD points to, or "" if detached.
func (s *Store) CurrentBranch() (string, error) {
	path, err := s.refPath("HEAD")
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	txt := strings.TrimSpace(string(b))
	if strings.HasPrefix(txt, "ref: refs/heads/") {
		return strings.TrimPrefix(txt, "ref: refs/heads/"), nil
	}
	return "", nil
}
