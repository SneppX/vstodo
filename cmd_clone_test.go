package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"vstodo/internal/app"
	"vstodo/internal/object"
	"vstodo/internal/tree"
)

func TestCloneFromHTTPRemoteChecksOutDefaultBranch(t *testing.T) {
	blobObject := &object.Object{Type: object.BlobType, Data: []byte("nested payload\n")}
	blobID := object.ComputeID(blobObject.Type, blobObject.Data)
	innerTreeData := tree.Build([]tree.Entry{{Mode: "100644", Name: "README", ID: blobID}})
	innerTreeObject := &object.Object{Type: object.TreeType, Data: innerTreeData}
	innerTreeID := object.ComputeID(innerTreeObject.Type, innerTreeObject.Data)
	rootTreeData := tree.Build([]tree.Entry{{Mode: "40000", Name: "docs", ID: innerTreeID}})
	rootTreeObject := &object.Object{Type: object.TreeType, Data: rootTreeData}
	rootTreeID := object.ComputeID(rootTreeObject.Type, rootTreeObject.Data)
	commitObject := &object.Object{
		Type: object.CommitType,
		Data: []byte(fmt.Sprintf("tree %s\nauthor Test <test@example.com> 1 +0000\ncommitter Test <test@example.com> 1 +0000\n\nremote commit\n", rootTreeID.Hex())),
	}
	commitID := object.ComputeID(commitObject.Type, commitObject.Data)
	packBytes := makeFetchPack(t, blobObject, innerTreeObject, rootTreeObject, commitObject)
	advertisement := packetLineForFetch("# service=git-upload-pack\n") + "0000" +
		packetLineForFetch(commitID.Hex()+" HEAD\x00multi_ack side-band-64k ofs-delta symref=HEAD:refs/heads/main\n") +
		packetLineForFetch(commitID.Hex()+" refs/heads/main\n") + "0000"
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
	destination := filepath.Join(t.TempDir(), "clone")
	if err := cmdClone([]string{server.URL + "/repo.git", destination}); err != nil {
		t.Fatal(err)
	}
	cloned, err := app.New(destination)
	if err != nil {
		t.Fatal(err)
	}
	head, err := cloned.Refs.Head()
	if err != nil {
		t.Fatal(err)
	}
	if head != commitID {
		t.Fatalf("HEAD = %s, want %s", head.Hex(), commitID.Hex())
	}
	branch, err := cloned.Refs.CurrentBranch()
	if err != nil {
		t.Fatal(err)
	}
	if branch != "main" {
		t.Fatalf("current branch = %q, want main", branch)
	}
	remote, err := remoteConfig(cloned)
	if err != nil {
		t.Fatal(err)
	}
	if got := remote.Get("remote.origin.url"); got != server.URL+"/repo.git" {
		t.Fatalf("origin URL = %q", got)
	}
	content, err := os.ReadFile(filepath.Join(destination, "docs", "README"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "nested payload\n" {
		t.Fatalf("nested file content = %q", content)
	}
}
