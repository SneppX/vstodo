package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vstodo/internal/app"
	"vstodo/internal/commit"
)

// cmdWorktree manages multiple working trees.
func cmdWorktree(a *app.App, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: vstodo worktree <add|list|remove|lock|unlock> ...")
	}
	switch args[0] {
	case "add":
		return worktreeAdd(a, args[1:])
	case "list":
		return worktreeList(a)
	case "remove":
		return worktreeRemove(a, args[1:])
	case "lock":
		return worktreeLock(a, args[1:])
	case "unlock":
		return worktreeUnlock(a, args[1:])
	default:
		return fmt.Errorf("vstodo: unknown worktree subcommand '%s'", args[0])
	}
}

func worktreeDir(a *app.App) string {
	return filepath.Join(a.Repo.GitDir, "worktrees")
}

func worktreeAdd(a *app.App, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: vstodo worktree add <path> <commit-ish>")
	}
	relPath := args[0]
	target := args[1]

	id, err := resolveCommitish(a, target)
	if err != nil {
		return err
	}
	obj, err := a.Store.Load(id)
	if err != nil {
		return err
	}
	c, err := commit.Parse(id, obj.Data)
	if err != nil {
		return err
	}

	absPath := filepath.Join(a.Repo.Root, relPath)
	if err := os.MkdirAll(absPath, 0o755); err != nil {
		return err
	}

	// Write the commit's tree into the new working tree.
	tm, err := treeToMap(a, c.Tree, "")
	if err != nil {
		return err
	}
	for path, bid := range tm {
		data, err := readBlob(a, bid)
		if err != nil {
			return err
		}
		full := filepath.Join(absPath, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, data, 0o644); err != nil {
			return err
		}
	}

	// Record the worktree in the admin file.
	wtAdmin := filepath.Join(a.Repo.GitDir, "worktrees", "worktrees-admin")
	var entries []string
	if b, err := os.ReadFile(wtAdmin); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.TrimSpace(line) != "" {
				entries = append(entries, line)
			}
		}
	}
	entries = append(entries, relPath+"="+id.Hex())
	var buf strings.Builder
	for i, e := range entries {
		if i > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(e)
	}
	buf.WriteByte('\n')
	if err := os.WriteFile(wtAdmin, []byte(buf.String()), 0o644); err != nil {
		return err
	}

	// Create a .vstodo file inside the new worktree pointing back to main gitdir.
	metaDir := filepath.Join(absPath, ".vstodo")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(metaDir, "gitdir"), []byte(a.Repo.GitDir+"\n"), 0o644); err != nil {
		return err
	}

	fmt.Printf("Preparing worktree %s (new branch)\nHEAD is now %s\n", relPath, id.Hex()[:7])
	return nil
}

func worktreeList(a *app.App) error {
	wtAdmin := filepath.Join(a.Repo.GitDir, "worktrees", "worktrees-admin")
	b, err := os.ReadFile(wtAdmin)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No linked working trees.")
			return nil
		}
		return err
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		fmt.Printf("%s\t%s\n", parts[0], parts[1][:7])
	}
	return nil
}

func worktreeRemove(a *app.App, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: vstodo worktree remove <path>")
	}
	relPath := args[0]
	absPath := filepath.Join(a.Repo.Root, relPath)

	wtAdmin := filepath.Join(a.Repo.GitDir, "worktrees", "worktrees-admin")
	b, err := os.ReadFile(wtAdmin)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no such worktree: %s", relPath)
		}
		return err
	}
	var kept []string
	found := false
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && parts[0] == relPath {
			found = true
			continue
		}
		kept = append(kept, line)
	}
	if !found {
		return fmt.Errorf("no such worktree: %s", relPath)
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
	if err := os.WriteFile(wtAdmin, []byte(buf.String()), 0o644); err != nil {
		return err
	}
	// Remove .vstodo metadata and the directory (only if empty of tracked content we created).
	_ = os.RemoveAll(filepath.Join(absPath, ".vstodo"))
	if err := os.Remove(absPath); err != nil && !os.IsNotExist(err) {
		// Directory may not be empty; that's acceptable.
	}
	fmt.Printf("Removed worktree %s\n", relPath)
	return nil
}

func worktreeLock(a *app.App, args []string) error {
	return worktreeLockState(a, args, true)
}

func worktreeUnlock(a *app.App, args []string) error {
	return worktreeLockState(a, args, false)
}

func worktreeLockState(a *app.App, args []string, lock bool) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: vstodo worktree lock|unlock <path>")
	}
	relPath := args[0]
	lockedFile := filepath.Join(a.Repo.GitDir, "worktrees", relPath+".locked")
	if lock {
		if err := os.WriteFile(lockedFile, []byte("locked\n"), 0o644); err != nil {
			return err
		}
		fmt.Printf("Locked worktree %s\n", relPath)
	} else {
		_ = os.Remove(lockedFile)
		fmt.Printf("Unlocked worktree %s\n", relPath)
	}
	return nil
}
