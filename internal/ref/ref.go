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

// Store persists refs under a gitDir.
type Store struct {
	GitDir string
}

// NewStore opens the ref store under gitDir.
func NewStore(gitDir string) *Store { return &Store{GitDir: gitDir} }

// ReadRef resolves refName (e.g. "refs/heads/main") to a digest.
func (s *Store) ReadRef(refName string) (sha1.Digest, error) {
	path := filepath.Join(s.GitDir, refName)
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return sha1.Zero, fmt.Errorf("%w: %s", ErrNotFound, refName)
		}
		return sha1.Zero, err
	}
	txt := strings.TrimSpace(string(b))
	if strings.HasPrefix(txt, "ref: ") {
		target := strings.TrimPrefix(txt, "ref: ")
		return s.readSymbolic(target)
	}
	d, err := sha1.FromHex(txt)
	if err != nil {
		return sha1.Zero, fmt.Errorf("ref: malformed %s", refName)
	}
	return d, nil
}

func (s *Store) readSymbolic(refName string) (sha1.Digest, error) {
	path := filepath.Join(s.GitDir, refName)
	b, err := os.ReadFile(path)
	if err != nil {
		return sha1.Zero, err
	}
	d, err := sha1.FromHex(strings.TrimSpace(string(b)))
	if err != nil {
		return sha1.Zero, fmt.Errorf("ref: malformed symbolic %s", refName)
	}
	return d, nil
}

// WriteRef sets refName to digest (non-symbolic).
func (s *Store) WriteRef(refName string, d sha1.Digest) error {
	path := filepath.Join(s.GitDir, refName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(d.Hex()+"\n"), 0o644)
}

// WriteRefSymbolic makes refName point to another ref (e.g. HEAD -> refs/heads/main).
func (s *Store) WriteRefSymbolic(refName, target string) error {
	path := filepath.Join(s.GitDir, refName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte("ref: "+target+"\n"), 0o644)
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
		if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
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

// DeleteRef removes refName.
func (s *Store) DeleteRef(refName string) error {
	path := filepath.Join(s.GitDir, refName)
	return os.Remove(path)
}

// CurrentBranch returns the short branch name HEAD points to, or "" if detached.
func (s *Store) CurrentBranch() (string, error) {
	b, err := os.ReadFile(filepath.Join(s.GitDir, "HEAD"))
	if err != nil {
		return "", err
	}
	txt := strings.TrimSpace(string(b))
	if strings.HasPrefix(txt, "ref: refs/heads/") {
		return strings.TrimPrefix(txt, "ref: refs/heads/"), nil
	}
	return "", nil
}
