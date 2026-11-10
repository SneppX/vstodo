package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

// cmdStash stashes changes in a dirty working tree away.
func cmdStash(a *app.App, args []string) error {
	if len(args) == 0 {
		args = []string{"push"}
	}
	switch args[0] {
	case "push":
		return stashPush(a, args[1:])
	case "pop":
		return stashPop(a, args[1:])
	case "list":
		return stashList(a)
	case "show":
		return stashShow(a, args[1:])
	case "drop":
		return stashDrop(a, args[1:])
	case "clear":
		return stashClear(a)
	default:
		return fmt.Errorf("vstodo: unknown stash subcommand '%s'", args[0])
	}
}

func stashPush(a *app.App, args []string) error {
	msg := "WIP on stash"
	for i, arg := range args {
		if arg == "-m" && i+1 < len(args) {
			msg = args[i+1]
			break
		}
	}

	headID, err := currentCommitID(a)
	if err != nil || headID == sha1.Zero {
		return fmt.Errorf("cannot stash: no commits")
	}

	// Capture working tree state: every tracked file's current on-disk content.
	indexMap := a.Index.Entries()
	workTree := make(map[string]sha1.Digest)
	for path := range indexMap {
		data, err := os.ReadFile(a.Repo.Path(path))
		if err != nil {
			if os.IsNotExist(err) {
				workTree[path] = sha1.Zero
				continue
			}
			return err
		}
		id := object.HashBlob(data)
		if _, err := a.Store.Store(&object.Object{Type: object.BlobType, Data: data}); err != nil {
			return err
		}
		workTree[path] = id
	}

	// Build tree from working tree.
	treeID, err := buildTreeFromIndex(a, workTree, "")
	if err != nil {
		return err
	}

	parents := []sha1.Digest{headID}
	cb := commit.Build(treeID, parents, "vstodo", "vstodo", msg)
	stashID := object.HashCommit(cb)
	if _, err := a.Store.Store(&object.Object{Type: object.CommitType, Data: cb}); err != nil {
		return err
	}

	// Append to refs/stash as a reflog-like stack (store as refs/stash with
	// the newest commit; keep a simple list file for history).
	stashPath := filepath.Join(a.Repo.GitDir, "refs", "stash")
	if err := os.MkdirAll(filepath.Dir(stashPath), 0o755); err != nil {
		return err
	}
	// Read existing stash log.
	var entries []string
	if b, err := os.ReadFile(stashPath); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.TrimSpace(line) != "" {
				entries = append(entries, line)
			}
		}
	}
	entries = append([]string{stashID.Hex()}, entries...)
	var buf strings.Builder
	for i, e := range entries {
		if i > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(e)
	}
	buf.WriteByte('\n')
	if err := os.WriteFile(stashPath, []byte(buf.String()), 0o644); err != nil {
		return err
	}
	// Also write refs/stash to the latest commit id for ref compatibility.
	if err := a.Refs.WriteRef("refs/stash", stashID); err != nil {
		return err
	}

	fmt.Printf("Saved working directory and index state %s: %s\n", stashID.Hex()[:7], msg)
	return nil
}

func stashPop(a *app.App, args []string) error {
	stashID := resolveStashRef(a, args)
	if stashID == sha1.Zero {
		return fmt.Errorf("no stash to pop")
	}
	if err := stashApply(a, stashID); err != nil {
		return err
	}
	return stashDropRef(a, stashID)
}

func stashApply(a *app.App, id sha1.Digest) error {
	obj, err := a.Store.Load(id)
	if err != nil {
		return err
	}
	c, err := commit.Parse(id, obj.Data)
	if err != nil {
		return err
	}
	tm, err := treeToMap(a, c.Tree, "")
	if err != nil {
		return err
	}
	// Restore working tree files.
	for path := range a.Index.Entries() {
		_ = removeFile(a, path)
	}
	for path, bid := range tm {
		var data []byte
		if bid != sha1.Zero {
			data, err = readBlob(a, bid)
			if err != nil {
				return err
			}
		}
		if bid == sha1.Zero {
			_ = removeFile(a, path)
			continue
		}
		if err := writeFile(a, path, data); err != nil {
			return err
		}
	}
	a.Index = indexFromTree(tm)
	if err := a.SaveIndex(); err != nil {
		return err
	}
	fmt.Printf("Applied stash %s\n", id.Hex()[:7])
	return nil
}

func stashList(a *app.App) error {
	stashPath := filepath.Join(a.Repo.GitDir, "refs", "stash")
	b, err := os.ReadFile(stashPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No stash entries.")
			return nil
		}
		return err
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		fmt.Println("No stash entries.")
		return nil
	}
	// Reverse so newest first (already newest-first in file).
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fmt.Printf("stash@{%d}: %s\n", i, line[:7])
	}
	return nil
}

func stashShow(a *app.App, args []string) error {
	stashID := resolveStashRef(a, args)
	if stashID == sha1.Zero {
		return fmt.Errorf("no stash to show")
	}
	obj, err := a.Store.Load(stashID)
	if err != nil {
		return err
	}
	c, err := commit.Parse(stashID, obj.Data)
	if err != nil {
		return err
	}
	fmt.Printf("stash %s: %s\n", stashID.Hex()[:7], c.Message)
	tm, err := treeToMap(a, c.Tree, "")
	if err != nil {
		return err
	}
	for path, bid := range tm {
		fmt.Printf("%s\t%s\n", bid.Hex()[:7], path)
	}
	return nil
}

func stashDrop(a *app.App, args []string) error {
	stashID := resolveStashRef(a, args)
	if stashID == sha1.Zero {
		return fmt.Errorf("no stash to drop")
	}
	return stashDropRef(a, stashID)
}

func stashDropRef(a *app.App, id sha1.Digest) error {
	stashPath := filepath.Join(a.Repo.GitDir, "refs", "stash")
	b, err := os.ReadFile(stashPath)
	if err != nil {
		return err
	}
	var kept []string
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == id.Hex() {
			continue
		}
		kept = append(kept, line)
	}
	var buf strings.Builder
	for i, e := range kept {
		if i > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(e)
	}
	if len(kept) > 0 {
		buf.WriteByte('\n')
	}
	if err := os.WriteFile(stashPath, []byte(buf.String()), 0o644); err != nil {
		return err
	}
	if len(kept) == 0 {
		_ = os.Remove(stashPath)
	} else {
		d, err := sha1.FromHex(kept[0])
		if err != nil {
			return err
		}
		if err := a.Refs.WriteRef("refs/stash", d); err != nil {
			return err
		}
	}
	fmt.Printf("Dropped %s\n", id.Hex()[:7])
	return nil
}

func stashClear(a *app.App) error {
	stashPath := filepath.Join(a.Repo.GitDir, "refs", "stash")
	if err := os.Remove(stashPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	_ = a.Refs.DeleteRef("refs/stash")
	fmt.Println("Cleared stash.")
	return nil
}

func resolveStashRef(a *app.App, args []string) sha1.Digest {
	if len(args) > 0 {
		// Parse stash@{N} or raw hash.
		s := args[0]
		if strings.HasPrefix(s, "stash@{") && strings.HasSuffix(s, "}") {
			inner := s[len("stash@{") : len(s)-1]
			n, err := strconv.Atoi(inner)
			if err != nil {
				return sha1.Zero
			}
			stashPath := filepath.Join(a.Repo.GitDir, "refs", "stash")
			b, err := os.ReadFile(stashPath)
			if err != nil {
				return sha1.Zero
			}
			var entries []string
			for _, line := range strings.Split(string(b), "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					entries = append(entries, line)
				}
			}
			if n < 0 || n >= len(entries) {
				return sha1.Zero
			}
			d, err := sha1.FromHex(entries[n])
			if err != nil {
				return sha1.Zero
			}
			return d
		}
		if d, err := sha1.FromHex(s); err == nil {
			return d
		}
		// Try ref.
		if d, err := a.Refs.ReadRef("refs/stash"); err == nil {
			return d
		}
		return sha1.Zero
	}
	// Default: latest stash.
	if d, err := a.Refs.ReadRef("refs/stash"); err == nil {
		return d
	}
	stashPath := filepath.Join(a.Repo.GitDir, "refs", "stash")
	b, err := os.ReadFile(stashPath)
	if err != nil {
		return sha1.Zero
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			d, _ := sha1.FromHex(line)
			return d
		}
	}
	return sha1.Zero
}
