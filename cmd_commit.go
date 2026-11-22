package main

import (
	"errors"
	"fmt"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/ref"
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

	branch, err := a.Refs.CurrentBranch()
	if err != nil {
		return fmt.Errorf("commit: resolve current branch: %w", err)
	}

	refName := "HEAD"
	var expected *sha1.Digest
	var parents []sha1.Digest
	if branch != "" {
		refName = "refs/heads/" + branch
		current, readErr := a.Refs.ReadRef(refName)
		if readErr == nil {
			expected = &current
			parents = append(parents, current)
		} else if !errors.Is(readErr, ref.ErrNotFound) {
			return fmt.Errorf("commit: read branch tip: %w", readErr)
		}
	} else {
		current, readErr := currentCommitID(a)
		if readErr != nil {
			return fmt.Errorf("commit: resolve detached HEAD: %w", readErr)
		}
		expected = &current
		parents = append(parents, current)
	}

	cb := commit.Build(treeID, parents, commitIdentity(a), commitIdentity(a), msg)
	commitID := object.HashCommit(cb)
	if _, err := a.Store.Store(&object.Object{Type: object.CommitType, Data: cb}); err != nil {
		return err
	}

	// Persist the staged snapshot before publishing the new commit tip. If
	// persistence fails, the visible branch or detached HEAD remains unchanged.
	if err := a.SaveIndex(); err != nil {
		return err
	}
	if err := a.Refs.UpdateRef(refName, expected, commitID); err != nil {
		return fmt.Errorf("commit: publish new tip: %w", err)
	}

	label := branch
	if label == "" {
		label = "detached"
	}
	fmt.Printf("[%s %s] %s\n", label, commitID.Hex()[:7], msg)
	return nil
}
