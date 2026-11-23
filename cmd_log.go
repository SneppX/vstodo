package main

import (
	"fmt"
	"strconv"
	"strings"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/sha1"
)

// cmdLog shows commit history (first-parent).
func cmdLog(a *app.App, args []string) error {
	limit := -1
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-n":
			if i+1 >= len(args) {
				return fmt.Errorf("usage: vstodo log [-n <count>]")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 0 {
				return fmt.Errorf("log: invalid -n value: %s", args[i])
			}
			limit = n
		case strings.HasPrefix(arg, "-n") && len(arg) > 2:
			n, err := strconv.Atoi(arg[2:])
			if err != nil || n < 0 {
				return fmt.Errorf("log: invalid -n value: %s", arg[2:])
			}
			limit = n
		case strings.HasPrefix(arg, "--max-count="):
			v := strings.TrimPrefix(arg, "--max-count=")
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return fmt.Errorf("log: invalid --max-count value: %s", v)
			}
			limit = n
		default:
			return fmt.Errorf("usage: vstodo log [-n <count>]")
		}
	}

	start, err := currentCommitID(a)
	if err != nil || start == sha1.Zero {
		fmt.Println("No commits yet.")
		return nil
	}
	var ids []sha1.Digest
	seen := make(map[sha1.Digest]bool)
	cur := start
	for cur != sha1.Zero && !seen[cur] {
		if limit >= 0 && len(ids) >= limit {
			break
		}
		seen[cur] = true
		ids = append(ids, cur)
		obj, err := a.Store.Load(cur)
		if err != nil {
			return err
		}
		c, err := commit.Parse(cur, obj.Data)
		if err != nil {
			return err
		}
		if len(c.Parents) > 0 {
			cur = c.Parents[0]
		} else {
			cur = sha1.Zero
		}
	}
	for _, id := range ids {
		obj, err := a.Store.Load(id)
		if err != nil {
			return err
		}
		c, err := commit.Parse(id, obj.Data)
		if err != nil {
			return err
		}
		fmt.Printf("commit %s\n", id.Hex())
		fmt.Printf("Author: %s\n", c.Author)
		fmt.Printf("Date:   %d\n", c.CommitterTS)
		fmt.Println()
		fmt.Println("    " + c.Message)
		fmt.Println()
	}
	return nil
}
