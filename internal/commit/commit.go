// Package commit builds and parses Git-compatible commit objects.
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

// Commit is a decoded commit object. Author and Committer contain the identity
// portion of their respective headers, without the timestamp and timezone.
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

// Parse decodes a Git commit object while retaining its original bytes. Keeping
// the original representation preserves unknown headers, signatures, header
// ordering, and the exact message when the object is marshalled again.
func Parse(id sha1.Digest, data []byte) (*Commit, error) {
	_ = id // The caller's object store is responsible for verifying the object ID.
	c := &Commit{raw: append([]byte(nil), data...)}
	headerEnd := bytes.Index(data, []byte{'\\n', '\\n'})
	var header, message []byte
	if headerEnd < 0 {
		header = data
	} else {
		header = data[:headerEnd]
		message = data[headerEnd+2:]
	}
	for _, line := range bytes.Split(header, []byte{'\\n'}) {
		if len(line) == 0 || line[0] == ' ' {
			continue
		}
		parts := bytes.SplitN(line, []byte{' '}, 2)
		if len(parts) != 2 {
			continue
		}
		value := string(parts[1])
		switch string(parts[0]) {
		case "tree":
			d, err := sha1.FromHex(value)
			if err != nil {
				return nil, fmt.Errorf("commit: invalid tree ID %q: %w", value, err)
			}
			c.Tree = d
		case "parent":
			d, err := sha1.FromHex(value)
			if err != nil {
				return nil, fmt.Errorf("commit: invalid parent ID %q: %w", value, err)
			}
			c.Parents = append(c.Parents, d)
		case "author":
			c.Author, c.AuthorTS, c.AuthorTZ = parseIdentity(value)
		case "committer":
			c.Committer, c.CommitterTS, c.CommitterTZ = parseIdentity(value)
		}
	}
	if c.Tree == sha1.Zero {
		return nil, fmt.Errorf("commit: missing or zero tree ID")
	}
	if headerEnd >= 0 {
		c.Message = string(message)
	}
	return c, nil
}

// parseIdentity separates the identity, Unix timestamp, and timezone from a
// Git author/committer header. Malformed suffixes are retained as identity text
// rather than silently inventing metadata.
func parseIdentity(value string) (string, int64, string) {
	zoneSep := strings.LastIndexByte(value, ' ')
	if zoneSep < 0 {
		return value, 0, ""
	}
	tz := value[zoneSep+1:]
	timestampSep := strings.LastIndexByte(value[:zoneSep], ' ')
	if timestampSep < 0 {
		return value, 0, ""
	}
	ts, err := strconv.ParseInt(value[timestampSep+1:zoneSep], 10, 64)
	if err != nil {
		return value, 0, ""
	}
	return value[:timestampSep], ts, tz
}

// Marshal returns the exact original bytes for parsed commits. For newly
// constructed commits it serializes the fields in Git's canonical header order.
func (c *Commit) Marshal() []byte {
	if c.raw != nil {
		return append([]byte(nil), c.raw...)
	}
	var buf bytes.Buffer
	buf.WriteString("tree ")
	buf.WriteString(c.Tree.Hex())
	buf.WriteByte('\\n')
	for _, parent := range c.Parents {
		buf.WriteString("parent ")
		buf.WriteString(parent.Hex())
		buf.WriteByte('\\n')
	}
	buf.WriteString("author ")
	buf.WriteString(formatIdentity(c.Author, c.AuthorTS, c.AuthorTZ))
	buf.WriteByte('\\n')
	buf.WriteString("committer ")
	buf.WriteString(formatIdentity(c.Committer, c.CommitterTS, c.CommitterTZ))
	buf.WriteByte('\\n')
	buf.WriteByte('\\n')
	buf.WriteString(c.Message)
	if !strings.HasSuffix(c.Message, "\\n") {
		buf.WriteByte('\\n')
	}
	return buf.Bytes()
}

func formatIdentity(identity string, timestamp int64, timezone string) string {
	if timezone == "" {
		timezone = "+0000"
	}
	return fmt.Sprintf("%s %d %s", normalizeIdentity(identity), timestamp, timezone)
}

func normalizeIdentity(identity string) string {
	identity = strings.TrimSpace(identity)
	if strings.Contains(identity, "<") && strings.HasSuffix(identity, ">") {
		return identity
	}
	return fmt.Sprintf("%s <%s>", identity, identity)
}

// ID returns the content-addressed digest.
func (c *Commit) ID() sha1.Digest { return object.HashCommit(c.Marshal()) }

// Build constructs a Git-compatible commit object. An identity may be supplied
// as "Name <email>" or as a short name, in which case the name is also used as
// the email address.
func Build(tree sha1.Digest, parents []sha1.Digest, author, committer, message string) []byte {
	now := time.Now()
	timestamp := now.Unix()
	timezone := formatTZ(timestamp)
	c := &Commit{
		Tree:        tree,
		Parents:     append([]sha1.Digest(nil), parents...),
		Author:      normalizeIdentity(author),
		AuthorTS:    timestamp,
		AuthorTZ:    timezone,
		Committer:   normalizeIdentity(committer),
		CommitterTS: timestamp,
		CommitterTZ: timezone,
		Message:     message,
	}
	return c.Marshal()
}

func formatTZ(timestamp int64) string {
	_, offset := time.Unix(timestamp, 0).In(time.Local).Zone()
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	return fmt.Sprintf("%s%02d%02d", sign, offset/3600, (offset%3600)/60)
}
