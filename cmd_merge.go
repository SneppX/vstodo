package main

import (
	"fmt"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

// cmdMerge merges another branch into the current branch.
func cmdMerge(a *app.App, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: vstodo merge <branch>")
	}
	branch := args[0]
	headID, err := currentCommitID(a)
	if err != nil || headID == sha1.Zero {
		return fmt.Errorf("nothing to merge into")
	}
	otherID, err := a.Refs.ReadRef("refs/heads/" + branch)
	if err != nil {
		return fmt.Errorf("no such branch: %s", branch)
	}
	if otherID == headID {
		fmt.Println("Already up to date.")
		return nil
	}
	headObj, err := a.Store.Load(headID)
	if err != nil {
		return err
	}
	headC, err := commit.Parse(headID, headObj.Data)
	if err != nil {
		return err
	}
	cb := commit.Build(headC.Tree, []sha1.Digest{headID, otherID}, "vstodo", "vstodo", "Merge branch '"+branch+"'")
	mergeID := object.HashCommit(cb)
	if _, err := a.Store.Store(&object.Object{Type: object.CommitType, Data: cb}); err != nil {
		return err
	}
	cur, _ := a.Refs.CurrentBranch()
	if err := a.Refs.WriteRef("refs/heads/"+cur, mergeID); err != nil {
		return err
	}
	if err := a.Refs.SetHead("ref: refs/heads/" + cur); err != nil {
		return err
	}
	fmt.Printf("Merged %s into %s\n", branch, cur)
	return nil
}
