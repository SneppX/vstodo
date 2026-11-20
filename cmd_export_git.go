package main

import (
	"fmt"

	"vstodo/internal/migrate"
)

func cmdExportGit(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: vstodo export-git [branch]")
	}
	branch := ""
	if len(args) == 1 {
		branch = args[0]
	}
	report, err := migrate.ExportGit(".", branch)
	if err != nil {
		return err
	}
	if report.UpToDate {
		fmt.Printf("Git branch %s is already at VSTodo commit %s.\n", report.Branch, report.Commit)
		return nil
	}
	fmt.Printf(
		"Exported VSTodo commit %s to Git branch %s (%d objects written). Git config, index, and working files were not modified.\n",
		report.Commit,
		report.Branch,
		report.ObjectsWritten,
	)
	return nil
}
