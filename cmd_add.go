package main

import (
	"fmt"
	"os"
	"path/filepath"

	"vstodo/internal/app"
	"vstodo/internal/object"
)

// cmdAdd stages file contents into the index.
func cmdAdd(a *app.App, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: vstodo add <file> [file...]")
	}
	for _, rel := range args {
		rel = filepath.ToSlash(rel)
		data, err := os.ReadFile(a.Repo.Path(rel))
		if err != nil {
			return err
		}
		id := object.HashBlob(data)
		if _, err := a.Store.Store(&object.Object{Type: object.BlobType, Data: data}); err != nil {
			return err
		}
		a.Index.Add(rel, id)
	}
	return a.SaveIndex()
}
