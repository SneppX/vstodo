// Package index is the staging area: path -> blob digest mapping.
// Stored as a simple line-oriented text file at .vstodo/index:
// <hex digest>\t<path>
package index

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"vstodo/internal/sha1"
)

// Index maps file paths to staged blob digests.
type Index struct {
	entries map[string]sha1.Digest
	path    string
}

// NewIndex loads (or creates) an index at path.
func NewIndex(path string) *Index {
	return &Index{entries: make(map[string]sha1.Digest), path: path}
}

// Load reads the index from disk.
func (idx *Index) Load() error {
	b, err := os.ReadFile(idx.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		d, err := sha1.FromHex(parts[0])
		if err != nil {
			continue
		}
		idx.entries[parts[1]] = d
	}
	return sc.Err()
}

// Save writes the index to disk.
func (idx *Index) Save() error {
	if err := os.MkdirAll(filepath.Dir(idx.path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	for path, d := range idx.entries {
		buf.WriteString(d.Hex() + "\t" + path + "\n")
	}
	return os.WriteFile(idx.path, buf.Bytes(), 0o644)
}

// Add stages path at digest d.
func (idx *Index) Add(path string, d sha1.Digest) { idx.entries[path] = d }

// Remove unstages path.
func (idx *Index) Remove(path string) { delete(idx.entries, path) }

// Get returns the staged digest for path, or Zero if absent.
func (idx *Index) Get(path string) sha1.Digest { return idx.entries[path] }

// Entries returns all staged path -> digest.
func (idx *Index) Entries() map[string]sha1.Digest { return idx.entries }

// Contains reports whether path is staged.
func (idx *Index) Contains(path string) bool { _, ok := idx.entries[path]; return ok }
