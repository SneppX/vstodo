package main

import (
	"fmt"

	"vstodo/internal/app"
	"vstodo/internal/fsck"
)

// cmdFSCK performs a read-only integrity check of all repository objects and refs.
func cmdFSCK(a *app.App, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: vstodo fsck")
	}
	report, err := fsck.Check(a)
	if err != nil {
		return err
	}
	fmt.Printf("vstodo fsck: %d objects verified (%d blobs, %d trees, %d commits, %d tags); %d references checked\n",
		report.Objects, report.Blobs, report.Trees, report.Commits, report.Tags, report.References)
	return nil
}
