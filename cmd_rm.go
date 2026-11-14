package main

import (
	"fmt"
	"path/filepath"

	"vstodo/internal/app"
)

// cmdRm removes files from the working tree and the index.
func cmdRm(a *app.App, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: vstodo rm <file> [file...]")
	}
	if err := ensureIndexFromHEAD(a); err != nil {
		return err
	}
	for _, rel := range args {
		rel = filepath.ToSlash(rel)
		a.Index.Remove(rel)
		if err := removeFile(a, rel); err != nil {
			return err
		}
	}
	return a.SaveIndex()
}
