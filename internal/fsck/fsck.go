// Package fsck verifies the integrity and internal references of a VSTodo repository.
package fsck

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
	"vstodo/internal/tree"
)

// Report summarizes a successful repository integrity check.
type Report struct {
	Objects    int
	Blobs      int
	Trees      int
	Commits    int
	Tags       int
	References int
}

// Check verifies every loose object's checksum, structure, and object links,
// then verifies all branch, tag, and HEAD references. It never modifies data.
func Check(a *app.App) (Report, error) {
	var report Report
	ids, err := a.Store.Objects()
	if err != nil {
		return report, fmt.Errorf("fsck: enumerate objects: %w", err)
	}
	objects := make(map[sha1.Digest]*object.Object, len(ids))
	for _, id := range ids {
		obj, err := a.Store.Load(id)
		if err != nil {
			return report, fmt.Errorf("fsck: object %s: %w", id.Hex(), err)
		}
		objects[id] = obj
		report.Objects++
		switch obj.Type {
		case object.BlobType:
			report.Blobs++
		case object.TreeType:
			report.Trees++
		case object.CommitType:
			report.Commits++
		case object.TagType:
			report.Tags++
		default:
			return report, fmt.Errorf("fsck: object %s has unsupported type %q", id.Hex(), obj.Type)
		}
	}
	for id, obj := range objects {
		switch obj.Type {
		case object.TreeType:
			if err := checkTree(id, obj.Data, objects); err != nil {
				return report, err
			}
		case object.CommitType:
			if err := checkCommit(id, obj.Data, objects); err != nil {
				return report, err
			}
		case object.TagType:
			if err := checkTag(id, obj.Data, objects); err != nil {
				return report, err
			}
		}
	}
	refs, err := a.Refs.ListRefs()
	if err != nil {
		return report, fmt.Errorf("fsck: list references: %w", err)
	}
	for name, id := range refs {
		obj, ok := objects[id]
		if !ok {
			return report, fmt.Errorf("fsck: reference %s points to missing object %s", name, id.Hex())
		}
		if strings.HasPrefix(name, "refs/heads/") && obj.Type != object.CommitType {
			return report, fmt.Errorf("fsck: branch %s points to %s object %s, not a commit", name, obj.Type, id.Hex())
		}
		report.References++
	}
	head, err := a.Refs.Head()
	if err == nil {
		obj, ok := objects[head]
		if !ok {
			return report, fmt.Errorf("fsck: HEAD points to missing object %s", head.Hex())
		}
		if obj.Type != object.CommitType {
			return report, fmt.Errorf("fsck: HEAD points to %s object %s, not a commit", obj.Type, head.Hex())
		}
	} else {
		branch, branchErr := a.Refs.CurrentBranch()
		if branchErr != nil {
			return report, fmt.Errorf("fsck: read HEAD: %w", branchErr)
		}
		if branch == "" || !errors.Is(err, os.ErrNotExist) {
			return report, fmt.Errorf("fsck: resolve HEAD: %w", err)
		}
		// An unborn symbolic branch is valid in a newly initialized repository.
	}
	return report, nil
}

func checkTree(id sha1.Digest, data []byte, objects map[sha1.Digest]*object.Object) error {
	parsed, err := tree.Parse(id, data)
	if err != nil {
		return fmt.Errorf("fsck: tree %s: %w", id.Hex(), err)
	}
	seen := make(map[string]bool, len(parsed.Entries))
	for _, entry := range parsed.Entries {
		if entry.Name == "" || entry.Name == "." || entry.Name == ".." ||
			strings.ContainsAny(entry.Name, "/\\") || strings.ContainsRune(entry.Name, 0) {
			return fmt.Errorf("fsck: tree %s has invalid entry name %q", id.Hex(), entry.Name)
		}
		if seen[entry.Name] {
			return fmt.Errorf("fsck: tree %s contains duplicate entry %q", id.Hex(), entry.Name)
		}
		seen[entry.Name] = true
		target, ok := objects[entry.ID]
		if entry.Mode == "160000" {
			if ok && target.Type != object.CommitType {
				return fmt.Errorf("fsck: tree %s gitlink %q points to %s, not a commit", id.Hex(), entry.Name, target.Type)
			}
			continue
		}
		if !ok {
			return fmt.Errorf("fsck: tree %s entry %q points to missing object %s", id.Hex(), entry.Name, entry.ID.Hex())
		}
		var expected object.Type
		switch entry.Mode {
		case "040000", "40000":
			expected = object.TreeType
		case "100644", "100755", "120000":
			expected = object.BlobType
		case "160000":
			expected = object.CommitType
		default:
			return fmt.Errorf("fsck: tree %s entry %q has unsupported mode %q", id.Hex(), entry.Name, entry.Mode)
		}
		if target.Type != expected {
			return fmt.Errorf("fsck: tree %s entry %q expects %s, found %s", id.Hex(), entry.Name, expected, target.Type)
		}
	}
	return nil
}

func checkCommit(id sha1.Digest, data []byte, objects map[sha1.Digest]*object.Object) error {
	parsed, err := commit.Parse(id, data)
	if err != nil {
		return fmt.Errorf("fsck: commit %s: %w", id.Hex(), err)
	}
	treeObject, ok := objects[parsed.Tree]
	if !ok || treeObject.Type != object.TreeType {
		return fmt.Errorf("fsck: commit %s references missing or non-tree root %s", id.Hex(), parsed.Tree.Hex())
	}
	for _, parent := range parsed.Parents {
		parentObject, ok := objects[parent]
		if !ok || parentObject.Type != object.CommitType {
			return fmt.Errorf("fsck: commit %s references missing or non-commit parent %s", id.Hex(), parent.Hex())
		}
	}
	return nil
}

func checkTag(id sha1.Digest, data []byte, objects map[sha1.Digest]*object.Object) error {
	header := strings.SplitN(string(data), "\n\n", 2)[0]
	var targetID sha1.Digest
	var targetType object.Type
	foundObject, foundType := false, false
	for _, line := range strings.Split(header, "\n") {
		switch {
		case strings.HasPrefix(line, "object "):
			parsed, err := sha1.FromHex(strings.TrimPrefix(line, "object "))
			if err != nil {
				return fmt.Errorf("fsck: tag %s has invalid target ID: %w", id.Hex(), err)
			}
			targetID, foundObject = parsed, true
		case strings.HasPrefix(line, "type "):
			targetType, foundType = object.Type(strings.TrimPrefix(line, "type ")), true
		}
	}
	if !foundObject || !foundType {
		return fmt.Errorf("fsck: tag %s is missing its object or type header", id.Hex())
	}
	target, ok := objects[targetID]
	if !ok {
		return fmt.Errorf("fsck: tag %s references missing object %s", id.Hex(), targetID.Hex())
	}
	if target.Type != targetType {
		return fmt.Errorf("fsck: tag %s declares target type %s but object %s is %s", id.Hex(), targetType, targetID.Hex(), target.Type)
	}
	return nil
}
