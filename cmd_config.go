package main

import (
	"fmt"
	"sort"

	"vstodo/internal/app"
)

func cmdConfig(a *app.App, args []string) error {
	cfg, err := repositoryConfig(a)
	if err != nil { return err }
	if len(args) == 0 || len(args) == 1 && (args[0] == "--list" || args[0] == "-l") {
		values := cfg.All()
		keys := make([]string, 0, len(values))
		for key := range values { keys = append(keys, key) }
		sort.Strings(keys)
		for _, key := range keys { fmt.Printf("%s=%s\n", key, values[key]) }
		return nil
	}
	switch args[0] {
	case "--get", "-get":
		if len(args) != 2 { return fmt.Errorf("usage: vstodo config --get <key>") }
		value := cfg.Get(args[1])
		if value == "" { return fmt.Errorf("config: key %q not found", args[1]) }
		fmt.Println(value)
		return nil
	case "--unset", "-unset":
		if len(args) != 2 { return fmt.Errorf("usage: vstodo config --unset <key>") }
		if err := cfg.Unset(args[1]); err != nil { return err }
		return cfg.Save()
	case "--global":
		return fmt.Errorf("config: global configuration is not supported yet; use repository-local settings")
	default:
		if len(args) != 2 { return fmt.Errorf("usage: vstodo config <key> <value>") }
		if err := cfg.Set(args[0], args[1]); err != nil { return err }
		return cfg.Save()
	}
}
