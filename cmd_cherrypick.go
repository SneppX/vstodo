package main

import (
	"fmt"
	"strings"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

// cmdCherryPick applies a single-parent commit using the same three-way
// machinery as merge, so conflicts are detected before repository state moves.
func cmdCherryPick(a *app.App, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: vstodo cherry-pick <commit>")
	}
	pickID, err := resolveCommitish(a, args[0])
	if err != nil {
		return err
	}
	pickObj, err := a.Store.Load(pickID)
	if err != nil {
		return err
	}
	if pickObj.Type != object.CommitType {
		return fmt.Errorf("cherry-pick: target is not a commit")
	}
	pickCommit, err := commit.Parse(pickID, pickObj.Data)
	if err != nil {
		return err
	}
	if len(pickCommit.Parents) != 1 {
		return fmt.Errorf("cherry-pick: merge commits and root commits are not supported; target must have exactly one parent")
	}

	headID, err := currentCommitID(a)
	if err != nil || headID == sha1.Zero {
		return fmt.Errorf("cherry-pick: no current commit")
	}
	branch, err := a.Refs.CurrentBranch()
	if err != nil || branch == "" {
		return fmt.Errorf("cherry-pick: cannot apply while HEAD is detached")
	}
	headObj, err := a.Store.Load(headID)
	if err != nil {
		return err
	}
	if headObj.Type != object.CommitType {
		return fmt.Errorf("cherry-pick: current HEAD is not a commit")
	}
	headCommit, err := commit.Parse(headID, headObj.Data)
	if err != nil {
		return err
	}
	if err := requireCleanWorkingTree(a, headCommit.Tree); err != nil {
		return fmt.Errorf("cherry-pick: %w", err)
	}

	parentID := pickCommit.Parents[0]
	parentObj, err := a.Store.Load(parentID)
	if err != nil {
		return err
	}
	if parentObj.Type != object.CommitType {
		return fmt.Errorf("cherry-pick: parent is not a commit")
	}
	parentCommit, err := commit.Parse(parentID, parentObj.Data)
	if err != nil {
		return err
	}
	baseTree, err := treeToMap(a, parentCommit.Tree, "")
	if err != nil {
		return err
	}
	oursTree, err := treeToMap(a, headCommit.Tree, "")
	if err != nil {
		return err
	}
	pickTree, err := treeToMap(a, pickCommit.Tree, "")
	if err != nil {
		return err
	}

	mergedTree, mergedContents, conflicts, err := mergeTrees(a, baseTree, oursTree, pickTree)
	if err != nil {
		return err
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("cherry-pick conflicts in %s; no files or refs were changed", strings.Join(conflicts, ", "))
	}
	for path, data := range mergedContents {
		id, err := a.Store.WriteRaw(object.BlobType, data)
		if err != nil {
			return fmt.Errorf("cherry-pick: store merged blob %s: %w", path, err)
		}
		mergedTree[path] = id
	}
	if sameTreeSnapshot(oursTree, mergedTree) {
		fmt.Println("The cherry-pick is empty; no changes applied.")
		return nil
	}
	treeID, err := buildTreeFromIndex(a, mergedTree, "")
	if err != nil {
		return err
	}
	message := "Cherry-pick: " + pickCommit.Message
	commitData := commit.Build(treeID, []sha1.Digest{headID}, "vstodo", "vstodo", message)
	newID := object.HashCommit(commitData)
	if _, err := a.Store.Store(&object.Object{Type: object.CommitType, Data: commitData}); err != nil {
		return err
	}
	if err := applyMergedTree(a, oursTree, mergedTree); err != nil {
		return err
	}
	if err := a.Refs.WriteRef("refs/heads/"+branch, newID); err != nil {
		return err
	}
	fmt.Printf("[%s %s] %s\n", branch, newID.Hex()[:7], message)
	return nil
}

func sameTreeSnapshot(a, b map[string]sha1.Digest) bool {
	if len(a) != len(b) {
		return false
	}
	for path, id := range a {
		if b[path] != id {
			return false
		}
	}
	return true
}
