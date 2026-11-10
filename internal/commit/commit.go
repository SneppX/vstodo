// Package commit builds and parses commit objects (git byte-compatible).
// Commit format:
//
//	tree <hex>
//	parent <hex>   (zero or more)
//	author <name> <email> <ts> <tz>
//	committer <name> <email> <ts> <tz>
//
//	<message>
package commit

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

// Commit is a decoded commit object.
type Commit struct {
	Tree        sha1.Digest
	Parents     []sha1.Digest
	Author      string
	AuthorTS    int64
	AuthorTZ    string
	Committer   string
	CommitterTS int64
	CommitterTZ string
	Message     string
	raw         []byte
}

// Parse decodes commit bytes.
func Parse(id sha1.Digest, data []byte) (*Commit, error) {
	c := &Commit{raw: data}
	// Split header from message at first blank line.
	headerEnd := bytes.Index(data, []byte{'\n', '\n'})
	var header, msg []byte
	if headerEnd < 0 {
		header = data
		msg = nil
	} else {
		header = data[:headerEnd]
		msg = data[headerEnd+2:]
	}
	lines := bytes.Split(header, []byte{'\n'})
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		sp := bytes.SplitN(line, []byte{' '}, 2)
		if len(sp) < 2 {
			continue
		}
		key := string(sp[0])
		val := string(sp[1])
		switch key {
		case "tree":
			d, err := sha1.FromHex(val)
			if err != nil {
				return nil, fmt.Errorf("commit: bad tree %q", val)
			}
			c.Tree = d
		case "parent":
			d, err := sha1.FromHex(val)
			if err != nil {
				return nil, fmt.Errorf("commit: bad parent %q", val)
			}
			c.Parents = append(c.Parents, d)
		case "author":
			c.Author = val
		case "committer":
			c.Committer = val
		}
	}
	// message is everything after the blank line; git strips a single leading newline
	c.Message = strings.TrimRight(string(msg), "\n")
	return c, nil
}

// Marshal encodes the commit back to bytes (not recomputing ID).
func (c *Commit) Marshal() []byte {
	var buf bytes.Buffer
	buf.WriteString("tree ")
	buf.WriteString(c.Tree.Hex())
	buf.WriteByte('\n')
	for _, p := range c.Parents {
		buf.WriteString("parent ")
		buf.WriteString(p.Hex())
		buf.WriteByte('\n')
	}
	buf.WriteString("author ")
	buf.WriteString(c.Author)
	buf.WriteByte('\n')
	buf.WriteString("committer ")
	buf.WriteString(c.Committer)
	buf.WriteByte('\n')
	buf.WriteByte('\n')
	buf.WriteString(c.Message)
	buf.WriteByte('\n')
	return buf.Bytes()
}

// ID returns the content-addressed digest.
func (c *Commit) ID() sha1.Digest { return object.HashCommit(c.Marshal()) }

// Build constructs commit bytes from fields.
func Build(tree sha1.Digest, parents []sha1.Digest, author, committer, message string) []byte {
	var buf bytes.Buffer
	buf.WriteString("tree ")
	buf.WriteString(tree.Hex())
	buf.WriteByte('\n')
	for _, p := range parents {
		buf.WriteString("parent ")
		buf.WriteString(p.Hex())
		buf.WriteByte('\n')
	}
	now := time.Now().Unix()
	tz := formatTZ(now)
	authorLine := fmt.Sprintf("author %s <%s> %d %s", author, author, now, tz)
	committerLine := fmt.Sprintf("committer %s <%s> %d %s", committer, committer, now, tz)
	buf.WriteString(authorLine)
	buf.WriteByte('\n')
	buf.WriteString(committerLine)
	buf.WriteByte('\n')
	buf.WriteByte('\n')
	buf.WriteString(message)
	buf.WriteByte('\n')
	return buf.Bytes()
}

func formatTZ(ts int64) string {
	loc := time.Now().Location()
	t := time.Unix(ts, 0).In(loc)
	_, off := t.Zone()
	sign := "+"
	if off < 0 {
		sign = "-"
		off = -off
	}
	h := off / 3600
	m := (off % 3600) / 60
	return fmt.Sprintf("%s%02d%02d", sign, h, m)
}

// Ensure strconv used (kept for potential future parsing).
var _ = strconv.Itoa
