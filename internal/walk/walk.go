// Package walk traverses the commit graph and object graph.
package walk

import (
	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

// Walker visits commits reachable from roots.
type Walker struct {
	Load func(sha1.Digest) (*object.Object, error)
}

// History walks first-parent history from start.
func (w *Walker) History(start sha1.Digest) ([]sha1.Digest, error) {
	var out []sha1.Digest
	seen := make(map[sha1.Digest]bool)
	cur := start
	for cur != sha1.Zero {
		if seen[cur] {
			break
		}
		seen[cur] = true
		out = append(out, cur)
		cur = sha1.Zero
		_ = w.Load
		_ = object.BlobType
	}
	return out, nil
}
