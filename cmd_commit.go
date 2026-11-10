package main

import (
	"fmt"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/index"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

// cmdCommit records staged changes to the repository.
func cmdCommit(a *app.App, args []string) error {
	msg := ""
	for i, arg := range args {
		if arg == "-m" && i+1 < len(args) {
			msg = args[i+1]
			break
		}
	}
	if msg == "" {
		return fmt.Errorf("vstodo: commit requires -m <message>")
	}

	treeID, err := buildTreeFromIndex(a, a.Index.Entries(), "")
	if err != nil {
		return err
	}

	var parents []sha1.Digest
	headID, err := currentCommitID(a)
	if err == nil && headID != sha1.Zero {
		parents = append(parents, headID)
	}

	cb := commit.Build(treeID, parents, "vstodo", "vstodo", msg)
	commitID := object.HashCommit(cb)
	if _, err := a.Store.Store(&object.Object{Type: object.CommitType, Data: cb}); err != nil {
		return err
	}

	branch, err := a.Refs.CurrentBranch()
	if err != nil || branch == "" {
		branch = "main"
	}
	if err := a.Refs.WriteRef("refs/heads/"+branch, commitID); err != nil {
		return err
	}
	if err := a.Refs.SetHead("ref: refs/heads/" + branch); err != nil {
		return err
	}

	a.Index = index.NewIndex(a.Repo.GitPath("index"))
	if err := a.SaveIndex(); err != nil {
		return err
	}

	fmt.Printf("[%s %s] %s\n", branch, commitID.Hex()[:7], msg)
	return nil
}
