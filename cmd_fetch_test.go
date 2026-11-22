package main

import (
	"bytes"
	"compress/zlib"
	cryptosha1 "crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"vstodo/internal/app"
	"vstodo/internal/object"
	"vstodo/internal/ref"
	"vstodo/internal/sha1"
)

func fetchPacket(payload []byte) []byte {
	body := append([]byte{1}, payload...)
	return append([]byte(fmt.Sprintf("%04x", len(body)+4)), body...)
}

func TestFetchRemoteStoresPackObjectsAndTrackingRefs(t *testing.T) {
	blob := &object.Object{Type: object.BlobType, Data: []byte("fetched payload")}
	commit := &object.Object{Type: object.CommitType, Data: []byte("tree 1111111111111111111111111111111111111111\nauthor Test <test@example.com> 1 +0000\ncommitter Test <test@example.com> 1 +0000\n\nremote commit\n")}
	packBytes := makeFetchPack(t, blob, commit)
	blobID := object.ComputeID(blob.Type, blob.Data)
	commitID := object.ComputeID(commit.Type, commit.Data)
	advertisement := packetLineForFetch("# service=git-upload-pack\n") + "0000" +
		packetLineForFetch(commitID.Hex()+" HEAD\x00multi_ack side-band-64k ofs-delta symref=HEAD:refs/heads/main\n") +
		packetLineForFetch(commitID.Hex()+" refs/heads/main\n") +
		packetLineForFetch(blobID.Hex()+" refs/heads/feature\n") + "0000"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repo.git/info/refs":
			w.Header().Set("Content-Type", "application/x-git-upload-pack-advertisement")
			_, _ = w.Write([]byte(advertisement))
		case r.Method == http.MethodPost && r.URL.Path == "/repo.git/git-upload-pack":
			w.Header().Set("Content-Type", "application/x-git-upload-pack-result")
			_, _ = w.Write(fetchPacket(packBytes))
			_, _ = w.Write([]byte("0000"))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	repository, err := app.Init(filepath.Join(t.TempDir(), "checkout"))
	if err != nil {
		t.Fatal(err)
	}
	if err := fetchRemote(repository, "origin", server.URL+"/repo.git"); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []*object.Object{blob, commit} {
		stored, err := repository.Store.Load(object.ComputeID(expected.Type, expected.Data))
		if err != nil {
			t.Fatal(err)
		}
		if stored.Type != expected.Type || !bytes.Equal(stored.Data, expected.Data) {
			t.Fatalf("stored object mismatch for %s", expected.Type)
		}
	}
	for name, expected := range map[string]string{
		"refs/remotes/origin/main":    commitID.Hex(),
		"refs/remotes/origin/feature": blobID.Hex(),
		"refs/remotes/origin/HEAD":    commitID.Hex(),
	} {
		got, err := repository.Refs.ReadRef(name)
		if err != nil {
			t.Fatal(err)
		}
		if got.Hex() != expected {
			t.Fatalf("%s = %s, want %s", name, got.Hex(), expected)
		}
	}
}

func packetLineForFetch(value string) string {
	return fmt.Sprintf("%04x%s", len(value)+4, value)
}

func makeFetchPack(t *testing.T, objects ...*object.Object) []byte {
	t.Helper()
	var pack bytes.Buffer
	pack.WriteString("PACK")
	if err := binary.Write(&pack, binary.BigEndian, uint32(2)); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(&pack, binary.BigEndian, uint32(len(objects))); err != nil {
		t.Fatal(err)
	}
	for _, obj := range objects {
		raw := obj.Data
		size := len(raw)
		first := byte(0)
		switch obj.Type {
		case object.CommitType:
			first = 1 << 4
		case object.TreeType:
			first = 2 << 4
		case object.BlobType:
			first = 3 << 4
		case object.TagType:
			first = 4 << 4
		default:
			t.Fatalf("unsupported test object type %q", obj.Type)
		}
		first |= byte(size & 0x0f)
		size >>= 4
		if size > 0 {
			first |= 0x80
		}
		pack.WriteByte(first)
		for size > 0 {
			next := byte(size & 0x7f)
			size >>= 7
			if size > 0 {
				next |= 0x80
			}
			pack.WriteByte(next)
		}
		compressor := zlib.NewWriter(&pack)
		if _, err := compressor.Write(raw); err != nil {
			t.Fatal(err)
		}
		if err := compressor.Close(); err != nil {
			t.Fatal(err)
		}
	}
	checksum := cryptosha1.Sum(pack.Bytes())
	pack.Write(checksum[:])
	return pack.Bytes()
}

func TestFetchTrackingRefRejectsStaleSnapshot(t *testing.T) {
	a, err := app.Init(filepath.Join(t.TempDir(), "checkout"))
	if err != nil {
		t.Fatal(err)
	}
	name := "refs/remotes/origin/main"
	oldID := sha1.Digest{1}
	newerID := sha1.Digest{2}
	staleID := sha1.Digest{3}
	if err := a.Refs.UpdateRef(name, nil, oldID); err != nil {
		t.Fatal(err)
	}
	expected, err := readRefExpectation(a, name)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Refs.UpdateRef(name, &oldID, newerID); err != nil {
		t.Fatal(err)
	}
	err = updateFetchedTrackingRef(a, fetchedTrackingRef{name: name, id: staleID, expected: expected})
	if !errors.Is(err, ref.ErrReferenceChanged) {
		t.Fatalf("stale fetch update error = %v, want ErrReferenceChanged", err)
	}
	got, err := a.Refs.ReadRef(name)
	if err != nil {
		t.Fatal(err)
	}
	if got != newerID {
		t.Fatalf("stale fetch overwrote newer ref: got %s, want %s", got.Hex(), newerID.Hex())
	}
}
