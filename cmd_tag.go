package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

// cmdTag lists, creates, or deletes lightweight and annotated tags.
func cmdTag(a *app.App, args []string) error {
	if len(args) == 0 {
		refs, err := a.Refs.ListRefs()
		if err != nil {
			return err
		}
		var names []string
		for name := range refs {
			if strings.HasPrefix(name, "refs/tags/") {
				names = append(names, strings.TrimPrefix(name, "refs/tags/"))
			}
		}
		sort.Strings(names)
		for _, name := range names {
			id, err := a.Refs.ReadRef("refs/tags/" + name)
			if err != nil {
				return err
			}
			fmt.Printf("%s\t%s\n", name, id.Hex()[:7])
		}
		return nil
	}

	var positional []string
	annotated := false
	force := false
	deleteTag := false
	message := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-a", "--annotate":
			annotated = true
		case "-f", "--force":
			force = true
		case "-d", "--delete":
			deleteTag = true
		case "-m", "--message":
			if i+1 >= len(args) {
				return fmt.Errorf("tag: -m requires a message")
			}
			i++
			message = args[i]
			annotated = true
		default:
			positional = append(positional, args[i])
		}
	}
	if len(positional) < 1 || len(positional) > 2 {
		return fmt.Errorf("usage: vstodo tag [-a] [-f] [-m message] <name> [target]")
	}
	name := positional[0]
	if err := validateTagName(name); err != nil {
		return err
	}
	refName := "refs/tags/" + name
	if deleteTag {
		if len(positional) != 1 {
			return fmt.Errorf("usage: vstodo tag -d <name>")
		}
		if err := a.Refs.DeleteRef(refName); err != nil {
			return fmt.Errorf("tag: delete %s: %w", name, err)
		}
		fmt.Printf("Deleted tag '%s'\n", name)
		return nil
	}
	if _, err := a.Refs.ReadRef(refName); err == nil && !force {
		return fmt.Errorf("tag '%s' already exists (use -f to replace it)", name)
	}

	target := "HEAD"
	if len(positional) == 2 {
		target = positional[1]
	}
	var targetID sha1.Digest
	var err error
	if target == "HEAD" {
		targetID, err = currentCommitID(a)
	} else {
		targetID, err = resolveCommitish(a, target)
	}
	if err != nil || targetID == sha1.Zero {
		return fmt.Errorf("tag: cannot resolve target %q", target)
	}
	targetObj, err := a.Store.Load(targetID)
	if err != nil {
		return err
	}
	if targetObj.Type == object.TagType {
		targetID, err = peelAnnotatedTag(a, targetID)
		if err != nil {
			return err
		}
		targetObj, err = a.Store.Load(targetID)
		if err != nil {
			return err
		}
	}
	if targetObj.Type != object.CommitType {
		return fmt.Errorf("tag: target %s is not a commit", targetID.Hex())
	}
	if _, err := commit.Parse(targetID, targetObj.Data); err != nil {
		return err
	}

	refID := targetID
	if annotated {
		if strings.TrimSpace(message) == "" {
			return fmt.Errorf("tag: annotated tags require a non-empty -m message")
		}
		payload := fmt.Sprintf("object %s\ntype commit\ntag %s\ntagger %s %d +0000\n\n%s\n",
			targetID.Hex(), name, commitIdentity(a), time.Now().Unix(), message)
		refID, err = a.Store.WriteRaw(object.TagType, []byte(payload))
		if err != nil {
			return err
		}
	}
	if err := a.Refs.WriteRef(refName, refID); err != nil {
		return err
	}
	fmt.Printf("Created tag '%s' at %s\n", name, targetID.Hex()[:7])
	return nil
}

func validateTagName(name string) error {
	if name == "" || strings.HasPrefix(name, "-") || strings.HasPrefix(name, "/") ||
		strings.HasSuffix(name, "/") || strings.HasSuffix(name, ".") ||
		strings.Contains(name, "..") || strings.Contains(name, "//") ||
		strings.Contains(name, "@{") || strings.ContainsAny(name, " ~^:?*[\\") {
		return fmt.Errorf("tag: invalid tag name %q", name)
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") {
			return fmt.Errorf("tag: invalid tag name %q", name)
		}
	}
	return nil
}

func peelAnnotatedTag(a *app.App, id sha1.Digest) (sha1.Digest, error) {
	for depth := 0; depth < 8; depth++ {
		obj, err := a.Store.Load(id)
		if err != nil {
			return sha1.Zero, err
		}
		if obj.Type != object.TagType {
			return id, nil
		}
		fields := strings.SplitN(string(obj.Data), "\n\n", 2)[0]
		found := false
		for _, line := range strings.Split(fields, "\n") {
			if strings.HasPrefix(line, "object ") {
				next, err := sha1.FromHex(strings.TrimPrefix(line, "object "))
				if err != nil {
					return sha1.Zero, fmt.Errorf("tag: malformed annotated tag %s", id.Hex())
				}
				id = next
				found = true
				break
			}
		}
		if !found {
			return sha1.Zero, fmt.Errorf("tag: annotated tag %s has no object header", id.Hex())
		}
	}
	return sha1.Zero, fmt.Errorf("tag: nested tag depth exceeded")
}
