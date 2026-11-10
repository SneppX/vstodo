// Package tree builds and parses tree objects.
// Tree entry: "<mode> <name>\0<20-byte hash>"
package tree

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

// Entry is a tree entry.
type Entry struct {
	Mode string // "100644", "100755", "040000", "120000"
	Name string
	ID   sha1.Digest
}

// Tree is a decoded tree object.
type Tree struct {
	Entries []Entry
	raw     []byte
}

// Parse decodes tree bytes.
func Parse(id sha1.Digest, data []byte) (*Tree, error) {
	t := &Tree{raw: data}
	rest := data
	for len(rest) > 0 {
		nul := bytes.IndexByte(rest, 0)
		if nul < 0 {
			return nil, fmt.Errorf("tree: malformed entry, missing NUL")
		}
		meta := rest[:nul]
		rest = rest[nul+1:]
		if len(rest) < 20 {
			return nil, fmt.Errorf("tree: truncated hash")
		}
		idBytes := rest[:20]
		rest = rest[20:]
		sp := strings.SplitN(string(meta), " ", 2)
		if len(sp) != 2 {
			return nil, fmt.Errorf("tree: malformed meta %q", meta)
		}
		d, err := sha1.FromBytes(idBytes)
		if err != nil {
			return nil, err
		}
		t.Entries = append(t.Entries, Entry{Mode: sp[0], Name: sp[1], ID: d})
	}
	return t, nil
}

// Marshal encodes the tree (entries sorted by name, git convention).
func (t *Tree) Marshal() []byte {
	ents := make([]Entry, len(t.Entries))
	copy(ents, t.Entries)
	sort.Slice(ents, func(i, j int) bool { return ents[i].Name < ents[j].Name })
	var buf bytes.Buffer
	for _, e := range ents {
		buf.WriteString(e.Mode)
		buf.WriteByte(' ')
		buf.WriteString(e.Name)
		buf.WriteByte(0)
		buf.Write(e.ID.Bytes())
	}
	return buf.Bytes()
}

// ID returns the content-addressed digest.
func (t *Tree) ID() sha1.Digest { return object.HashTree(t.Marshal()) }

// Build constructs a tree from entries.
func Build(entries []Entry) []byte {
	t := &Tree{Entries: entries}
	return t.Marshal()
}
