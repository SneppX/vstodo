// Package main demonstrates embedding vstodo as a library.
// It creates a repository, adds files, commits, and reads history
// entirely programmatically -- no CLI subprocess required.
//
// Run:
//
//	go run ./examples/embedded_vcs.go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/index"
	"vstodo/internal/object"
	"vstodo/internal/repo"
	"vstodo/internal/sha1"
	"vstodo/internal/tree"
)

func main() {
	dir, err := os.MkdirTemp("", "vstodo-embedded-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	a, err := app.Init(dir)
	if err != nil {
		panic(err)
	}

	// Write some working-tree files.
	writeFile(a.Repo, "hello.txt", []byte("hello, vstodo\n"))
	writeFile(a.Repo, "notes.txt", []byte("some notes\n"))

	// Stage them: hash content as blobs, store blobs, update index.
	for _, rel := range []string{"hello.txt", "notes.txt"} {
		data, err := os.ReadFile(a.Repo.Path(rel))
		if err != nil {
			panic(err)
		}
		id := object.HashBlob(data)
		if _, err := a.Store.Store(&object.Object{Type: object.BlobType, Data: data}); err != nil {
			panic(err)
		}
		a.Index.Add(rel, id)
	}
	if err := a.SaveIndex(); err != nil {
		panic(err)
	}

	// Build a tree from the index and commit it.
	treeID, err := buildTreeFromIndex(a, a.Index.Entries(), "")
	if err != nil {
		panic(err)
	}
	var parents []sha1.Digest
	head, err := a.Refs.Head()
	if err == nil && head != sha1.Zero {
		parents = append(parents, head)
	}
	cb := commit.Build(treeID, parents, "vstodo", "vstodo", "Initial commit")
	commitID := object.HashCommit(cb)
	if _, err := a.Store.Store(&object.Object{Type: object.CommitType, Data: cb}); err != nil {
		panic(err)
	}
	if err := a.Refs.WriteRef("refs/heads/main", commitID); err != nil {
		panic(err)
	}
	if err := a.Refs.SetHead("ref: refs/heads/main"); err != nil {
		panic(err)
	}
	a.Index = index.NewIndex(a.Repo.GitPath("index"))
	if err := a.SaveIndex(); err != nil {
		panic(err)
	}

	fmt.Printf("committed %s\n", commitID.Hex())

	// Read history back programmatically.
	start, err := a.Refs.Head()
	if err != nil {
		panic(err)
	}
	printHistory(a, start)

	// Verify the blob is content-addressed and retrievable.
	obj, err := a.Store.Load(object.HashBlob([]byte("hello, vstodo\n")))
	if err != nil {
		panic(err)
	}
	if string(obj.Data) != "hello, vstodo\n" {
		panic("blob content mismatch")
	}
	fmt.Printf("blob %s verified (%d bytes)\n", obj.ID.Hex(), len(obj.Data))
}

func writeFile(r *repo.Repository, rel string, data []byte) {
	full := r.Path(rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(full, data, 0o644); err != nil {
		panic(err)
	}
}

func buildTreeFromIndex(a *app.App, entries map[string]sha1.Digest, prefix string) (sha1.Digest, error) {
	type dirEntry struct {
		mode string
		name string
		id   sha1.Digest
	}
	subdirs := make(map[string]map[string]sha1.Digest)
	var tops []dirEntry
	for path, id := range entries {
		rel := strings.TrimPrefix(path, prefix)
		rel = strings.TrimLeft(rel, "/")
		parts := strings.SplitN(rel, string(filepath.Separator), 2)
		name := parts[0]
		if len(parts) == 1 {
			tops = append(tops, dirEntry{mode: "100644", name: name, id: id})
		} else {
			sub := parts[0]
			if subdirs[sub] == nil {
				subdirs[sub] = make(map[string]sha1.Digest)
			}
			subdirs[sub][prefix+rel] = id
		}
	}
	var te []tree.Entry
	for _, t := range tops {
		te = append(te, tree.Entry{Mode: t.mode, Name: t.name, ID: t.id})
	}
	for sub, subents := range subdirs {
		subID, err := buildTreeFromIndex(a, subents, prefix+sub+string(filepath.Separator))
		if err != nil {
			return sha1.Zero, err
		}
		te = append(te, tree.Entry{Mode: "040000", Name: sub, ID: subID})
	}
	t := &tree.Tree{Entries: te}
	data := t.Marshal()
	if _, err := a.Store.Store(&object.Object{Type: object.TreeType, Data: data}); err != nil {
		return sha1.Zero, err
	}
	return object.HashTree(data), nil
}

func printHistory(a *app.App, start sha1.Digest) {
	seen := make(map[sha1.Digest]bool)
	cur := start
	for cur != sha1.Zero && !seen[cur] {
		seen[cur] = true
		obj, err := a.Store.Load(cur)
		if err != nil {
			panic(err)
		}
		c, err := commit.Parse(cur, obj.Data)
		if err != nil {
			panic(err)
		}
		fmt.Printf("commit %s\n%s\n    %s\n\n", cur.Hex(), c.Author, c.Message)
		if len(c.Parents) > 0 {
			cur = c.Parents[0]
		} else {
			cur = sha1.Zero
		}
	}
}
