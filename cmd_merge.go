package main

import (
	"fmt"
	"os"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

// cmdMerge performs a safe fast-forward merge. Divergent histories are rejected
// until a real three-way merge implementation is available; creating a merge
// commit that reuses only the current tree would silently lose the other side.
func cmdMerge(a *app.App, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: vstodo merge <branch>")
	}
	branch := args[0]
	currentID, err := currentCommitID(a)
	if err != nil || currentID == sha1.Zero {
		return fmt.Errorf("nothing to merge into")
	}
	otherID, err := a.Refs.ReadRef("refs/heads/" + branch)
	if err != nil {
		return fmt.Errorf("no such branch: %s", branch)
	}
	if otherID == currentID {
		fmt.Println("Already up to date.")
		return nil
	}

	currentObj, err := a.Store.Load(currentID)
	if err != nil {
		return err
	}
	currentCommit, err := commit.Parse(currentID, currentObj.Data)
	if err != nil {
		return err
	}
	otherObj, err := a.Store.Load(otherID)
	if err != nil {
		return err
	}
	otherCommit, err := commit.Parse(otherID, otherObj.Data)
	if err != nil {
		return err
	}

	currentIsAncestor, err := isCommitAncestor(a, currentID, otherID)
	if err != nil {
		return err
	}
	if !currentIsAncestor {
		otherIsAncestor, err := isCommitAncestor(a, otherID, currentID)
		if err != nil {
			return err
		}
		if otherIsAncestor {
			fmt.Println("Already up to date.")
			return nil
		}
		return fmt.Errorf("cannot merge divergent histories yet; refusing to create a merge commit that would omit changes from %q", branch)
	}

	currentBranch, err := a.Refs.CurrentBranch()
	if err != nil || currentBranch == "" {
		return fmt.Errorf("cannot fast-forward while HEAD is detached")
	}
	if err := requireCleanWorkingTree(a, currentCommit.Tree); err != nil {
		return err
	}
	currentTree, err := treeToMap(a, currentCommit.Tree, "")
	if err != nil {
		return err
	}
	otherTree, err := treeToMap(a, otherCommit.Tree, "")
	if err != nil {
		return err
	}

	// Preflight every target blob before changing the working tree.
	contents := make(map[string][]byte, len(otherTree))
	for path, id := range otherTree {
		data, err := readBlob(a, id)
		if err != nil {
			return fmt.Errorf("merge: read %s: %w", path, err)
		}
		contents[path] = data
	}
	for path := range currentTree {
		if _, exists := otherTree[path]; !exists {
			if err := removeFile(a, path); err != nil {
				return fmt.Errorf("merge: remove %s: %w", path, err)
			}
		}
	}
	for path, data := range contents {
		if err := writeFile(a, path, data); err != nil {
			return fmt.Errorf("merge: write %s: %w", path, err)
		}
	}
	if err := a.Refs.WriteRef("refs/heads/"+currentBranch, otherID); err != nil {
		return err
	}
	fmt.Printf("Fast-forward %s to %s\n", currentBranch, otherID.Hex()[:7])
	return nil
}

func isCommitAncestor(a *app.App, ancestor, descendant sha1.Digest) (bool, error) {
	queue := []sha1.Digest{descendant}
	visited := make(map[sha1.Digest]bool)
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if id == ancestor {
			return true, nil
		}
		if visited[id] {
			continue
		}
		visited[id] = true
		obj, err := a.Store.Load(id)
		if err != nil {
			return false, err
		}
		if obj.Type != object.CommitType {
			return false, fmt.Errorf("merge: object %s is not a commit", id.Hex())
		}
		parsed, err := commit.Parse(id, obj.Data)
		if err != nil {
			return false, err
		}
		queue = append(queue, parsed.Parents...)
	}
	return false, nil
}

func requireCleanWorkingTree(a *app.App, headTree sha1.Digest) error {
	tracked, err := treeToMap(a, headTree, "")
	if err != nil {
		return err
	}
	staged := a.Index.Entries()
	if len(staged) != len(tracked) {
		return fmt.Errorf("merge: staged changes present; commit or unstage them first")
	}
	for path, id := range tracked {
		if staged[path] != id {
			return fmt.Errorf("merge: staged changes present; commit or unstage them first")
		}
	}
	files, err := walkFiles(a.Repo.Root)
	if err != nil {
		return err
	}
	seen := make(map[string]bool, len(files))
	for _, path := range files {
		id, exists := tracked[path]
		if !exists {
			return fmt.Errorf("merge: untracked file %q would be at risk; move or remove it first", path)
		}
		fullPath, err := worktreePath(a, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return err
		}
		if object.HashBlob(data) != id {
			return fmt.Errorf("merge: local changes in %q; commit or restore them first", path)
		}
		seen[path] = true
	}
	for path := range tracked {
		if !seen[path] {
			return fmt.Errorf("merge: tracked file %q is missing; restore it first", path)
		}
	}
	return nil
}
