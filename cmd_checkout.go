package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

// cmdCheckout safely switches to a branch or commit after verifying that the
// current index and working tree contain no changes that could be overwritten.
func cmdCheckout(a *app.App, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: vstodo checkout <branch|commit>")
	}
	target := args[0]
	var id sha1.Digest
	branchName := ""

	if branchID, err := a.Refs.ReadRef("refs/heads/" + target); err == nil {
		id = branchID
		branchName = target
	} else {
		var err error
		id, err = sha1.FromHex(target)
		if err != nil {
			return fmt.Errorf("not a branch or commit: %s", target)
		}
	}

	var currentTree map[string]sha1.Digest
	headID, headErr := currentCommitID(a)
	if headErr == nil && headID != sha1.Zero {
		headObj, err := a.Store.Load(headID)
		if err != nil {
			return fmt.Errorf("checkout: load current HEAD: %w", err)
		}
		if headObj.Type != object.CommitType {
			return fmt.Errorf("checkout: current HEAD %s is not a commit", headID.Hex())
		}
		headCommit, err := commit.Parse(headID, headObj.Data)
		if err != nil {
			return fmt.Errorf("checkout: parse current HEAD: %w", err)
		}
		currentTree, err = treeToMap(a, headCommit.Tree, "")
		if err != nil {
			return err
		}
		if err := requireCleanWorkingTree(a, headCommit.Tree); err != nil {
			return fmt.Errorf("checkout: %w", err)
		}
	} else {
		currentTree = make(map[string]sha1.Digest)
		if err := requireCleanWorkingTree(a, sha1.Zero); err != nil {
			return fmt.Errorf("checkout: %w", err)
		}
	}

	targetObj, err := a.Store.Load(id)
	if err != nil {
		return fmt.Errorf("checkout: load target: %w", err)
	}
	if targetObj.Type != object.CommitType {
		return fmt.Errorf("checkout: target %s is not a commit", id.Hex())
	}
	targetCommit, err := commit.Parse(id, targetObj.Data)
	if err != nil {
		return fmt.Errorf("checkout: parse target: %w", err)
	}
	targetTree, err := treeToMap(a, targetCommit.Tree, "")
	if err != nil {
		return err
	}
	if err := validateCheckoutTreePaths(targetTree); err != nil {
		return err
	}

	// Read and validate every target blob before touching the working tree.
	contents := make(map[string][]byte, len(targetTree))
	for path, blobID := range targetTree {
		if _, err := worktreePath(a, path); err != nil {
			return err
		}
		data, err := readBlob(a, blobID)
		if err != nil {
			return fmt.Errorf("checkout: read %s: %w", path, err)
		}
		contents[path] = data
	}

	// Remove obsolete tracked files first, then prune empty directories. This
	// also supports file-to-directory and directory-to-file transitions.
	for path := range currentTree {
		if _, exists := targetTree[path]; exists {
			continue
		}
		if err := removeFile(a, path); err != nil {
			return fmt.Errorf("checkout: remove %s: %w", path, err)
		}
		if err := pruneEmptyParentDirs(a, path); err != nil {
			return err
		}
	}
	for path, data := range contents {
		if err := writeFile(a, path, data); err != nil {
			return fmt.Errorf("checkout: write %s: %w", path, err)
		}
	}

	// Replace the full index snapshot; do not retain entries from the old HEAD.
	for path := range a.Index.Entries() {
		a.Index.Remove(path)
	}
	for path, blobID := range targetTree {
		a.Index.Add(path, blobID)
	}
	if err := a.SaveIndex(); err != nil {
		return fmt.Errorf("checkout: save index: %w", err)
	}

	if branchName != "" {
		if err := a.Refs.SetHead("ref: refs/heads/" + branchName); err != nil {
			return err
		}
		fmt.Printf("Switched to branch '%s'\n", branchName)
	} else {
		if err := a.Refs.SetHead(id.Hex()); err != nil {
			return err
		}
		fmt.Printf("Note: detached HEAD at %s\n", id.Hex()[:7])
	}
	return nil
}

func validateCheckoutTreePaths(entries map[string]sha1.Digest) error {
	for path := range entries {
		parts := strings.Split(path, "/")
		for i := 1; i < len(parts); i++ {
			parent := strings.Join(parts[:i], "/")
			if _, exists := entries[parent]; exists {
				return fmt.Errorf("checkout: invalid tree: %q is both a file and a parent directory", parent)
			}
		}
	}
	return nil
}

func pruneEmptyParentDirs(a *app.App, rel string) error {
	full, err := worktreePath(a, rel)
	if err != nil {
		return err
	}
	root, err := filepath.Abs(a.Repo.Root)
	if err != nil {
		return err
	}
	dir := filepath.Dir(full)
	for dir != root {
		if err := os.Remove(dir); err != nil {
			if os.IsNotExist(err) {
				dir = filepath.Dir(dir)
				continue
			}
			// Non-empty directories are intentionally retained.
			break
		}
		dir = filepath.Dir(dir)
	}
	return nil
}
