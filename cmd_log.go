package main

import (
	"fmt"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/sha1"
)

// cmdLog shows commit history (first-parent).
func cmdLog(a *app.App, args []string) error {
	start, err := currentCommitID(a)
	if err != nil || start == sha1.Zero {
		fmt.Println("No commits yet.")
		return nil
	}
	var ids []sha1.Digest
	seen := make(map[sha1.Digest]bool)
	cur := start
	for cur != sha1.Zero && !seen[cur] {
		seen[cur] = true
		ids = append(ids, cur)
		obj, err := a.Store.Load(cur)
		if err != nil {
			return err
		}
		c, err := commit.Parse(cur, obj.Data)
		if err != nil {
			return err
		}
		if len(c.Parents) > 0 {
			cur = c.Parents[0]
		} else {
			cur = sha1.Zero
		}
	}
	for _, id := range ids {
		obj, err := a.Store.Load(id)
		if err != nil {
			return err
		}
		c, err := commit.Parse(id, obj.Data)
		if err != nil {
			return err
		}
		fmt.Printf("commit %s\n", id.Hex())
		fmt.Printf("Author: %s\n", c.Author)
		fmt.Printf("Date:   %d\n", c.CommitterTS)
		fmt.Println()
		fmt.Println("    " + c.Message)
		fmt.Println()
	}
	return nil
}
