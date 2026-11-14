// Package main shared helpers for vstodo commands.
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
	"vstodo/internal/tree"
)

// readBlob returns the content of the blob object at id.
func readBlob(a *app.App, id sha1.Digest) ([]byte, error) {
	obj, err := a.Store.Load(id)
	if err != nil {
		return nil, err
	}
	if obj.Type != object.BlobType {
		return nil, fmt.Errorf("object %s is not a blob", id.Hex())
	}
	return obj.Data, nil
}

// writeFileFromIndex writes all staged files from the index into the working tree.
func writeFilesFromIndex(a *app.App) error {
	for path, id := range a.Index.Entries() {
		data, err := readBlob(a, id)
		if err != nil {
			return err
		}
		full := a.Repo.Path(path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// walkFiles returns all regular files under root, relative paths.
func walkFiles(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if strings.HasPrefix(rel, ".vstodo"+string(filepath.Separator)) || rel == ".vstodo" || strings.HasPrefix(rel, ".git"+string(filepath.Separator)) || rel == ".git" {
			return nil
		}
		out = append(out, rel)
		return nil
	})
	return out, err
}

// buildTreeFromIndex recursively builds tree objects from the index and returns the root tree id.
func buildTreeFromIndex(a *app.App, entries map[string]sha1.Digest, prefix string) (sha1.Digest, error) {
	// Group entries by top-level component.
	type dirEntry struct {
		mode string
		name string
		id   sha1.Digest
	}
	subdirs := make(map[string]map[string]sha1.Digest)
	var tops []dirEntry
	for path, id := range entries {
		rel := strings.TrimPrefix(path, prefix)
		rel = strings.TrimLeft(rel, "/")
		parts := strings.SplitN(rel, string(filepath.Separator), 2)
		name := parts[0]
		if len(parts) == 1 {
			tops = append(tops, dirEntry{mode: "100644", name: name, id: id})
		} else {
			sub := parts[0]
			if subdirs[sub] == nil {
				subdirs[sub] = make(map[string]sha1.Digest)
			}
			subdirs[sub][prefix+rel] = id
		}
	}
	var te []tree.Entry
	for _, t := range tops {
		te = append(te, tree.Entry{Mode: t.mode, Name: t.name, ID: t.id})
	}
	for sub, subents := range subdirs {
		subID, err := buildTreeFromIndex(a, subents, prefix+sub+string(filepath.Separator))
		if err != nil {
			return sha1.Zero, err
		}
		te = append(te, tree.Entry{Mode: "040000", Name: sub, ID: subID})
	}
	t := &tree.Tree{Entries: te}
	data := t.Marshal()
	if _, err := a.Store.Store(&object.Object{Type: object.TreeType, Data: data}); err != nil {
		return sha1.Zero, err
	}
	return object.HashTree(data), nil
}

// treeToMap flattens a tree object into path->blob id, recursing into subtrees.
func treeToMap(a *app.App, id sha1.Digest, prefix string) (map[string]sha1.Digest, error) {
	obj, err := a.Store.Load(id)
	if err != nil {
		return nil, err
	}
	if obj.Type != object.TreeType {
		return nil, fmt.Errorf("object %s is not a tree", id.Hex())
	}
	t, err := tree.Parse(id, obj.Data)
	if err != nil {
		return nil, err
	}
	out := make(map[string]sha1.Digest)
	for _, e := range t.Entries {
		p := prefix + e.Name
		if e.Mode == "040000" {
			sub, err := treeToMap(a, e.ID, p+string(filepath.Separator))
			if err != nil {
				return nil, err
			}
			for k, v := range sub {
				out[k] = v
			}
		} else {
			out[p] = e.ID
		}
	}
	return out, nil
}

// currentCommitID resolves HEAD to a commit id.
func currentCommitID(a *app.App) (sha1.Digest, error) {
	return a.Refs.Head()
}

// ensureIndexFromHEAD upgrades repositories created by older VSTodo versions,
// which cleared the index after each commit, to a full-snapshot index.
func ensureIndexFromHEAD(a *app.App) error {
	if len(a.Index.Entries()) != 0 {
		return nil
	}
	marker := a.Repo.GitPath("index.snapshot-v1")
	if _, err := os.Stat(marker); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	headID, err := currentCommitID(a)
	if err != nil || headID == sha1.Zero {
		return nil
	}
	obj, err := a.Store.Load(headID)
	if err != nil {
		return err
	}
	if obj.Type != object.CommitType {
		return fmt.Errorf("HEAD %s does not point to a commit", headID.Hex())
	}
	parsed, err := commit.Parse(headID, obj.Data)
	if err != nil {
		return err
	}
	entries, err := treeToMap(a, parsed.Tree, "")
	if err != nil {
		return err
	}
	for path, id := range entries {
		a.Index.Add(path, id)
	}
	return nil
}

// worktreePath resolves a repository-relative path and rejects traversal or
// absolute paths. Tree objects and command arguments must never escape the
// working tree when used for filesystem operations.
func worktreePath(a *app.App, rel string) (string, error) {
	if rel == "" || filepath.IsAbs(rel) {
		return "", fmt.Errorf("invalid working-tree path %q", rel)
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("working-tree path escapes repository: %q", rel)
	}
	root, err := filepath.Abs(a.Repo.Root)
	if err != nil {
		return "", err
	}
	full := filepath.Join(root, clean)
	resolved, err := filepath.Rel(root, full)
	if err != nil || resolved == ".." || strings.HasPrefix(resolved, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("working-tree path escapes repository: %q", rel)
	}
	return full, nil
}

// writeFile ensures parent dirs exist and writes data inside the working tree.
func writeFile(a *app.App, rel string, data []byte) error {
	full, err := worktreePath(a, rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, data, 0o644)
}

// removeFile deletes a working-tree file if present.
func removeFile(a *app.App, rel string) error {
	full, err := worktreePath(a, rel)
	if err != nil {
		return err
	}
	if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// isTracked reports whether rel is in the index.
func isTracked(a *app.App, rel string) bool { return a.Index.Contains(rel) }

// stagedDigest returns the staged digest for rel or Zero.
func stagedDigest(a *app.App, rel string) sha1.Digest { return a.Index.Get(rel) }
