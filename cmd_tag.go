package main

import (
	"fmt"

	"vstodo/internal/app"
)

// cmdTag creates, lists, or deletes tags.
func cmdTag(a *app.App, args []string) error {
	if len(args) == 0 {
		refs, err := a.Refs.ListRefs()
		if err != nil {
			return err
		}
		for name, id := range refs {
			if len(name) < 10 || name[:9] != "refs/tags" {
				continue
			}
			fmt.Printf("%s\t%s\n", name[10:], id.Hex()[:7])
		}
		return nil
	}
	name := args[0]
	_ = name
	return fmt.Errorf("tag not fully implemented")
}
