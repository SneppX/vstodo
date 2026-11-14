package main

import (
	"fmt"
	"os"
	"strings"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

// cmdReset moves the current branch or detached HEAD to a target commit.
// --soft leaves the index and worktree untouched; --mixed resets the index;
// --hard resets both the index and tracked working-tree files.
func cmdReset(a *app.App, args []string) error {
	mode := "mixed"
	target := "HEAD"
	positionals := 0
	for _, arg := range args {
		switch arg {
		case "--soft", "soft":
			mode = "soft"
		case "--mixed", "mixed":
			mode = "mixed"
		case "--hard", "hard":
			mode = "hard"
		default:
			positionals++
			if positionals > 1 {
				return fmt.Errorf("usage: vstodo reset [--soft|--mixed|--hard] [target]")
			}
			target = arg
		}
	}

	targetID, err := resolveResetTarget(a, target)
	if err != nil {
		return err
	}
	targetObj, err := a.Store.Load(targetID)
	if err != nil {
		return fmt.Errorf("reset: load target: %w", err)
	}
	if targetObj.Type != object.CommitType {
		return fmt.Errorf("reset: target %s is not a commit", targetID.Hex())
	}
	targetCommit, err := commit.Parse(targetID, targetObj.Data)
	if err != nil {
		return fmt.Errorf("reset: parse target: %w", err)
	}
	targetTree, err := treeToMap(a, targetCommit.Tree, "")
	if err != nil {
		return err
	}

	if mode == "hard" {
		if err := resetHardWorktree(a, targetTree); err != nil {
			return err
		}
	}
	if mode != "soft" {
		for path := range a.Index.Entries() {
			a.Index.Remove(path)
		}
		for path, id := range targetTree {
			a.Index.Add(path, id)
		}
		if err := a.SaveIndex(); err != nil {
			return fmt.Errorf("reset: save index: %w", err)
		}
	}
	if err := updateCurrentHead(a, targetID); err != nil {
		return err
	}
	fmt.Printf("Reset %s to %s\n", mode, targetID.Hex()[:7])
	return nil
}

func resolveResetTarget(a *app.App, target string) (sha1.Digest, error) {
	if target == "HEAD" {
		id, err := currentCommitID(a)
		if err != nil {
			return sha1.Zero, fmt.Errorf("reset: resolve HEAD: %w", err)
		}
		return id, nil
	}
	if id, err := a.Refs.ReadRef("refs/heads/" + target); err == nil {
		return id, nil
	}
	if id, err := a.Refs.ReadRef("refs/tags/" + target); err == nil {
		return id, nil
	}
	id, err := sha1.FromHex(target)
	if err != nil {
		return sha1.Zero, fmt.Errorf("reset: unknown target %q", target)
	}
	return id, nil
}

func updateCurrentHead(a *app.App, id sha1.Digest) error {
	branch, err := a.Refs.CurrentBranch()
	if err == nil && branch != "" {
		return a.Refs.WriteRef("refs/heads/"+branch, id)
	}
	return a.Refs.SetHead(id.Hex())
}

func resetHardWorktree(a *app.App, target map[string]sha1.Digest) error {
	managed := make(map[string]bool)
	headID, err := currentCommitID(a)
	if err == nil && headID != sha1.Zero {
		obj, err := a.Store.Load(headID)
		if err != nil {
			return err
		}
		if obj.Type != object.CommitType {
			return fmt.Errorf("reset: current HEAD is not a commit")
		}
		c, err := commit.Parse(headID, obj.Data)
		if err != nil {
			return err
		}
		current, err := treeToMap(a, c.Tree, "")
		if err != nil {
			return err
		}
		for path := range current {
			managed[path] = true
		}
	}
	for path := range a.Index.Entries() {
		managed[path] = true
	}

	files, err := walkFiles(a.Repo.Root)
	if err != nil {
		return err
	}
	var untracked []string
	for _, path := range files {
		if !managed[path] {
			untracked = append(untracked, path)
		}
	}
	for targetPath := range target {
		for _, untrackedPath := range untracked {
			if pathsOverlap(targetPath, untrackedPath) {
				return fmt.Errorf("reset --hard: untracked file %q conflicts with target path %q", untrackedPath, targetPath)
			}
		}
	}

	contents := make(map[string][]byte, len(target))
	for path, id := range target {
		if _, err := worktreePath(a, path); err != nil {
			return err
		}
		data, err := readBlob(a, id)
		if err != nil {
			return fmt.Errorf("reset --hard: read %s: %w", path, err)
		}
		contents[path] = data
	}
	for path := range managed {
		if _, exists := target[path]; exists {
			continue
		}
		if err := removeFile(a, path); err != nil {
			return fmt.Errorf("reset --hard: remove %s: %w", path, err)
		}
		if err := pruneEmptyParentDirs(a, path); err != nil {
			return err
		}
	}
	for path, data := range contents {
		if err := writeFile(a, path, data); err != nil {
			return fmt.Errorf("reset --hard: write %s: %w", path, err)
		}
	}
	return nil
}

func pathsOverlap(a, b string) bool {
	return a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}
