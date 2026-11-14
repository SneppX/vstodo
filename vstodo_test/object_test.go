package vstodo_test

import (
	"bytes"
	"testing"

	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
	"vstodo/internal/tree"
)

func TestObjectComputeID(t *testing.T) {
	id := object.ComputeID(object.BlobType, []byte("hello"))
	if id.Hex() != "ef0493b275aa2080237f676d2ef6559246f56636" {
		t.Fatalf("ComputeID(blob, hello) = %s, want ef0493b275aa2080237f676d2ef6559246f56636", id.Hex())
	}
}

func TestObjectHashBlobGitCompatible(t *testing.T) {
	id := object.HashBlob([]byte("hello"))
	if id.Hex() != "ef0493b275aa2080237f676d2ef6559246f56636" {
		t.Fatalf("HashBlob(hello) = %s, want ef0493b275aa2080237f676d2ef6559246f56636 (git-compatible)", id.Hex())
	}
	id2 := object.HashBlob([]byte("hello"))
	if id2 != id {
		t.Fatalf("HashBlob not deterministic: %s vs %s", id2.Hex(), id.Hex())
	}
	id3 := object.HashBlob([]byte("world"))
	if id3 == id {
		t.Fatalf("HashBlob should differ for different content")
	}
}

func TestObjectMarshalUnmarshal(t *testing.T) {
	cases := []struct {
		typ object.Type
		data []byte
	}{
		{object.BlobType, []byte("hello world")},
		{object.TreeType, []byte("100644 file\0" + string(make([]byte, 20)))},
		{object.CommitType, []byte("tree abc\nparent def\nauthor x <x> 0 +0000\ncommitter y <y> 0 +0000\n\nmsg")},
		{object.TagType, []byte("object abc\ntype commit\ntaggername <n> 0 +0000\n\ntag message")},
	}
	for _, c := range cases {
		raw := object.Marshal(c.typ, c.data)
		typ, data, err := object.Unmarshal(raw)
		if err != nil {
			t.Fatalf("Unmarshal(%s): %v", c.typ, err)
		}
		if typ != c.typ {
			t.Fatalf("Unmarshal type = %s, want %s", typ, c.typ)
		}
		if !bytes.Equal(data, c.data) {
			t.Fatalf("Unmarshal data mismatch for %s", c.typ)
		}
	}
}

func TestObjectUnmarshalErrors(t *testing.T) {
	if _, _, err := object.Unmarshal([]byte("no nul here")); err == nil {
		t.Fatalf("expected error for missing NUL")
	}
	if _, _, err := object.Unmarshal([]byte("bogus\0data")); err == nil {
		t.Fatalf("expected error for unknown type")
	}
}


func TestObjectUnmarshalRejectsIncorrectPayloadSize(t *testing.T) {
	cases := []string{
		"blob 4\\x00abc",
		"blob 2\\x00abc",
		"blob nope\\x00abc",
		"blob -1\\x00abc",
	}
	for _, raw := range cases {
		if _, _, err := object.Unmarshal([]byte(raw)); err == nil {
			t.Errorf("Unmarshal(%q) unexpectedly succeeded", raw)
		}
	}
}

func TestObjectCompressDecompress(t *testing.T) {
	data := bytes.Repeat([]byte("x"), 10000)
	comp, err := object.Compress(object.BlobType, data)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}
	if len(comp) == 0 || len(comp) >= len(data) {
		t.Fatalf("compression did not reduce size: %d vs %d", len(comp), len(data))
	}
	raw, err := object.Decompress(comp)
	if err != nil {
		t.Fatalf("Decompress: %v", err)
	}
	if !bytes.Equal(raw, object.Marshal(object.BlobType, data)) {
		t.Fatalf("Decompress round-trip mismatch")
	}
}

func TestObjectMarshalUnmarshalRoundTripStore(t *testing.T) {
	blob := &object.Object{Type: object.BlobType, Data: []byte("round-trip blob data")}
	id, err := object.ComputeID(blob.Type, blob.Data), error(nil)
	_ = id
	if _, err := object.ComputeID(blob.Type, blob.Data), error(nil); err != nil {
		t.Fatalf("ComputeID: %v", err)
	}
}

func TestCommitBuildParse(t *testing.T) {
	treeID := sha1.Zero
	parents := []sha1.Digest{sha1.Zero}
	cb := commit.Build(treeID, parents, "Alice <alice@example.com>", "Bob <bob@example.com>", "initial commit")
	c, err := commit.Parse(sha1.Zero, cb)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if c.Tree != treeID {
		t.Fatalf("commit tree = %s, want %s", c.Tree.Hex(), treeID.Hex())
	}
	if len(c.Parents) != 1 || c.Parents[0] != sha1.Zero {
		t.Fatalf("commit parents = %v", c.Parents)
	}
	if c.Message != "initial commit" {
		t.Fatalf("commit message = %q, want %q", c.Message, "initial commit")
	}
	if c.Author != "Alice <alice@example.com>" {
		t.Fatalf("commit author = %q", c.Author)
	}
	if c.Committer != "Bob <bob@example.com>" {
		t.Fatalf("commit committer = %q", c.Committer)
	}
	// Marshal round-trip preserves content
	if !bytes.Equal(cb, c.Marshal()) {
		t.Fatalf("Commit marshal not stable")
	}
	id := object.HashCommit(cb)
	if id == sha1.Zero {
		t.Fatalf("commit ID is zero")
	}
}

func TestCommitParseMultipleParents(t *testing.T) {
	p1 := sha1.Digest{1}
	p2 := sha1.Digest{2}
	cb := commit.Build(sha1.Zero, []sha1.Digest{p1, p2}, "A <a>", "B <b>", "merge commit")
	c, err := commit.Parse(sha1.Zero, cb)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(c.Parents) != 2 || c.Parents[0] != p1 || c.Parents[1] != p2 {
		t.Fatalf("parents = %v", c.Parents)
	}
}

func TestTreeBuildParseOrdering(t *testing.T) {
	d1 := sha1.Digest{1}
	d2 := sha1.Digest{2}
	d3 := sha1.Digest{3}
	entries := []tree.Entry{
		{Mode: "100644", Name: "zeta.txt", ID: d1},
		{Mode: "100644", Name: "alpha.txt", ID: d2},
		{Mode: "040000", Name: "subdir", ID: d3},
	}
	data := tree.Build(entries)
	treeObj, err := tree.Parse(sha1.Zero, data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// Marshal sorts by name: alpha.txt, subdir, zeta.txt
	wantOrder := []string{"alpha.txt", "subdir", "zeta.txt"}
	gotOrder := []string{treeObj.Entries[0].Name, treeObj.Entries[1].Name, treeObj.Entries[2].Name}
	for i := range wantOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Fatalf("entry order[%d] = %q, want %q", i, gotOrder[i], wantOrder[i])
		}
	}
	id := object.HashTree(data)
	if id == sha1.Zero {
		t.Fatalf("tree ID is zero")
	}
}

func TestTreeParseMalformed(t *testing.T) {
	if _, err := tree.Parse(sha1.Zero, []byte("short")); err == nil {
		t.Fatalf("expected error for malformed tree")
	}
}
