package main

import (
	"fmt"
	"sort"
	"strings"

	"vstodo/internal/app"
	"vstodo/internal/sha1"
)

// cmdBranch lists, creates, deletes, or renames local branches.
func cmdBranch(a *app.App, args []string) error {
	if len(args) == 0 {
		refs, err := a.Refs.ListRefs()
		if err != nil {
			return err
		}
		current, _ := a.Refs.CurrentBranch()
		var names []string
		for name := range refs {
			if strings.HasPrefix(name, "refs/heads/") {
				names = append(names, strings.TrimPrefix(name, "refs/heads/"))
			}
		}
		sort.Strings(names)
		for _, name := range names {
			id, err := a.Refs.ReadRef("refs/heads/" + name)
			if err != nil {
				return err
			}
			mark := " "
			if name == current {
				mark = "*"
			}
			fmt.Printf("%s%s\t%s\n", mark, name, id.Hex()[:7])
		}
		return nil
	}

	switch args[0] {
	case "-d", "--delete":
		if len(args) != 2 {
			return fmt.Errorf("usage: vstodo branch -d <name>")
		}
		return deleteBranch(a, args[1], false)
	case "-D", "--force-delete":
		if len(args) != 2 {
			return fmt.Errorf("usage: vstodo branch -D <name>")
		}
		return deleteBranch(a, args[1], true)
	case "-m", "--move":
		if len(args) != 3 {
			return fmt.Errorf("usage: vstodo branch -m <old> <new>")
		}
		return renameBranch(a, args[1], args[2], false)
	case "-M", "--force-move":
		if len(args) != 3 {
			return fmt.Errorf("usage: vstodo branch -M <old> <new>")
		}
		return renameBranch(a, args[1], args[2], true)
	}

	if len(args) > 2 {
		return fmt.Errorf("usage: vstodo branch <name> [start-point]")
	}
	name := args[0]
	if err := validateBranchName(name); err != nil {
		return err
	}
	refName := "refs/heads/" + name
	if _, err := a.Refs.ReadRef(refName); err == nil {
		return fmt.Errorf("branch '%s' already exists", name)
	}
	var id sha1.Digest
	var err error
	if len(args) == 2 {
		id, err = resolveCommitish(a, args[1])
	} else {
		id, err = currentCommitID(a)
	}
	if err != nil || id == sha1.Zero {
		return fmt.Errorf("branch: cannot resolve start point")
	}
	if err := a.Refs.WriteRef(refName, id); err != nil {
		return err
	}
	fmt.Printf("Branch '%s' created at %s\n", name, id.Hex()[:7])
	return nil
}

func validateBranchName(name string) error {
	if name == "" || strings.HasPrefix(name, "-") || strings.HasPrefix(name, "/") ||
		strings.HasSuffix(name, "/") || strings.HasSuffix(name, ".") ||
		strings.Contains(name, "..") || strings.Contains(name, "//") ||
		strings.Contains(name, "@{") || strings.ContainsAny(name, " ~^:?*[\\") {
		return fmt.Errorf("branch: invalid branch name %q", name)
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") {
			return fmt.Errorf("branch: invalid branch name %q", name)
		}
	}
	return nil
}

func deleteBranch(a *app.App, name string, force bool) error {
	if err := validateBranchName(name); err != nil {
		return err
	}
	current, _ := a.Refs.CurrentBranch()
	if current == name {
		return fmt.Errorf("cannot delete the checked-out branch '%s'", name)
	}
	id, err := a.Refs.ReadRef("refs/heads/" + name)
	if err != nil {
		return fmt.Errorf("branch '%s' not found", name)
	}
	if !force {
		headID, err := currentCommitID(a)
		if err != nil {
			return err
		}
		merged, err := isCommitAncestor(a, id, headID)
		if err != nil {
			return err
		}
		if !merged {
			return fmt.Errorf("branch '%s' is not merged; use -D to force deletion", name)
		}
	}
	if err := a.Refs.DeleteRef("refs/heads/" + name); err != nil {
		return err
	}
	fmt.Printf("Deleted branch '%s'\n", name)
	return nil
}

func renameBranch(a *app.App, oldName, newName string, force bool) error {
	if err := validateBranchName(oldName); err != nil {
		return err
	}
	if err := validateBranchName(newName); err != nil {
		return err
	}
	if oldName == newName {
		return fmt.Errorf("branch: source and destination are identical")
	}
	oldID, err := a.Refs.ReadRef("refs/heads/" + oldName)
	if err != nil {
		return fmt.Errorf("branch '%s' not found", oldName)
	}
	newRef := "refs/heads/" + newName
	if _, err := a.Refs.ReadRef(newRef); err == nil {
		if !force {
			return fmt.Errorf("branch '%s' already exists", newName)
		}
		if err := a.Refs.DeleteRef(newRef); err != nil {
			return err
		}
	}
	if err := a.Refs.WriteRef(newRef, oldID); err != nil {
		return err
	}
	current, _ := a.Refs.CurrentBranch()
	if current == oldName {
		if err := a.Refs.SetHead("ref: " + newRef); err != nil {
			return err
		}
	}
	if err := a.Refs.DeleteRef("refs/heads/" + oldName); err != nil {
		return err
	}
	fmt.Printf("Renamed branch '%s' to '%s'\n", oldName, newName)
	return nil
}
