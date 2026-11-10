package main

import (
	"fmt"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
	"vstodo/internal/tree"
)

// cmdShow shows a commit, tag, or tree object.
func cmdShow(a *app.App, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: vstodo show <object>")
	}
	id, err := sha1.FromHex(args[0])
	if err != nil {
		return err
	}
	obj, err := a.Store.Load(id)
	if err != nil {
		return err
	}
	switch obj.Type {
	case object.CommitType:
		c, err := commit.Parse(id, obj.Data)
		if err != nil {
			return err
		}
		fmt.Printf("commit %s\n", id.Hex())
		fmt.Printf("Author: %s\n", c.Author)
		fmt.Printf("Committer: %s\n", c.Committer)
		fmt.Println()
		fmt.Println(c.Message)
		treeMap, err := treeToMap(a, c.Tree, "")
		if err != nil {
			return err
		}
		for path, bid := range treeMap {
			fmt.Printf("%s\t%s (blob)\n", bid.Hex()[:7], path)
		}
	case object.TreeType:
		t, err := tree.Parse(id, obj.Data)
		if err != nil {
			return err
		}
		for _, e := range t.Entries {
			fmt.Printf("%s %s\t%s\n", e.Mode, e.ID.Hex()[:7], e.Name)
		}
	case object.BlobType:
		fmt.Print(string(obj.Data))
	default:
		fmt.Printf("type: %s\n", obj.Type)
	}
	return nil
}
