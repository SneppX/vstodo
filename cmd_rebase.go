package main

import (
	"fmt"
	"strings"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

// cmdRebase replays the current branch's linear commits on top of upstream.
// It computes every replay before changing the worktree or branch ref.
func cmdRebase(a *app.App, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: vstodo rebase <upstream>")
	}
	upstreamID, err := resolveCommitish(a, args[0])
	if err != nil {
		return err
	}
	headID, err := currentCommitID(a)
	if err != nil || headID == sha1.Zero {
		return fmt.Errorf("rebase: no current commit")
	}
	branch, err := a.Refs.CurrentBranch()
	if err != nil || branch == "" {
		return fmt.Errorf("rebase: cannot rebase while HEAD is detached")
	}
	if upstreamID == headID {
		fmt.Println("Current branch is already up to date.")
		return nil
	}
	headObj, err := a.Store.Load(headID)
	if err != nil {
		return err
	}
	if headObj.Type != object.CommitType {
		return fmt.Errorf("rebase: current HEAD is not a commit")
	}
	headCommit, err := commit.Parse(headID, headObj.Data)
	if err != nil {
		return err
	}
	upstreamObj, err := a.Store.Load(upstreamID)
	if err != nil {
		return err
	}
	if upstreamObj.Type != object.CommitType {
		return fmt.Errorf("rebase: upstream is not a commit")
	}
	upstreamCommit, err := commit.Parse(upstreamID, upstreamObj.Data)
	if err != nil {
		return err
	}
	if err := requireCleanWorkingTree(a, headCommit.Tree); err != nil {
		return fmt.Errorf("rebase: %w", err)
	}

	upstreamIsAncestor, err := isCommitAncestor(a, upstreamID, headID)
	if err != nil {
		return err
	}
	if upstreamIsAncestor {
		fmt.Println("Current branch is already based on upstream.")
		return nil
	}
	baseID, err := findMergeBase(a, headID, upstreamID)
	if err != nil {
		return err
	}

	type replayItem struct {
		id     sha1.Digest
		commit *commit.Commit
	}
	var replay []replayItem
	cursor := headID
	for cursor != baseID {
		obj, err := a.Store.Load(cursor)
		if err != nil {
			return err
		}
		if obj.Type != object.CommitType {
			return fmt.Errorf("rebase: object %s is not a commit", cursor.Hex())
		}
		parsed, err := commit.Parse(cursor, obj.Data)
		if err != nil {
			return err
		}
		if len(parsed.Parents) != 1 {
			return fmt.Errorf("rebase: merge commits are not supported; refusing to rewrite history")
		}
		replay = append(replay, replayItem{id: cursor, commit: parsed})
		cursor = parsed.Parents[0]
	}
	for i, j := 0, len(replay)-1; i < j; i, j = i+1, j-1 {
		replay[i], replay[j] = replay[j], replay[i]
	}

	headTree, err := treeToMap(a, headCommit.Tree, "")
	if err != nil {
		return err
	}
	newTree, err := treeToMap(a, upstreamCommit.Tree, "")
	if err != nil {
		return err
	}
	newParentID := upstreamID
	for _, item := range replay {
		oldCommit := item.commit
		parentObj, err := a.Store.Load(oldCommit.Parents[0])
		if err != nil {
			return err
		}
		parentCommit, err := commit.Parse(oldCommit.Parents[0], parentObj.Data)
		if err != nil {
			return err
		}
		oldBaseTree, err := treeToMap(a, parentCommit.Tree, "")
		if err != nil {
			return err
		}
		oldCommitTree, err := treeToMap(a, oldCommit.Tree, "")
		if err != nil {
			return err
		}
		mergedTree, mergedContents, conflicts, err := mergeTrees(a, oldBaseTree, newTree, oldCommitTree)
		if err != nil {
			return err
		}
		if len(conflicts) > 0 {
			return fmt.Errorf("rebase conflicts while replaying %s in %s; no refs or worktree files were changed", item.id.Hex()[:7], strings.Join(conflicts, ", "))
		}
		for path, data := range mergedContents {
			id, err := a.Store.WriteRaw(object.BlobType, data)
			if err != nil {
				return fmt.Errorf("rebase: store merged blob %s: %w", path, err)
			}
			mergedTree[path] = id
		}
		if sameTreeSnapshot(newTree, mergedTree) {
			continue
		}
		treeID, err := buildTreeFromIndex(a, mergedTree, "")
		if err != nil {
			return err
		}
		rebasedData := commit.Build(treeID, []sha1.Digest{newParentID}, oldCommit.Author, "vstodo", oldCommit.Message)
		newID := object.HashCommit(rebasedData)
		if _, err := a.Store.Store(&object.Object{Type: object.CommitType, Data: rebasedData}); err != nil {
			return err
		}
		newParentID = newID
		newTree = mergedTree
	}
	if newParentID == headID {
		fmt.Println("No commits needed to be replayed.")
		return nil
	}
	if err := applyMergedTree(a, headTree, newTree); err != nil {
		return err
	}
	if err := a.Refs.WriteRef("refs/heads/"+branch, newParentID); err != nil {
		return err
	}
	fmt.Printf("Successfully rebased %s onto %s\n", branch, upstreamID.Hex()[:7])
	return nil
}
