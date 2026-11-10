package main

import (
	"fmt"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
	"vstodo/internal/tree"
)

// cmdNotes adds or inspects object notes.
func cmdNotes(a *app.App, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: vstodo notes <add|show|list|remove> [object] [note]")
	}
	switch args[0] {
	case "add":
		return notesAdd(a, args[1:])
	case "show":
		return notesShow(a, args[1:])
	case "list":
		return notesList(a)
	case "remove":
		return notesRemove(a, args[1:])
	default:
		return fmt.Errorf("vstodo: unknown notes subcommand '%s'", args[0])
	}
}

const notesRef = "refs/notes/commits"

// notesTreeID returns the tree id of the notes tree (creating empty if absent).
func notesTreeID(a *app.App) (sha1.Digest, error) {
	if d, err := a.Refs.ReadRef(notesRef); err == nil {
		obj, err := a.Store.Load(d)
		if err != nil {
			return sha1.Zero, err
		}
		if obj.Type == object.TreeType {
			return d, nil
		}
		if obj.Type == object.CommitType {
			c, err := commit.Parse(d, obj.Data)
			if err != nil {
				return sha1.Zero, err
			}
			return c.Tree, nil
		}
	}
	// Empty tree.
	empty := &tree.Tree{}
	data := empty.Marshal()
	id := object.HashTree(data)
	if _, err := a.Store.Store(&object.Object{Type: object.TreeType, Data: data}); err != nil {
		return sha1.Zero, err
	}
	return id, nil
}

func notesAdd(a *app.App, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: vstodo notes add <object> <note>")
	}
	objID, err := resolveCommitish(a, args[0])
	if err != nil {
		return err
	}
	note := args[1]

	// Ensure object exists.
	if ok, err := a.Store.Has(objID); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("object %s not found", objID.Hex()[:7])
	}

	// Notes tree maps object hex -> note blob.
	treeID, err := notesTreeID(a)
	if err != nil {
		return err
	}
	obj, err := a.Store.Load(treeID)
	if err != nil {
		return err
	}
	t, err := tree.Parse(treeID, obj.Data)
	if err != nil {
		return err
	}
	entries := make([]tree.Entry, len(t.Entries))
	copy(entries, t.Entries)

	noteID := object.HashBlob([]byte(note))
	if _, err := a.Store.Store(&object.Object{Type: object.BlobType, Data: []byte(note)}); err != nil {
		return err
	}
	found := false
	for i := range entries {
		if entries[i].Name == objID.Hex() {
			entries[i].ID = noteID
			entries[i].Mode = "100644"
			found = true
			break
		}
	}
	if !found {
		entries = append(entries, tree.Entry{Mode: "100644", Name: objID.Hex(), ID: noteID})
	}

	data := tree.Build(entries)
	newTreeID := object.HashTree(data)
	if _, err := a.Store.Store(&object.Object{Type: object.TreeType, Data: data}); err != nil {
		return err
	}

	// Commit the new notes tree (notes stored as commit pointing to tree, refs/notes).
	head, _ := currentCommitID(a)
	parents := []sha1.Digest{}
	if head != sha1.Zero {
		parents = append(parents, head)
	}
	cb := commit.Build(newTreeID, parents, "vstodo", "vstodo", "notes: add "+objID.Hex()[:7])
	commitID := object.HashCommit(cb)
	if _, err := a.Store.Store(&object.Object{Type: object.CommitType, Data: cb}); err != nil {
		return err
	}
	if err := a.Refs.WriteRef(notesRef, commitID); err != nil {
		return err
	}
	fmt.Printf("Added note for %s\n", objID.Hex()[:7])
	return nil
}

func notesShow(a *app.App, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: vstodo notes show <object>")
	}
	objID, err := resolveCommitish(a, args[0])
	if err != nil {
		return err
	}
	treeID, err := notesTreeID(a)
	if err != nil {
		return err
	}
	obj, err := a.Store.Load(treeID)
	if err != nil {
		return err
	}
	t, err := tree.Parse(treeID, obj.Data)
	if err != nil {
		return err
	}
	for _, e := range t.Entries {
		if e.Name == objID.Hex() {
			data, err := readBlob(a, e.ID)
			if err != nil {
				return err
			}
			fmt.Print(string(data))
			return nil
		}
	}
	return fmt.Errorf("no note for %s", objID.Hex()[:7])
}

func notesList(a *app.App) error {
	treeID, err := notesTreeID(a)
	if err != nil {
		return err
	}
	obj, err := a.Store.Load(treeID)
	if err != nil {
		return err
	}
	t, err := tree.Parse(treeID, obj.Data)
	if err != nil {
		return err
	}
	for _, e := range t.Entries {
		fmt.Printf("%s\t%s\n", e.Name[:7], e.ID.Hex()[:7])
	}
	return nil
}

func notesRemove(a *app.App, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: vstodo notes remove <object>")
	}
	objID, err := resolveCommitish(a, args[0])
	if err != nil {
		return err
	}
	treeID, err := notesTreeID(a)
	if err != nil {
		return err
	}
	obj, err := a.Store.Load(treeID)
	if err != nil {
		return err
	}
	t, err := tree.Parse(treeID, obj.Data)
	if err != nil {
		return err
	}
	entries := make([]tree.Entry, 0, len(t.Entries))
	found := false
	for _, e := range t.Entries {
		if e.Name == objID.Hex() {
			found = true
			continue
		}
		entries = append(entries, e)
	}
	if !found {
		return fmt.Errorf("no note for %s", objID.Hex()[:7])
	}
	data := tree.Build(entries)
	newTreeID := object.HashTree(data)
	if _, err := a.Store.Store(&object.Object{Type: object.TreeType, Data: data}); err != nil {
		return err
	}
	head, _ := currentCommitID(a)
	parents := []sha1.Digest{}
	if head != sha1.Zero {
		parents = append(parents, head)
	}
	cb := commit.Build(newTreeID, parents, "vstodo", "vstodo", "notes: remove "+objID.Hex()[:7])
	commitID := object.HashCommit(cb)
	if _, err := a.Store.Store(&object.Object{Type: object.CommitType, Data: cb}); err != nil {
		return err
	}
	if err := a.Refs.WriteRef(notesRef, commitID); err != nil {
		return err
	}
	fmt.Printf("Removed note for %s\n", objID.Hex()[:7])
	return nil
}
