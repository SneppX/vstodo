package main

import (
	"fmt"
	"path/filepath"

	"vstodo/internal/migrate"
)

func cmdAdopt(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: vstodo adopt [path]")
	}
	root := "."
	if len(args) == 1 {
		root = args[0]
	}
	report, err := migrate.Adopt(root)
	if err != nil {
		return err
	}
	fmt.Printf("Adopted Git repository: %d loose objects, %d packed objects, %d refs, %d remotes\n",
		report.LooseObjects, report.PackedObjects, report.References, report.Remotes)
	if report.Branch != "" {
		fmt.Printf("Current branch: %s\n", report.Branch)
	}
	fmt.Printf("Git config and index snapshots are preserved under %s/migration/.\n", filepath.ToSlash(report.MetadataDir))
	if report.GitExcludeUpdated {
		fmt.Printf("Added /%s/ to .git/info/exclude; existing ignore rules were preserved.\n", filepath.ToSlash(report.MetadataDir))
	} else {
		fmt.Printf("The /%s/ Git exclude rule was already present.\n", filepath.ToSlash(report.MetadataDir))
	}
	fmt.Println("Git objects, refs, config, index, remotes, and working files remain intact.")
	return nil
}
