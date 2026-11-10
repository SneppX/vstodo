package main

import (
	"fmt"
	"sort"
	"strings"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/index"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

// cmdCherryPick applies the changes introduced by existing commits.
func cmdCherryPick(a *app.App, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: vstodo cherry-pick <commit>")
	}
	id, err := resolveCommitish(a, args[0])
	if err != nil {
		return err
	}

	obj, err := a.Store.Load(id)
	if err != nil {
		return err
	}
	c, err := commit.Parse(id, obj.Data)
	if err != nil {
		return err
	}

	if len(c.Parents) == 0 {
		return fmt.Errorf("cannot cherry-pick root commit %s", id.Hex()[:7])
	}
	parentID := c.Parents[0]

	// Compute the diff between parent and this commit.
	parentTreeID, err := commitTreeID(a, parentID)
	if err != nil {
		return err
	}
	parentTree, err := treeToMap(a, parentTreeID, "")
	if err != nil {
		return err
	}
	commitTree, err := treeToMap(a, c.Tree, "")
	if err != nil {
		return err
	}

	// Apply changes to the working tree and index: for each path in commitTree,
	// write the blob content; for paths removed, delete.
	for path, bid := range commitTree {
		if parentTree[path] != bid {
			if bid == sha1.Zero {
				_ = removeFile(a, path)
				a.Index.Remove(path)
				continue
			}
			data, err := readBlob(a, bid)
			if err != nil {
				return err
			}
			if err := writeFile(a, path, data); err != nil {
				return err
			}
			a.Index.Add(path, bid)
		}
	}
	for path := range parentTree {
		if _, ok := commitTree[path]; !ok {
			_ = removeFile(a, path)
			a.Index.Remove(path)
		}
	}
	if err := a.SaveIndex(); err != nil {
		return err
	}

	// Create a commit with the cherry-picked changes.
	headID, err := currentCommitID(a)
	if err != nil || headID == sha1.Zero {
		headID = sha1.Zero
	}
	parents := []sha1.Digest{}
	if headID != sha1.Zero {
		parents = append(parents, headID)
	}
	newTreeID, err := buildTreeFromIndex(a, a.Index.Entries(), "")
	if err != nil {
		return err
	}
	cb := commit.Build(newTreeID, parents, "vstodo", "vstodo", "Cherry-pick: "+c.Message)
	newID := object.HashCommit(cb)
	if _, err := a.Store.Store(&object.Object{Type: object.CommitType, Data: cb}); err != nil {
		return err
	}
	branch, err := a.Refs.CurrentBranch()
	if err != nil || branch == "" {
		branch = "main"
	}
	if err := a.Refs.WriteRef("refs/heads/"+branch, newID); err != nil {
		return err
	}
	if err := a.Refs.SetHead("ref: refs/heads/" + branch); err != nil {
		return err
	}
	a.Index = index.NewIndex(a.Repo.GitPath("index"))
	if err := a.SaveIndex(); err != nil {
		return err
	}

	fmt.Printf("[%s %s] Cherry-pick: %s\n", branch, newID.Hex()[:7], c.Message)
	return nil
}

func commitTreeID(a *app.App, id sha1.Digest) (sha1.Digest, error) {
	obj, err := a.Store.Load(id)
	if err != nil {
		return sha1.Zero, err
	}
	c, err := commit.Parse(id, obj.Data)
	if err != nil {
		return sha1.Zero, err
	}
	return c.Tree, nil
}

var _ = sort.Strings
var _ = strings.Join
