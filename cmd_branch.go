package main

import (
	"fmt"

	"vstodo/internal/app"
	"vstodo/internal/sha1"
)

// cmdBranch lists, creates, or deletes branches.
func cmdBranch(a *app.App, args []string) error {
	if len(args) == 0 {
		refs, err := a.Refs.ListRefs()
		if err != nil {
			return err
		}
		current, _ := a.Refs.CurrentBranch()
		for name, id := range refs {
			// only heads
			if len(name) < 12 || name[:11] != "refs/heads/" {
				continue
			}
			short := name[11:]
			mark := " "
			if short == current {
				mark = "*"
			}
			fmt.Printf("%s%s\t%s\n", mark, short, id.Hex()[:7])
		}
		return nil
	}
	name := args[0]
	headID, err := currentCommitID(a)
	if err != nil || headID == sha1.Zero {
		return fmt.Errorf("no commits yet; cannot create branch")
	}
	if err := a.Refs.WriteRef("refs/heads/"+name, headID); err != nil {
		return err
	}
	fmt.Printf("Branch '%s' created\n", name)
	return nil
}
