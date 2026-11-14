// Package gc implements conservative garbage collection for loose objects.
// Packed-object deletion is intentionally disabled until pack-backed reads are
// integrated into the storage layer.
package gc

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/ref"
	"vstodo/internal/sha1"
	"vstodo/internal/tree"
)

// GC performs a non-destructive dry run. Use GCWithOptions with dryRun=false
// to remove unreachable loose objects.
func GC(a *app.App) error {
	return GCWithOptions(a, true)
}

// GCWithOptions marks all objects reachable from HEAD, branches, tags, and
// stash history. It only prunes unreachable loose objects; it never deletes
// objects that may be stored in packs.
func GCWithOptions(a *app.App, dryRun bool) error {
	live, err := markReachable(a)
	if err != nil {
		return fmt.Errorf("gc: mark: %w", err)
	}
	all, err := a.Store.Objects()
	if err != nil {
		return fmt.Errorf("gc: enumerate loose objects: %w", err)
	}
	var unreachable []sha1.Digest
	for _, id := range all {
		if !live[id] {
			unreachable = append(unreachable, id)
		}
	}
	if dryRun {
		fmt.Printf("vstodo gc: reachable=%d unreachable=%d (dry run; use --prune to remove loose objects)\n", len(live), len(unreachable))
		return nil
	}
	for _, id := range unreachable {
		if err := os.Remove(loosePath(a.Store.ObjectsDir, id)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("gc: prune %s: %w", id.Hex(), err)
		}
	}
	fmt.Printf("vstodo gc: reachable=%d pruned=%d (loose objects only; repacking is not enabled)\n", len(live), len(unreachable))
	return nil
}

func markReachable(a *app.App) (map[sha1.Digest]bool, error) {
	live := make(map[sha1.Digest]bool)
	if head, err := a.Refs.Head(); err == nil && head != sha1.Zero {
		if err := mark(live, a, head); err != nil {
			return nil, err
		}
	} else if err != nil && !errors.Is(err, ref.ErrNotFound) && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	refs, err := a.Refs.ListRefs()
	if err != nil {
		return nil, err
	}
	for _, id := range refs {
		if err := mark(live, a, id); err != nil {
			return nil, err
		}
	}

	stashLog := filepath.Join(a.Repo.GitDir, "logs", "refs", "stash")
	if data, err := os.ReadFile(stashLog); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			id, err := sha1.FromHex(line)
			if err != nil {
				return nil, fmt.Errorf("gc: malformed stash entry %q", line)
			}
			if err := mark(live, a, id); err != nil {
				return nil, err
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if stashID, err := a.Refs.ReadRef("refs/stash"); err == nil {
		if err := mark(live, a, stashID); err != nil {
			return nil, err
		}
	}
	return live, nil
}

func mark(live map[sha1.Digest]bool, a *app.App, id sha1.Digest) error {
	if id == sha1.Zero || live[id] {
		return nil
	}
	obj, err := a.Store.Load(id)
	if err != nil {
		return fmt.Errorf("referenced object %s cannot be loaded: %w", id.Hex(), err)
	}
	live[id] = true
	switch obj.Type {
	case object.CommitType:
		c, err := commit.Parse(id, obj.Data)
		if err != nil {
			return err
		}
		if err := mark(live, a, c.Tree); err != nil {
			return err
		}
		for _, parent := range c.Parents {
			if err := mark(live, a, parent); err != nil {
				return err
			}
		}
	case object.TreeType:
		t, err := tree.Parse(id, obj.Data)
		if err != nil {
			return err
		}
		for _, entry := range t.Entries {
			if err := mark(live, a, entry.ID); err != nil {
				return err
			}
		}
	case object.TagType:
		header := strings.SplitN(string(obj.Data), "\n\n", 2)[0]
		found := false
		for _, line := range strings.Split(header, "\n") {
			if strings.HasPrefix(line, "object ") {
				target, err := sha1.FromHex(strings.TrimPrefix(line, "object "))
				if err != nil {
					return fmt.Errorf("gc: malformed tag object %s", id.Hex())
				}
				found = true
				if err := mark(live, a, target); err != nil {
					return err
				}
				break
			}
		}
		if !found {
			return fmt.Errorf("gc: tag object %s has no target", id.Hex())
		}
	case object.BlobType:
		// Blob objects do not reference other objects.
	default:
		return fmt.Errorf("gc: unsupported object type %s for %s", obj.Type, id.Hex())
	}
	return nil
}

func loosePath(objectsDir string, id sha1.Digest) string {
	h := id.Hex()
	return filepath.Join(objectsDir, h[:2], h[2:])
}
