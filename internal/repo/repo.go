// Package repo provides repository root discovery, init, and path helpers.
// A vstodo repository lives under .vstodo (configurable via VSTODO_DIR).
package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultDir is the default repository directory name.
const DefaultDir = ".vstodo"

// EnvOverride is the environment variable that can override the dir name.
const EnvOverride = "VSTODO_DIR"

// Repository is a vstodo repository handle.
type Repository struct {
	Root   string // working tree root
	GitDir string // .vstodo directory path
}

// Find walks up from start looking for a .vstodo directory.
func Find(start string) (*Repository, error) {
	dir := func() string {
		if d := os.Getenv(EnvOverride); d != "" {
			return d
		}
		return DefaultDir
	}()

	cur, err := filepath.Abs(start)
	if err != nil {
		return nil, err
	}

	for {
		candidate := filepath.Join(cur, dir)
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return &Repository{Root: cur, GitDir: candidate}, nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return nil, fmt.Errorf("not a vstodo repository")
}

// Init creates a new empty repository at root.
func Init(root string) (*Repository, error) {
	dir := func() string {
		if d := os.Getenv(EnvOverride); d != "" {
			return d
		}
		return DefaultDir
	}()

	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, err
	}
	gitDir := filepath.Join(abs, dir)
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		return nil, err
	}
	for _, sub := range []string{"objects", "refs/heads", "refs/tags"} {
		if err := os.MkdirAll(filepath.Join(gitDir, sub), 0o755); err != nil {
			return nil, err
		}
	}
	// HEAD points to refs/heads/main
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte("[core]\n\trepositoryformatversion = 1\n\tsymlinks = false\n"), 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(gitDir, "description"), []byte("vstodo repository\n"), 0o644); err != nil {
		return nil, err
	}
	return &Repository{Root: abs, GitDir: gitDir}, nil
}

// Path joins repo-root-relative pieces under the working tree.
func (r *Repository) Path(pieces ...string) string {
	return filepath.Join(append([]string{r.Root}, pieces...)...)
}

// GitPath joins pieces under .vstodo.
func (r *Repository) GitPath(pieces ...string) string {
	return filepath.Join(append([]string{r.GitDir}, pieces...)...)
}

// ObjectsDir returns .vstodo/objects.
func (r *Repository) ObjectsDir() string { return r.GitPath("objects") }

// RefPath returns .vstodo/refs/<namespace>/<name>.
func (r *Repository) RefPath(namespace, name string) string {
	return r.GitPath("refs", namespace, name)
}

// HeadPath returns .vstodo/HEAD.
func (r *Repository) HeadPath() string { return r.GitPath("HEAD") }

// IsBare reports whether the repo is bare.
func (r *Repository) IsBare() bool {
	return strings.HasSuffix(r.Root, DefaultDir) || r.Root == r.GitDir
}
