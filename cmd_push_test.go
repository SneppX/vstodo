package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
	"vstodo/internal/tree"
)

func pushPacketLine(value string) string { return fmt.Sprintf("%04x%s", len(value)+4, value) }

func TestCmdPushPublishesFastForwardBranch(t *testing.T) {
	a, err := app.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	blob := &object.Object{Type: object.BlobType, Data: []byte("payload\n")}
	blobID, err := a.Store.Store(blob)
	if err != nil {
		t.Fatal(err)
	}
	treeData := tree.Build([]tree.Entry{{Mode: "100644", Name: "file.txt", ID: blobID}})
	treeID, err := a.Store.Store(&object.Object{Type: object.TreeType, Data: treeData})
	if err != nil {
		t.Fatal(err)
	}
	identity := "Ammar <ammar@example.test>"
	oldData := commit.Build(treeID, nil, identity, identity, "initial")
	oldID := object.HashCommit(oldData)
	if _, err := a.Store.Store(&object.Object{Type: object.CommitType, Data: oldData}); err != nil {
		t.Fatal(err)
	}
	newData := commit.Build(treeID, []sha1.Digest{oldID}, identity, identity, "next")
	newID := object.HashCommit(newData)
	if _, err := a.Store.Store(&object.Object{Type: object.CommitType, Data: newData}); err != nil {
		t.Fatal(err)
	}
	if err := a.Refs.WriteRef("refs/heads/main", newID); err != nil {
		t.Fatal(err)
	}
	if err := a.Refs.SetHead("ref: refs/heads/main"); err != nil {
		t.Fatal(err)
	}

	var pushed bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repo/info/refs":
			if r.Method != http.MethodGet || r.URL.Query().Get("service") != "git-receive-pack" {
				t.Fatalf("unexpected discovery request: %s %s", r.Method, r.URL.String())
			}
			body := pushPacketLine("# service=git-receive-pack\n") + "0000" +
				pushPacketLine(oldID.Hex()+" HEAD\x00symref=HEAD:refs/heads/main\n") +
				pushPacketLine(oldID.Hex()+" refs/heads/main\n") + "0000"
			w.Header().Set("Content-Type", "application/x-git-receive-pack-advertisement")
			_, _ = io.WriteString(w, body)
		case "/repo/git-receive-pack":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected push method: %s", r.Method)
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(body), oldID.Hex()+" "+newID.Hex()+" refs/heads/main\x00report-status") {
				t.Fatalf("push request omitted expected ref update: %q", body[:min(len(body), 300)])
			}
			pushed = true
			w.Header().Set("Content-Type", "application/x-git-receive-pack-result")
			_, _ = io.WriteString(w, pushPacketLine("unpack ok\n")+pushPacketLine("ok refs/heads/main\n")+"0000")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	if err := cmdRemote(a, []string{"add", "upstream", "https://fetch.example.test/team/repo.git"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := remoteConfig(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Set("remote.upstream.pushurl", server.URL+"/repo"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Set("branch.main.pushRemote", "upstream"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if err := cmdPush(a, nil); err != nil {
		t.Fatal(err)
	}
	if !pushed {
		t.Fatal("remote did not receive push request")
	}
	tracking, err := a.Refs.ReadRef("refs/remotes/upstream/main")
	if err != nil || tracking != newID {
		t.Fatalf("tracking ref = %s, err=%v; want %s", tracking.Hex(), err, newID.Hex())
	}
}

func TestCollectReachableObjectsSkipsUnreachableObjects(t *testing.T) {
	a, err := app.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	blob := &object.Object{Type: object.BlobType, Data: []byte("reachable")}
	blobID, err := a.Store.Store(blob)
	if err != nil {
		t.Fatal(err)
	}
	treeData := tree.Build([]tree.Entry{{Mode: "100644", Name: "file", ID: blobID}})
	treeID, err := a.Store.Store(&object.Object{Type: object.TreeType, Data: treeData})
	if err != nil {
		t.Fatal(err)
	}
	identity := "Ammar <ammar@example.test>"
	commitData := commit.Build(treeID, nil, identity, identity, "reachable commit")
	commitID := object.HashCommit(commitData)
	if _, err := a.Store.Store(&object.Object{Type: object.CommitType, Data: commitData}); err != nil {
		t.Fatal(err)
	}
	unreachableID, err := a.Store.Store(&object.Object{Type: object.BlobType, Data: []byte("unreachable")})
	if err != nil {
		t.Fatal(err)
	}
	objects, err := collectReachableObjects(a, commitID)
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[sha1.Digest]bool)
	for _, obj := range objects {
		seen[obj.ID] = true
	}
	for _, id := range []sha1.Digest{blobID, treeID, commitID} {
		if !seen[id] {
			t.Errorf("reachable object %s omitted", id.Hex())
		}
	}
	if seen[unreachableID] {
		t.Errorf("unreachable object %s included", unreachableID.Hex())
	}
}

func TestCmdPushRejectsNonFastForwardRemoteHistory(t *testing.T) {
	a, err := app.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	blobID, err := a.Store.Store(&object.Object{Type: object.BlobType, Data: []byte("local")})
	if err != nil {
		t.Fatal(err)
	}
	treeID, err := a.Store.Store(&object.Object{Type: object.TreeType, Data: tree.Build([]tree.Entry{{Mode: "100644", Name: "local.txt", ID: blobID}})})
	if err != nil {
		t.Fatal(err)
	}
	identity := "Ammar <ammar@example.test>"
	localData := commit.Build(treeID, nil, identity, identity, "local")
	localID := object.HashCommit(localData)
	if _, err := a.Store.Store(&object.Object{Type: object.CommitType, Data: localData}); err != nil {
		t.Fatal(err)
	}
	if err := a.Refs.WriteRef("refs/heads/main", localID); err != nil {
		t.Fatal(err)
	}
	if err := a.Refs.SetHead("ref: refs/heads/main"); err != nil {
		t.Fatal(err)
	}
	remoteID := sha1.Digest{9, 9, 9, 9}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			t.Fatal("non-fast-forward push reached receive-pack")
		}
		body := pushPacketLine("# service=git-receive-pack\n") + "0000" + pushPacketLine(remoteID.Hex()+" refs/heads/main\n") + "0000"
		w.Header().Set("Content-Type", "application/x-git-receive-pack-advertisement")
		_, _ = io.WriteString(w, body)
	}))
	defer server.Close()
	if err := cmdRemote(a, []string{"add", "origin", server.URL + "/repo"}); err != nil {
		t.Fatal(err)
	}
	if err := cmdPush(a, nil); err == nil || !strings.Contains(err.Error(), "fast-forward") {
		t.Fatalf("expected non-fast-forward rejection, got %v", err)
	}
}
