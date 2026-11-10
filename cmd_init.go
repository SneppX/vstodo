package main

import (
	"fmt"

	"vstodo/internal/app"
)

// cmdInit initializes a new vstodo repository.
func cmdInit(args []string) error {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}
	if _, err := app.Init(root); err != nil {
		return err
	}
	fmt.Printf("Initialized empty vstodo repository in %s/.vstodo/\n", root)
	return nil
}
