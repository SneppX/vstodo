package main

import (
	"fmt"
	"os"
	"sort"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

// cmdStatus shows staged, unstaged, and untracked changes.
func cmdStatus(a *app.App, args []string) error {
	if err := ensureIndexFromHEAD(a); err != nil {
		return err
	}
	if err := a.SaveIndex(); err != nil {
		return err
	}

	branch, err := a.Refs.CurrentBranch()
	if err != nil || branch == "" {
		branch = "(detached)"
	}
	fmt.Printf("On branch %s\n", branch)

	headID, err := currentCommitID(a)
	if err != nil {
		return err
	}
	var headTree map[string]sha1.Digest
	if headID == sha1.Zero {
		fmt.Println("No commits yet")
		headTree = make(map[string]sha1.Digest)
	} else {
		obj, err := a.Store.Load(headID)
		if err != nil {
			return err
		}
		if obj.Type != object.CommitType {
			return fmt.Errorf("HEAD %s does not point to a commit", headID.Hex())
		}
		c, err := commit.Parse(headID, obj.Data)
		if err != nil {
			return err
		}
		headTree, err = treeToMap(a, c.Tree, "")
		if err != nil {
			return err
		}
	}

	indexEntries := a.Index.Entries()
	var stagedNew, stagedModified, stagedDeleted []string
	for path, id := range indexEntries {
		oldID, existed := headTree[path]
		if !existed {
			stagedNew = append(stagedNew, path)
		} else if oldID != id {
			stagedModified = append(stagedModified, path)
		}
	}
	for path := range headTree {
		if _, exists := indexEntries[path]; !exists {
			stagedDeleted = append(stagedDeleted, path)
		}
	}
	sort.Strings(stagedNew)
	sort.Strings(stagedModified)
	sort.Strings(stagedDeleted)
	if len(stagedNew)+len(stagedModified)+len(stagedDeleted) > 0 {
		fmt.Println("Changes to be committed:")
		for _, path := range stagedNew {
			fmt.Printf("  new file:      %s\n", path)
		}
		for _, path := range stagedModified {
			fmt.Printf("  modified:      %s\n", path)
		}
		for _, path := range stagedDeleted {
			fmt.Printf("  deleted:       %s\n", path)
		}
	}

	files, err := walkFiles(a.Repo.Root)
	if err != nil {
		return err
	}
	seen := make(map[string]bool, len(files))
	var modified, untracked, deleted []string
	for _, path := range files {
		stagedID, tracked := indexEntries[path]
		if !tracked {
			untracked = append(untracked, path)
			continue
		}
		fullPath, err := worktreePath(a, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return err
		}
		if object.HashBlob(data) != stagedID {
			modified = append(modified, path)
		}
		seen[path] = true
	}
	for path := range indexEntries {
		if !seen[path] {
			deleted = append(deleted, path)
		}
	}
	sort.Strings(modified)
	sort.Strings(untracked)
	sort.Strings(deleted)
	if len(modified) > 0 {
		fmt.Println("Changes not staged for commit:")
		for _, path := range modified {
			fmt.Printf("  modified:      %s\n", path)
		}
	}
	if len(deleted) > 0 {
		fmt.Println("Deleted from working tree:")
		for _, path := range deleted {
			fmt.Printf("  deleted:       %s\n", path)
		}
	}
	if len(untracked) > 0 {
		fmt.Println("Untracked files:")
		for _, path := range untracked {
			fmt.Printf("  %s\n", path)
		}
	}
	if len(stagedNew)+len(stagedModified)+len(stagedDeleted)+len(modified)+len(deleted)+len(untracked) == 0 {
		fmt.Println("Working tree clean")
	}
	return nil
}
