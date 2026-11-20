package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vstodo/internal/app"
	"vstodo/internal/config"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

func writeGitObject(t *testing.T, gitDir string, typ object.Type, data []byte) string {
	t.Helper()
	compressed, err := object.Compress(typ, data)
	if err != nil { t.Fatal(err) }
	id := object.ComputeID(typ, data).Hex()
	path := filepath.Join(gitDir, "objects", id[:2], id[2:])
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(path, compressed, 0o444); err != nil { t.Fatal(err) }
	return id
}

func makeGitRepo(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(filepath.Join(gitDir, "refs", "heads"), 0o755); err != nil { t.Fatal(err) }
	if err := os.MkdirAll(filepath.Join(gitDir, "refs", "tags"), 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil { t.Fatal(err) }
	config := "[core]\n\trepositoryformatversion = 0\n\tfilemode = true\n\n[user]\n\tname = Git User\n\temail = git@example.test\n\n[remote \"origin\"]\n\turl = https://example.test/team/project.git\n\tfetch = +refs/heads/*:refs/remotes/origin/*\n\n[branch \"main\"]\n\tremote = origin\n\tmerge = refs/heads/main\n"
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(config), 0o644); err != nil { t.Fatal(err) }
	blobID := writeGitObject(t, gitDir, object.BlobType, []byte("tracked file\n"))
	treeData := []byte("100644 file.txt\x00")
	blobBytes := make([]byte, 20)
	for i := 0; i < 20; i++ {
		var v byte
		for _, c := range blobID[i*2:i*2+2] {
			v <<= 4
			if c >= '0' && c <= '9' { v |= byte(c-'0') } else { v |= byte(c-'a'+10) }
		}
		blobBytes[i] = v
	}
	treeData = append(treeData, blobBytes...)
	treeID := writeGitObject(t, gitDir, object.TreeType, treeData)
	commitData := []byte("tree " + treeID + "\nauthor Git User <git@example.test> 1 +0000\ncommitter Git User <git@example.test> 1 +0000\n\ninitial commit\n")
	commitID := writeGitObject(t, gitDir, object.CommitType, commitData)
	if err := os.WriteFile(filepath.Join(gitDir, "refs", "heads", "main"), []byte(commitID+"\n"), 0o644); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(gitDir, "packed-refs"), []byte("# pack-refs with: peeled fully-peeled\n"+commitID+" refs/tags/v1.0\n"), 0o644); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(gitDir, "index"), []byte("git-index-snapshot"), 0o644); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("working copy remains unchanged\n"), 0o644); err != nil { t.Fatal(err) }
	return root, commitID
}

func TestAdoptPreservesGitAndImportsHistoryAndRemotes(t *testing.T) {
	root, commitID := makeGitRepo(t)
	gitConfigBefore, _ := os.ReadFile(filepath.Join(root, ".git", "config"))
	gitIndexBefore, _ := os.ReadFile(filepath.Join(root, ".git", "index"))
	workingBefore, _ := os.ReadFile(filepath.Join(root, "file.txt"))

	report, err := Adopt(root)
	if err != nil { t.Fatal(err) }
	if report.LooseObjects != 3 || report.References != 2 || report.Remotes != 1 || report.Branch != "main" {
		t.Fatalf("unexpected adoption report: %+v", report)
	}
	a, err := app.New(root)
	if err != nil { t.Fatal(err) }
	head, err := a.Refs.Head()
	if err != nil || head.Hex() != commitID { t.Fatalf("HEAD = %s, err=%v", head.Hex(), err) }
	for _, name := range []string{"refs/heads/main", "refs/tags/v1.0"} {
		if _, err := a.Refs.ReadRef(name); err != nil { t.Fatalf("missing %s: %v", name, err) }
	}
	cfg, err := remoteConfigForTest(a)
	if err != nil { t.Fatal(err) }
	if cfg["remote.origin.url"] != "https://example.test/team/project.git" ||
		cfg["remote.origin.fetch"] != "+refs/heads/*:refs/remotes/origin/*" ||
		cfg["branch.main.remote"] != "origin" || cfg["user.name"] != "Git User" {
		t.Fatalf("Git configuration was not migrated: %#v", cfg)
	}
	if _, err := a.Store.Load(mustDigest(t, commitID)); err != nil { t.Fatal(err) }
	configAfter, _ := os.ReadFile(filepath.Join(root, ".git", "config"))
	indexAfter, _ := os.ReadFile(filepath.Join(root, ".git", "index"))
	workingAfter, _ := os.ReadFile(filepath.Join(root, "file.txt"))
	if string(configAfter) != string(gitConfigBefore) || string(indexAfter) != string(gitIndexBefore) || string(workingAfter) != string(workingBefore) {
		t.Fatal("adoption modified Git metadata or the working tree")
	}
	if _, err := os.Stat(filepath.Join(root, ".vstodo", "migration", "git-config.backup")); err != nil { t.Fatal(err) }
	if _, err := os.Stat(filepath.Join(root, ".vstodo", "migration", "git-index.backup")); err != nil { t.Fatal(err) }
}

func TestAdoptRefusesExistingVSTodo(t *testing.T) {
	root, _ := makeGitRepo(t)
	if err := os.Mkdir(filepath.Join(root, ".vstodo"), 0o755); err != nil { t.Fatal(err) }
	if _, err := Adopt(root); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected existing repository refusal, got %v", err)
	}
}

func TestAdoptRefusesGitAlternatesAndSHA256(t *testing.T) {
	t.Run("alternates", func(t *testing.T) {
		root, _ := makeGitRepo(t)
		path := filepath.Join(root, ".git", "objects", "info", "alternates")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { t.Fatal(err) }
		if err := os.WriteFile(path, []byte("../shared/objects\n"), 0o644); err != nil { t.Fatal(err) }
		if _, err := Adopt(root); err == nil || !strings.Contains(err.Error(), "alternates") { t.Fatalf("got %v", err) }
		if _, err := os.Stat(filepath.Join(root, ".vstodo")); !os.IsNotExist(err) { t.Fatal("created VSTodo metadata on rejected adoption") }
	})
	t.Run("sha256", func(t *testing.T) {
		root, _ := makeGitRepo(t)
		path := filepath.Join(root, ".git", "config")
		data, _ := os.ReadFile(path)
		data = append(data, []byte("\n[extensions]\n\tobjectFormat = sha256\n")...)
		if err := os.WriteFile(path, data, 0o644); err != nil { t.Fatal(err) }
		if _, err := Adopt(root); err == nil || !strings.Contains(err.Error(), "SHA-256") { t.Fatalf("got %v", err) }
	})
}

func mustDigest(t *testing.T, value string) sha1.Digest {
	t.Helper()
	d, err := sha1.FromHex(value)
	if err != nil { t.Fatal(err) }
	return d
}

func remoteConfigForTest(a *app.App) (map[string]string, error) {
	cfg := config.New(a.Repo.GitPath("config"))
	if err := cfg.Load(); err != nil { return nil, err }
	return cfg.All(), nil
}
