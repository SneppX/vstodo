package main

import (
	"fmt"
	"os"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/sha1"
)

// cmdCheckout switches branches or restores files.
func cmdCheckout(a *app.App, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: vstodo checkout <branch|commit>")
	}
	target := args[0]
	var id sha1.Digest
	var err error
	// Try as branch ref.
	id, err = a.Refs.ReadRef("refs/heads/" + target)
	if err != nil {
		// Try as raw hash.
		id, err = sha1.FromHex(target)
		if err != nil {
			return fmt.Errorf("not a branch or object: %s", target)
		}
	}
	obj, err := a.Store.Load(id)
	if err != nil {
		return err
	}
	c, err := commit.Parse(id, obj.Data)
	if err != nil {
		return err
	}
	// Write tree to working tree.
	treeMap, err := treeToMap(a, c.Tree, "")
	if err != nil {
		return err
	}
	for path, bid := range treeMap {
		data, err := readBlob(a, bid)
		if err != nil {
			return err
		}
		if err := writeFile(a, path, data); err != nil {
			return err
		}
	}
	// Update HEAD.
	if _, err := a.Refs.ReadRef("refs/heads/" + target); err == nil {
		if err := a.Refs.SetHead("ref: refs/heads/" + target); err != nil {
			return err
		}
		fmt.Printf("Switched to branch '%s'\n", target)
	} else {
		if err := a.Refs.SetHead(id.Hex()); err != nil {
			return err
		}
		fmt.Printf("Note: detached HEAD at %s\n", id.Hex()[:7])
	}
	_ = os.Stdout
	return nil
}
