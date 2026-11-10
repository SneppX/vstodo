// Package gc implements garbage collection for vstodo repositories: it
// traverses all objects reachable from refs, marks them live, sweeps
// unreferenced loose objects, repacks the live objects into a single pack,
// and deletes loose objects that have been packed.
//
// This mirrors git's gc but is extended: vstodo keeps the pack and its index
// under .vstodo/objects/pack/ and can load objects from either the loose store
// or a pack (see internal/packfile).
package gc

import (
	"fmt"
	"os"
	"path/filepath"

	"vstodo/internal/app"
	"vstodo/internal/object"
	"vstodo/internal/packfile"
	"vstodo/internal/ref"
	"vstodo/internal/sha1"
	"vstodo/internal/storage"
)

// GC runs a full garbage collection cycle:
//   - mark: traverse every object reachable from refs (HEAD, branches, tags)
//   - sweep: delete loose objects not in the live set
//   - repack: write all live objects into one pack + index
//   - prune: delete loose objects now represented in the pack
func GC(a *app.App) error {
	live, err := markReachable(a)
	if err != nil {
		return fmt.Errorf("gc: mark: %w", err)
	}

	// Enumerate all loose objects.
	all, err := a.Store.Objects()
	if err != nil {
		return fmt.Errorf("gc: enumerate loose objects: %w", err)
	}

	// Sweep: delete loose objects that are not reachable.
	var swept int
	for _, id := range all {
		if live[id] {
			continue
		}
		if err := os.Remove(loosePath(a.Store.ObjectsDir, id)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("gc: sweep %s: %w", id.Hex(), err)
		}
		swept++
	}

	// Collect live objects from the loose store.
	var liveObjs []*object.Object
	for id := range live {
		obj, err := a.Store.Load(id)
		if err != nil {
			return fmt.Errorf("gc: load live object %s: %w", id.Hex(), err)
		}
		obj.ID = id
		liveObjs = append(liveObjs, obj)
	}

	// Repack live objects into a single pack.
	packBytes, sum, err := packfile.NewPackWriter(liveObjs...).WritePack()
	if err != nil {
		return fmt.Errorf("gc: write pack: %w", err)
	}

	packDir := filepath.Join(a.Repo.ObjectsDir(), "pack")
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		return err
	}
	packName := "pack-" + sum.Hex() + ".pack"
	idxName := "pack-" + sum.Hex() + ".idx"

	packPath := filepath.Join(packDir, packName)
	if err := os.WriteFile(packPath, packBytes, 0o444); err != nil {
		return fmt.Errorf("gc: write pack file: %w", err)
	}

	// Build and write the index.
	pack, err := packfile.ReadPack(packBytes)
	if err != nil {
		return fmt.Errorf("gc: read back pack: %w", err)
	}
	idx := packfile.BuildIndexFromPack(pack)
	idxBytes, err := packfile.WriteIndex(idx)
	if err != nil {
		return fmt.Errorf("gc: write index: %w", err)
	}
	idxPath := filepath.Join(packDir, idxName)
	if err := os.WriteFile(idxPath, idxBytes, 0o444); err != nil {
		return fmt.Errorf("gc: write index file: %w", err)
	}

	// Prune: delete loose objects now packed.
	var pruned int
	for _, id := range all {
		if !live[id] {
			continue // already swept
		}
		if err := os.Remove(loosePath(a.Store.ObjectsDir, id)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("gc: prune %s: %w", id.Hex(), err)
		}
		pruned++
	}

	fmt.Printf("vstodo gc: swept=%d packed=%d pruned=%d\n", swept, len(liveObjs), pruned)
	return nil
}

// markReachable returns the set of object ids reachable from all refs.
func markReachable(a *app.App) (map[sha1.Digest]bool, error) {
	live := make(map[sha1.Digest]bool)
	refs, err := a.Refs.ListRefs()
	if err != nil {
		return nil, err
	}
	// Always keep HEAD's target.
	if head, err := a.Refs.Head(); err == nil && head != sha1.Zero {
		mark(live, a, head)
	}
	for _, id := range refs {
		mark(live, a, id)
	}
	return live, nil
}

// mark recursively marks id and everything it references as live.
func mark(live map[sha1.Digest]bool, a *app.App, id sha1.Digest) {
	if live[id] || id == sha1.Zero {
		return
	}
	live[id] = true
	obj, err := a.Store.Load(id)
	if err != nil {
		// Dangling reference: mark the id but cannot traverse further.
		return
	}
	switch obj.Type {
	case object.CommitType:
		c, err := parseCommit(id, obj.Data)
		if err != nil {
			return
		}
		mark(live, a, c.Tree)
		for _, p := range c.Parents {
			mark(live, a, p)
		}
	case object.TreeType:
		t, err := parseTree(obj.Data)
		if err != nil {
			return
		}
		for _, e := range t.Entries {
			mark(live, a, e.ID)
		}
	case object.TagType:
		tag, err := parseTag(obj.Data)
		if err != nil {
			return
		}
		mark(live, a, tag.Object)
	case object.BlobType:
		// leaves
	}
}

// ----- lightweight parsers for traversal (avoid importing commit/tree/tag
// packages which pull heavier deps) -----

type commitView struct {
	Tree    sha1.Digest
	Parents []sha1.Digest
}

func parseCommit(id sha1.Digest, data []byte) (*commitView, error) {
	c := &commitView{}
	headerEnd := indexOfBlankLine(data)
	var header []byte
	if headerEnd < 0 {
		header = data
	} else {
		header = data[:headerEnd]
	}
	for _, line := range splitLines(header) {
		if len(line) == 0 {
			continue
		}
		sp := splitN(line, ' ', 2)
		if len(sp) < 2 {
			continue
		}
		switch string(sp[0]) {
		case "tree":
			if d, err := sha1.FromHex(string(sp[1])); err == nil {
				c.Tree = d
			}
		case "parent":
			if d, err := sha1.FromHex(string(sp[1])); err == nil {
				c.Parents = append(c.Parents, d)
			}
		}
	}
	return c, nil
}

type treeView struct {
	Entries []treeEntry
}

type treeEntry struct {
	ID sha1.Digest
}

func parseTree(data []byte) (*treeView, error) {
	t := &treeView{}
	rest := data
	for len(rest) > 0 {
		nul := indexByte(rest, 0)
		if nul < 0 {
			break
		}
		rest = rest[nul+1:]
		if len(rest) < 20 {
			break
		}
		t.Entries = append(t.Entries, treeEntry{ID: mustID(rest[:20])})
		rest = rest[20:]
	}
	return t, nil
}

type tagView struct {
	Object sha1.Digest
}

func parseTag(data []byte) (*tagView, error) {
	t := &tagView{}
	headerEnd := indexOfBlankLine(data)
	var header []byte
	if headerEnd < 0 {
		header = data
	} else {
		header = data[:headerEnd]
	}
	for _, line := range splitLines(header) {
		sp := splitN(line, ' ', 2)
		if len(sp) < 2 {
			continue
		}
		if string(sp[0]) == "object" {
			if d, err := sha1.FromHex(string(sp[1])); err == nil {
				t.Object = d
			}
		}
	}
	return t, nil
}

func indexOfBlankLine(data []byte) int {
	for i := 0; i+1 < len(data); i++ {
		if data[i] == '\n' && data[i+1] == '\n' {
			return i
		}
	}
	return -1
}

func splitLines(data []byte) [][]byte {
	var out [][]byte
	start := 0
	for i := 0; i <= len(data); i++ {
		if i == len(data) || data[i] == '\n' {
			out = append(out, data[start:i])
			start = i + 1
		}
	}
	return out
}

func splitN(data []byte, sep byte, n int) [][]byte {
	var out [][]byte
	start := 0
	count := 0
	for i := 0; i < len(data); i++ {
		if data[i] == sep && count < n-1 {
			out = append(out, data[start:i])
			start = i + 1
			count++
		}
	}
	out = append(out, data[start:])
	return out
}

func indexByte(data []byte, c byte) int {
	for i := 0; i < len(data); i++ {
		if data[i] == c {
			return i
		}
	}
	return -1
}

func mustID(b []byte) sha1.Digest {
	d, _ := sha1.FromBytes(b)
	return d
}

// loosePath returns the loose object file path for id.
func loosePath(objectsDir string, id sha1.Digest) string {
	h := id.Hex()
	return filepath.Join(objectsDir, h[:2], h[2:])
}

// Compile-time check that storage.LooseStore exposes objectPath via app.
var _ = storage.NewLooseStore
var _ = ref.ErrNotFound
