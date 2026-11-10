package main

import (
	"fmt"
	"os"

	"vstodo/internal/app"
	"vstodo/internal/object"
)

// cmdHashObject computes the object ID for a file (git byte-compatible).
func cmdHashObject(a *app.App, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: vstodo hash-object <file>")
	}
	data, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	id := object.HashBlob(data)
	fmt.Println(id.Hex())
	return nil
}
