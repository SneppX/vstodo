package main

import (
	"fmt"
	"os"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

// cmdStatus shows the working tree status.
func cmdStatus(a *app.App, args []string) error {
	branch, err := a.Refs.CurrentBranch()
	if err != nil || branch == "" {
		branch = "(detached)"
	}
	fmt.Printf("On branch %s\n", branch)

	headID, err := currentCommitID(a)
	if err != nil || headID == sha1.Zero {
		fmt.Println("No commits yet")
	} else {
		obj, err := a.Store.Load(headID)
		if err != nil {
			return err
		}
		c, err := commit.Parse(headID, obj.Data)
		if err != nil {
			return err
		}
		treeMap, err := treeToMap(a, c.Tree, "")
		if err != nil {
			return err
		}

		indexEntries := a.Index.Entries()
		var staged []string
		for path, id := range indexEntries {
			if headID == sha1.Zero || treeMap[path] != id {
				staged = append(staged, path)
			}
		}
		if len(staged) > 0 {
			fmt.Println("Changes to be committed:")
			for _, p := range staged {
				fmt.Printf("  new file:   %s\n", p)
			}
		}

		files, err := walkFiles(a.Repo.Root)
		if err != nil {
			return err
		}
		var modified []string
		var untracked []string
		for _, f := range files {
			stagedID := stagedDigest(a, f)
			data, err := os.ReadFile(a.Repo.Path(f))
			if err != nil {
				continue
			}
			id := object.HashBlob(data)
			if stagedID == sha1.Zero {
				untracked = append(untracked, f)
			} else if id != stagedID {
				modified = append(modified, f)
			}
		}
		if len(modified) > 0 {
			fmt.Println("Changes not staged for commit:")
			for _, p := range modified {
				fmt.Printf("  modified:   %s\n", p)
			}
		}
		if len(untracked) > 0 {
			fmt.Println("Untracked files:")
			for _, p := range untracked {
				fmt.Printf("  %s\n", p)
			}
		}
	}
	return nil
}
