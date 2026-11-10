package main

import (
	"fmt"

	"vstodo/internal/app"
	"vstodo/internal/object"
)

// cmdReset resets current HEAD to the specified state.
func cmdReset(a *app.App, args []string) error {
	mode := "mixed"
	target := "HEAD"
	if len(args) >= 1 {
		target = args[0]
	}
	if len(args) >= 2 {
		mode = args[1]
	}
	_ = object.CommitType
	return fmt.Errorf("reset not fully implemented (mode=%s target=%s)", mode, target)
}
