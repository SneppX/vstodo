package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

func cmdStash(a *app.App, args []string) error {
	if len(args) == 0 {
		args = []string{"push"}
	}
	switch args[0] {
	case "push":
		return stashPush(a, args[1:])
	case "pop":
		return stashPop(a, args[1:])
	case "apply":
		id := resolveStashRef(a, args[1:])
		if id == sha1.Zero {
			return fmt.Errorf("no stash to apply")
		}
		return stashApply(a, id)
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
	includeUntracked := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-m", "--message":
			if i+1 >= len(args) {
				return fmt.Errorf("stash push: -m requires a message")
			}
			i++
			msg = args[i]
		case "-u", "--include-untracked":
			includeUntracked = true
		}
	}

	headID, err := currentCommitID(a)
	if err != nil || headID == sha1.Zero {
		return fmt.Errorf("cannot stash: no commits")
	}
	if err := ensureIndexFromHEAD(a); err != nil {
		return err
	}
	headObj, err := a.Store.Load(headID)
	if err != nil {
		return err
	}
	headCommit, err := commit.Parse(headID, headObj.Data)
	if err != nil {
		return err
	}
	headTree, err := treeToMap(a, headCommit.Tree, "")
	if err != nil {
		return err
	}
	indexTree := a.Index.Entries()
	managed := make(map[string]bool, len(headTree)+len(indexTree))
	for path := range headTree {
		managed[path] = true
	}
	for path := range indexTree {
		managed[path] = true
	}

	workTree := make(map[string]sha1.Digest)
	files, err := walkFiles(a.Repo.Root)
	if err != nil {
		return err
	}
	for _, path := range files {
		if !managed[path] && !includeUntracked {
			continue
		}
		full, err := worktreePath(a, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(full)
		if err != nil {
			return err
		}
		id, err := a.Store.WriteRaw(object.BlobType, data)
		if err != nil {
			return err
		}
		workTree[path] = id
	}
	if sameTreeSnapshot(headTree, workTree) && sameTreeSnapshot(headTree, indexTree) {
		return fmt.Errorf("no local changes to stash")
	}

	indexTreeID, err := buildTreeFromIndex(a, indexTree, "")
	if err != nil {
		return err
	}
	indexCommitData := commit.Build(indexTreeID, []sha1.Digest{headID}, "vstodo", "vstodo", "index state for stash")
	indexCommitID, err := a.Store.WriteRaw(object.CommitType, indexCommitData)
	if err != nil {
		return err
	}
	workTreeID, err := buildTreeFromIndex(a, workTree, "")
	if err != nil {
		return err
	}
	stashData := commit.Build(workTreeID, []sha1.Digest{headID, indexCommitID}, "vstodo", "vstodo", msg)
	stashID, err := a.Store.WriteRaw(object.CommitType, stashData)
	if err != nil {
		return err
	}
	if err := appendStashLog(a, stashID); err != nil {
		return err
	}
	if err := a.Refs.WriteRef("refs/stash", stashID); err != nil {
		return err
	}

	// Reset tracked and optionally included untracked files to the original HEAD.
	if err := applyMergedTree(a, workTree, headTree); err != nil {
		return fmt.Errorf("stash saved as %s but worktree reset failed: %w", stashID.Hex()[:7], err)
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
	stashObj, err := a.Store.Load(id)
	if err != nil {
		return err
	}
	if stashObj.Type != object.CommitType {
		return fmt.Errorf("stash: target is not a commit")
	}
	stashCommit, err := commit.Parse(id, stashObj.Data)
	if err != nil {
		return err
	}
	if len(stashCommit.Parents) < 2 {
		return fmt.Errorf("stash: legacy stash without index state is not supported")
	}

	headID, err := currentCommitID(a)
	if err != nil || headID == sha1.Zero {
		return fmt.Errorf("stash apply: no current commit")
	}
	headObj, err := a.Store.Load(headID)
	if err != nil {
		return err
	}
	headCommit, err := commit.Parse(headID, headObj.Data)
	if err != nil {
		return err
	}
	if err := requireCleanWorkingTree(a, headCommit.Tree); err != nil {
		return fmt.Errorf("stash apply: %w", err)
	}

	baseObj, err := a.Store.Load(stashCommit.Parents[0])
	if err != nil {
		return err
	}
	baseCommit, err := commit.Parse(stashCommit.Parents[0], baseObj.Data)
	if err != nil {
		return err
	}
	indexObj, err := a.Store.Load(stashCommit.Parents[1])
	if err != nil {
		return err
	}
	indexCommit, err := commit.Parse(stashCommit.Parents[1], indexObj.Data)
	if err != nil {
		return err
	}
	baseTree, err := treeToMap(a, baseCommit.Tree, "")
	if err != nil {
		return err
	}
	currentTree, err := treeToMap(a, headCommit.Tree, "")
	if err != nil {
		return err
	}
	stashTree, err := treeToMap(a, stashCommit.Tree, "")
	if err != nil {
		return err
	}
	stashIndexTree, err := treeToMap(a, indexCommit.Tree, "")
	if err != nil {
		return err
	}

	mergedWorkTree, workContents, workConflicts, err := mergeTrees(a, baseTree, currentTree, stashTree)
	if err != nil {
		return err
	}
	mergedIndex, indexContents, indexConflicts, err := mergeTrees(a, baseTree, currentTree, stashIndexTree)
	if err != nil {
		return err
	}
	if len(workConflicts) > 0 || len(indexConflicts) > 0 {
		conflicts := append(workConflicts, indexConflicts...)
		return fmt.Errorf("stash conflicts in %s; no files or refs were changed", strings.Join(conflicts, ", "))
	}
	if err := persistStashMergedBlobs(a, mergedWorkTree, workContents); err != nil {
		return err
	}
	if err := persistStashMergedBlobs(a, mergedIndex, indexContents); err != nil {
		return err
	}
	if err := applyMergedTree(a, currentTree, mergedWorkTree); err != nil {
		return err
	}
	for path := range a.Index.Entries() {
		a.Index.Remove(path)
	}
	for path, blobID := range mergedIndex {
		a.Index.Add(path, blobID)
	}
	if err := a.SaveIndex(); err != nil {
		return err
	}
	fmt.Printf("Applied stash %s\n", id.Hex()[:7])
	return nil
}

func persistStashMergedBlobs(a *app.App, tree map[string]sha1.Digest, contents map[string][]byte) error {
	for path, data := range contents {
		id, err := a.Store.WriteRaw(object.BlobType, data)
		if err != nil {
			return fmt.Errorf("stash: store merged blob %s: %w", path, err)
		}
		tree[path] = id
	}
	return nil
}

func stashLogPath(a *app.App) string {
	return filepath.Join(a.Repo.GitDir, "logs", "refs", "stash")
}

func appendStashLog(a *app.App, id sha1.Digest) error {
	path := stashLogPath(a)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var entries []string
	if data, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				entries = append(entries, line)
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	entries = append([]string{id.Hex()}, entries...)
	return writeStashLog(path, entries)
}

func writeStashLog(path string, entries []string) error {
	var b strings.Builder
	for i, entry := range entries {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(entry)
	}
	if len(entries) > 0 {
		b.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func stashList(a *app.App) error {
	data, err := os.ReadFile(stashLogPath(a))
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No stash entries.")
			return nil
		}
		return err
	}
	entries := stashLogEntries(data)
	if len(entries) == 0 {
		fmt.Println("No stash entries.")
		return nil
	}
	for i, entry := range entries {
		fmt.Printf("stash@{%d}: %s\n", i, entry[:7])
	}
	return nil
}

func stashShow(a *app.App, args []string) error {
	id := resolveStashRef(a, args)
	if id == sha1.Zero {
		return fmt.Errorf("no stash to show")
	}
	obj, err := a.Store.Load(id)
	if err != nil {
		return err
	}
	c, err := commit.Parse(id, obj.Data)
	if err != nil {
		return err
	}
	fmt.Printf("stash %s: %s\n", id.Hex()[:7], c.Message)
	tm, err := treeToMap(a, c.Tree, "")
	if err != nil {
		return err
	}
	var paths []string
	for path := range tm {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		fmt.Printf("%s\t%s\n", tm[path].Hex()[:7], path)
	}
	return nil
}

func stashDrop(a *app.App, args []string) error {
	id := resolveStashRef(a, args)
	if id == sha1.Zero {
		return fmt.Errorf("no stash to drop")
	}
	return stashDropRef(a, id)
}

func stashDropRef(a *app.App, id sha1.Digest) error {
	path := stashLogPath(a)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var kept []string
	for _, entry := range stashLogEntries(data) {
		if entry != id.Hex() {
			kept = append(kept, entry)
		}
	}
	if err := writeStashLog(path, kept); err != nil {
		return err
	}
	if len(kept) == 0 {
		_ = os.Remove(path)
		return a.Refs.DeleteRef("refs/stash")
	}
	latest, err := sha1.FromHex(kept[0])
	if err != nil {
		return err
	}
	if err := a.Refs.WriteRef("refs/stash", latest); err != nil {
		return err
	}
	fmt.Printf("Dropped %s\n", id.Hex()[:7])
	return nil
}

func stashClear(a *app.App) error {
	if err := os.Remove(stashLogPath(a)); err != nil && !os.IsNotExist(err) {
		return err
	}
	_ = a.Refs.DeleteRef("refs/stash")
	fmt.Println("Cleared stash.")
	return nil
}

func stashLogEntries(data []byte) []string {
	var entries []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			entries = append(entries, line)
		}
	}
	return entries
}

func resolveStashRef(a *app.App, args []string) sha1.Digest {
	if len(args) > 0 {
		s := args[0]
		if strings.HasPrefix(s, "stash@{") && strings.HasSuffix(s, "}") {
			n, err := strconv.Atoi(s[len("stash@{") : len(s)-1])
			if err != nil || n < 0 {
				return sha1.Zero
			}
			data, err := os.ReadFile(stashLogPath(a))
			if err != nil {
				return sha1.Zero
			}
			entries := stashLogEntries(data)
			if n >= len(entries) {
				return sha1.Zero
			}
			id, _ := sha1.FromHex(entries[n])
			return id
		}
		if id, err := sha1.FromHex(s); err == nil {
			return id
		}
	}
	if id, err := a.Refs.ReadRef("refs/stash"); err == nil {
		return id
	}
	return sha1.Zero
}
