// Package walk traverses the commit graph and object graph.
package walk

import (
	"fmt"

	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

// Walker visits commits reachable from roots.
type Walker struct {
	Load func(sha1.Digest) (*object.Object, error)
}

// History walks the full commit graph reachable from start, following all
// parents in breadth-first order. The returned slice includes start when
// present and contains no duplicates.
func (w *Walker) History(start sha1.Digest) ([]sha1.Digest, error) {
	if w.Load == nil {
		return nil, fmt.Errorf("walk: nil Load")
	}
	var out []sha1.Digest
	seen := make(map[sha1.Digest]bool)
	queue := []sha1.Digest{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == sha1.Zero || seen[cur] {
			continue
		}
		seen[cur] = true
		out = append(out, cur)
		obj, err := w.Load(cur)
		if err != nil {
			return nil, err
		}
		c, err := commit.Parse(cur, obj.Data)
		if err != nil {
			return nil, err
		}
		for _, p := range c.Parents {
			if !seen[p] {
				queue = append(queue, p)
			}
		}
	}
	return out, nil
}
