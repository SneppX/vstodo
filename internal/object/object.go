package object

// Object is a vstodo/git object: blob, tree, commit, or tag.

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"strconv"
	"strings"

	"vstodo/internal/sha1"
)

// Type is the type of an object.
type Type string

const (
	BlobType   Type = "blob"
	TreeType   Type = "tree"
	CommitType Type = "commit"
	TagType    Type = "tag"
)

// validTypes is the set of object types vstodo supports.
var validTypes = map[Type]bool{
	BlobType:   true,
	TreeType:   true,
	CommitType: true,
	TagType:    true,
}

// Object holds decoded object content and its identity.
type Object struct {
	Type Type
	Data []byte
	ID   sha1.Digest
}

// ObjectStore persists objects content-addressed by SHA-1.
type Store interface {
	// Store writes obj and returns its digest.
	Store(obj *Object) (sha1.Digest, error)
	// Load returns the decoded object for id.
	Load(id sha1.Digest) (*Object, error)
	// Has returns true if id is present.
	Has(id sha1.Digest) (bool, error)
}

// ComputeID derives the git-compatible SHA-1 address of content.
func ComputeID(typ Type, data []byte) sha1.Digest {
	h := sha1.NewHasher()
	h.Write([]byte(typ))
	h.Write([]byte(" "))
	h.Write([]byte(fmt.Sprintf("%d", len(data))))
	h.Write([]byte{0})
	h.Write(data)
	return h.SumDigest()
}

// Marshal serializes an object into its wire/on-disk bytes: header + NUL + payload.
func Marshal(typ Type, data []byte) []byte {
	var buf bytes.Buffer
	buf.WriteString(string(typ))
	buf.WriteByte(' ')
	buf.WriteString(fmt.Sprintf("%d", len(data)))
	buf.WriteByte(0)
	buf.Write(data)
	return buf.Bytes()
}

// Unmarshal splits header bytes into type and payload.
func Unmarshal(raw []byte) (Type, []byte, error) {
	idx := bytes.IndexByte(raw, 0)
	if idx < 0 {
		return "", nil, fmt.Errorf("object: missing NUL separator in header")
	}
	header := string(raw[:idx])
	data := raw[idx+1:]
	sp := strings.SplitN(header, " ", 2)
	if len(sp) != 2 || sp[0] == "" || sp[1] == "" {
		return "", nil, fmt.Errorf("object: malformed header %q", header)
	}
	typ := Type(sp[0])
	if !validTypes[typ] {
		return "", nil, fmt.Errorf("object: unknown type %q", typ)
	}
	size, err := strconv.ParseUint(sp[1], 10, 64)
	if err != nil {
		return "", nil, fmt.Errorf("object: invalid payload size %q: %w", sp[1], err)
	}
	if size != uint64(len(data)) {
		return "", nil, fmt.Errorf("object: payload size mismatch: header says %d bytes, got %d", size, len(data))
	}
	return typ, data, nil
}

// Compress returns the zlib-compressed on-disk form of an object.
func Compress(typ Type, data []byte) ([]byte, error) {
	raw := Marshal(typ, data)
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write(raw); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decompress reverses Compress.
func Decompress(blob []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(blob))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// HashBlob computes the SHA-1 address of file content (git byte-identical).
func HashBlob(data []byte) sha1.Digest { return ComputeID(BlobType, data) }

// HashTree computes the SHA-1 address of a tree object.
func HashTree(data []byte) sha1.Digest { return ComputeID(TreeType, data) }

// HashCommit computes the SHA-1 address of a commit object.
func HashCommit(data []byte) sha1.Digest { return ComputeID(CommitType, data) }

// Blob represents a file-content object.
type Blob struct{ Data []byte }

// TreeEntry is one entry in a tree object.
type TreeEntry struct {
	Mode string // e.g. "100644", "100755", "040000", "120000"
	Name string
	ID   sha1.Digest
}

// Tree represents a directory object.
type Tree struct {
	Entries []TreeEntry
}

// Commit represents a snapshot object.
type Commit struct {
	Tree        sha1.Digest
	Parents     []sha1.Digest
	Author      string
	AuthorTS    int64
	Committer   string
	CommitterTS int64
	Message     string
}

// Tag represents an annotated tag object.
type Tag struct {
	Object   sha1.Digest
	Type     Type
	Tagger   string
	TaggerTS int64
	Message  string
}
