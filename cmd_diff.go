package main

import (
	"fmt"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/sha1"
)

// cmdDiff shows changes between two commits or index vs HEAD.
func cmdDiff(a *app.App, args []string) error {
	if len(args) == 0 {
		headID, err := currentCommitID(a)
		if err != nil || headID == sha1.Zero {
			fmt.Println("No commits yet.")
			return nil
		}
		obj, err := a.Store.Load(headID)
		if err != nil {
			return err
		}
		c, err := commit.Parse(headID, obj.Data)
		if err != nil {
			return err
		}
		headMap, err := treeToMap(a, c.Tree, "")
		if err != nil {
			return err
		}
		indexMap := a.Index.Entries()
		for path, id := range indexMap {
			if headMap[path] != id {
				fmt.Printf("diff --git a/%s b/%s\n", path, path)
				fmt.Printf("index %s..%s\n", headMap[path].Hex()[:7], id.Hex()[:7])
			}
		}
		for path := range headMap {
			if _, ok := indexMap[path]; !ok {
				fmt.Printf("diff --deleted a/%s\n", path)
			}
		}
		return nil
	}
	lid, err := sha1.FromHex(args[0])
	if err != nil {
		return err
	}
	rid, err := sha1.FromHex(args[1])
	if err != nil {
		return err
	}
	lo, err := a.Store.Load(lid)
	if err != nil {
		return err
	}
	ro, err := a.Store.Load(rid)
	if err != nil {
		return err
	}
	lc, err := commit.Parse(lid, lo.Data)
	if err != nil {
		return err
	}
	rc, err := commit.Parse(rid, ro.Data)
	if err != nil {
		return err
	}
	lm, err := treeToMap(a, lc.Tree, "")
	if err != nil {
		return err
	}
	rm, err := treeToMap(a, rc.Tree, "")
	if err != nil {
		return err
	}
	for path, id := range rm {
		if lm[path] != id {
			fmt.Printf("diff --git a/%s b/%s\n", path, path)
			fmt.Printf("%s -> %s\n", lm[path].Hex()[:7], id.Hex()[:7])
		}
	}
	for path := range lm {
		if _, ok := rm[path]; !ok {
			fmt.Printf("diff --deleted a/%s\n", path)
		}
	}
	return nil
}
