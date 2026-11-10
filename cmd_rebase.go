package main

import (
	"fmt"

	"vstodo/internal/app"
)

// cmdRebase reapplies commits on top of another base tip.
func cmdRebase(a *app.App, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: vstodo rebase <base>")
	}
	return fmt.Errorf("rebase onto '%s' not yet implemented", args[0])
}
