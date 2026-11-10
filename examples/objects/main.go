// Package main demonstrates low-level vstodo object operations:
// blobs, trees, commits, and annotated tags, using the library directly.
//
// Run:
//
//	go run ./examples/objects
package main

import (
	"fmt"
	"os"

	"vstodo/internal/object"
	"vstodo/internal/sha1"
	"vstodo/internal/storage"
	"vstodo/internal/tree"
)

func main() {
	dir, err := os.MkdirTemp("", "vstodo-objects-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	store := storage.NewLooseStore(joinPath(dir, ".vstodo", "objects"))

	// --- Blob ---
	blobData := []byte("file contents here\n")
	blobID := object.HashBlob(blobData)
	if _, err := store.Store(&object.Object{Type: object.BlobType, Data: blobData}); err != nil {
		panic(err)
	}
	fmt.Printf("blob   %s  (%d bytes)\n", blobID.Hex(), len(blobData))

	// --- Tree ---
	emptyTree := &tree.Tree{Entries: nil}
	emptyTreeBytes := emptyTree.Marshal()
	emptyTreeID, err := store.Store(&object.Object{Type: object.TreeType, Data: emptyTreeBytes})
	if err != nil {
		panic(err)
	}
	t := &tree.Tree{
		Entries: []tree.Entry{
			{Mode: "100644", Name: "hello.txt", ID: blobID},
			{Mode: "040000", Name: "subdir", ID: emptyTreeID},
		},
	}
	treeBytes := t.Marshal()
	treeID := object.HashTree(treeBytes)
	if _, err := store.Store(&object.Object{Type: object.TreeType, Data: treeBytes}); err != nil {
		panic(err)
	}
	fmt.Printf("tree   %s  (%d entries)\n", treeID.Hex(), len(t.Entries))

	// --- Commit ---
	commitBytes := []byte("tree " + treeID.Hex() + "\n" +
		"author vstodo <vstodo@vstodo> 1234567890 +0000\n" +
		"committer vstodo <vstodo@vstodo> 1234567890 +0000\n" +
		"\n" +
		"example commit\n")
	commitID := object.HashCommit(commitBytes)
	if _, err := store.Store(&object.Object{Type: object.CommitType, Data: commitBytes}); err != nil {
		panic(err)
	}
	fmt.Printf("commit %s\n", commitID.Hex())

	// --- Tag ---
	tagBytes := []byte("object " + commitID.Hex() + "\n" +
		"type commit\n" +
		"tagger vstodo <vstodo@vstodo> 1234567890 +0000\n" +
		"\n" +
		"example tag\n")
	tagID, err := store.Store(&object.Object{Type: object.TagType, Data: tagBytes})
	if err != nil {
		panic(err)
	}
	fmt.Printf("tag    %s\n", tagID.Hex())

	// --- Verify round-trip: load every object back ---
	for _, id := range []sha1.Digest{blobID, treeID, commitID, tagID} {
		obj, err := store.Load(id)
		if err != nil {
			panic(err)
		}
		recomputed := object.ComputeID(obj.Type, obj.Data)
		if recomputed != id {
			panic("content-address mismatch")
		}
		ok, err := store.Has(id)
		if err != nil || !ok {
			panic("store.Has failed")
		}
		fmt.Printf("verified %s type=%s size=%d\n", id.Hex(), obj.Type, len(obj.Data))
	}
}

func joinPath(parts ...string) string {
	result := parts[0]
	for _, p := range parts[1:] {
		result += "/" + p
	}
	return result
}
