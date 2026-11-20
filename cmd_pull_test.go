package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
	"vstodo/internal/tree"
)

func TestCmdPullFetchesAndFastForwardsCurrentBranch(t *testing.T) {
	a, err := app.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	baseBlob := &object.Object{Type: object.BlobType, Data: []byte("base\n")}
	baseBlobID, err := a.Store.Store(baseBlob)
	if err != nil {
		t.Fatal(err)
	}
	baseTreeData := tree.Build([]tree.Entry{{Mode: "100644", Name: "base.txt", ID: baseBlobID}})
	baseTreeID, err := a.Store.Store(&object.Object{Type: object.TreeType, Data: baseTreeData})
	if err != nil {
		t.Fatal(err)
	}
	identity := "Ammar <ammar@example.test>"
	baseCommitData := commit.Build(baseTreeID, nil, identity, identity, "base")
	baseCommitID := object.HashCommit(baseCommitData)
	if _, err := a.Store.Store(&object.Object{Type: object.CommitType, Data: baseCommitData}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.Repo.Root, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.Index.Add("base.txt", baseBlobID)
	if err := a.SaveIndex(); err != nil {
		t.Fatal(err)
	}
	if err := a.Refs.WriteRef("refs/heads/main", baseCommitID); err != nil {
		t.Fatal(err)
	}
	if err := a.Refs.SetHead("ref: refs/heads/main"); err != nil {
		t.Fatal(err)
	}

	remoteBlob := &object.Object{Type: object.BlobType, Data: []byte("from remote\n")}
	remoteBlobID := object.ComputeID(remoteBlob.Type, remoteBlob.Data)
	remoteTreeData := tree.Build([]tree.Entry{
		{Mode: "100644", Name: "base.txt", ID: baseBlobID},
		{Mode: "100644", Name: "remote.txt", ID: remoteBlobID},
	})
	remoteTree := &object.Object{Type: object.TreeType, Data: remoteTreeData}
	remoteTreeID := object.ComputeID(remoteTree.Type, remoteTree.Data)
	remoteCommitData := commit.Build(remoteTreeID, []sha1.Digest{baseCommitID}, identity, identity, "remote update")
	remoteCommit := &object.Object{Type: object.CommitType, Data: remoteCommitData}
	remoteCommitID := object.ComputeID(remoteCommit.Type, remoteCommit.Data)
	packBytes := makeFetchPack(t, remoteBlob, remoteTree, remoteCommit)
	advertisement := packetLineForFetch("# service=git-upload-pack\n") + "0000" +
		packetLineForFetch(remoteCommitID.Hex()+" HEAD\x00symref=HEAD:refs/heads/develop\n") +
		packetLineForFetch(remoteCommitID.Hex()+" refs/heads/develop\n") + "0000"
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
	if err := cmdRemote(a, []string{"add", "upstream", server.URL + "/repo.git"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := remoteConfig(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Set("branch.main.remote", "upstream"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Set("branch.main.merge", "refs/heads/develop"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	if err := cmdPull(a, nil); err != nil {
		t.Fatal(err)
	}
	head, err := a.Refs.Head()
	if err != nil || head != remoteCommitID {
		t.Fatalf("HEAD = %s, err=%v; want %s", head.Hex(), err, remoteCommitID.Hex())
	}
	data, err := os.ReadFile(filepath.Join(a.Repo.Root, "remote.txt"))
	if err != nil || string(data) != "from remote\n" {
		t.Fatalf("remote.txt = %q, err=%v", data, err)
	}
	tracking, err := a.Refs.ReadRef("refs/remotes/upstream/develop")
	if err != nil || tracking != remoteCommitID {
		t.Fatalf("tracking ref = %s, err=%v", tracking.Hex(), err)
	}
}
