package main

import (
	"fmt"

	"vstodo/internal/app"
	"vstodo/internal/sha1"
)

// cmdCatFile provides the content or type of repository objects.
func cmdCatFile(a *app.App, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: vstodo cat-file (-t|-p) <object>")
	}
	opt := args[0]
	id, err := sha1.FromHex(args[1])
	if err != nil {
		return err
	}
	obj, err := a.Store.Load(id)
	if err != nil {
		return err
	}
	switch opt {
	case "-t":
		fmt.Println(obj.Type)
	case "-p":
		fmt.Print(string(obj.Data))
	default:
		return fmt.Errorf("unknown option %s", opt)
	}
	return nil
}
