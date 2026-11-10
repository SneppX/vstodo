package main

import (
	"fmt"

	"vstodo/internal/app"
	"vstodo/internal/transport"
)

// cmdClone clones a repository from src to dst.
func cmdClone(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: vstodo clone <src> <dst>")
	}
	src, dst := args[0], args[1]

	srcApp, err := app.New(src)
	if err != nil {
		return fmt.Errorf("open src: %w", err)
	}

	dstApp, err := app.Init(dst)
	if err != nil {
		return fmt.Errorf("init dst: %w", err)
	}

	t := transport.NewLocalTransport()
	if err := t.Clone(srcApp, dstApp); err != nil {
		return fmt.Errorf("clone: %w", err)
	}

	fmt.Printf("Cloned '%s' into '%s'\n", src, dst)
	return nil
}
