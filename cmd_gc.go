package main

import (
	"fmt"

	"vstodo/internal/app"
	"vstodo/internal/gc"
)

// cmdGC performs a safe loose-object garbage-collection pass.
func cmdGC(a *app.App, args []string) error {
	dryRun := true
	for _, arg := range args {
		switch arg {
		case "--dry-run":
			dryRun = true
		case "--prune":
			dryRun = false
		default:
			return fmt.Errorf("usage: vstodo gc [--dry-run|--prune]")
		}
	}
	return gc.GCWithOptions(a, dryRun)
}
