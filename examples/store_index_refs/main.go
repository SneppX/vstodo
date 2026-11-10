// Package main demonstrates vstodo storage, index, and ref manipulation:
// creating a store, writing objects, staging entries in the index, and
// managing HEAD/branch/tag refs.
//
// Run:
//
//	go run ./examples/store_index_refs.go
package main

import (
	"fmt"
	"os"

	"vstodo/internal/app"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

func main() {
	dir, err := os.MkdirTemp("", "vstodo-store-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	// app.Init creates repo, store, index, and refs wired together.
	a, err := app.Init(dir)
	if err != nil {
		panic(err)
	}

	// --- Storage ---
	blobID := object.HashBlob([]byte("stored content\n"))
	if _, err := a.Store.Store(&object.Object{Type: object.BlobType, Data: []byte("stored content\n")}); err != nil {
		panic(err)
	}
	obj, err := a.Store.Load(blobID)
	if err != nil {
		panic(err)
	}
	fmt.Printf("store: loaded %s (%d bytes), has=%t\n", obj.ID.Hex(), len(obj.Data), true)

	// --- Index ---
	a.Index.Add("file.txt", blobID)
	a.Index.Add("other.txt", sha1.Zero) // placeholder digest
	if err := a.SaveIndex(); err != nil {
		panic(err)
	}
	fmt.Printf("index: staged %d entries\n", len(a.Index.Entries()))
	fmt.Printf("index: contains file.txt=%t, Get=%s\n",
		a.Index.Contains("file.txt"), a.Index.Get("file.txt").Hex())

	// Unstage other.txt.
	a.Index.Remove("other.txt")
	if err := a.SaveIndex(); err != nil {
		panic(err)
	}
	fmt.Printf("index: after remove, %d entries\n", len(a.Index.Entries()))

	// --- Refs ---
	head, err := a.Refs.Head()
	fmt.Printf("refs: HEAD initial=%s (err=%v)\n", head.Hex(), err)

	// Create a branch pointing at HEAD (Zero here since no commits yet).
	if err := a.Refs.WriteRef("refs/heads/dev", sha1.Zero); err != nil {
		panic(err)
	}
	branch, err := a.Refs.ReadRef("refs/heads/dev")
	fmt.Printf("refs: refs/heads/dev=%s (err=%v)\n", branch.Hex(), err)

	// List all refs.
	refs, err := a.Refs.ListRefs()
	if err != nil {
		panic(err)
	}
	fmt.Printf("refs: listed %d refs:\n", len(refs))
	for name, id := range refs {
		fmt.Printf("  %-20s %s\n", name, id.Hex())
	}

	// Delete the demo branch.
	if err := a.Refs.DeleteRef("refs/heads/dev"); err != nil {
		panic(err)
	}
	fmt.Println("refs: deleted refs/heads/demo")

	fmt.Println("done")
}
