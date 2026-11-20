package migrate

import (
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"vstodo/internal/app"
	"vstodo/internal/config"
	"vstodo/internal/object"
	"vstodo/internal/packfile"
	"vstodo/internal/sha1"
)

func writeGitObject(t *testing.T, gitDir string, typ object.Type, data []byte) string {
	t.Helper()
	compressed, err := object.Compress(typ, data)
	if err != nil {
		t.Fatal(err)
	}
	id := object.ComputeID(typ, data).Hex()
	path := filepath.Join(gitDir, "objects", id[:2], id[2:])
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, compressed, 0o444); err != nil {
		t.Fatal(err)
	}
	return id
}

func makeGitRepo(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(filepath.Join(gitDir, "refs", "heads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(gitDir, "refs", "tags"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(gitDir, "info"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "info", "exclude"), []byte("# user ignore rules\n*.local\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	gitConfig := "[core]\n\trepositoryformatversion = 0\n\tfilemode = true\n\n[user]\n\tname = Git User\n\temail = git@example.test\n\n[remote \"origin\"]\n\turl = https://example.test/team/project.git\n\tpushurl = https://push.example.test/team/project.git\n\tfetch = +refs/heads/*:refs/remotes/origin/*\n\n[branch \"main\"]\n\tremote = origin\n\tmerge = refs/heads/main\n\tpushRemote = origin\n"
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(gitConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	blobID := writeGitObject(t, gitDir, object.BlobType, []byte("tracked file\n"))
	blobBytes, err := hex.DecodeString(blobID)
	if err != nil {
		t.Fatal(err)
	}
	treeData := append([]byte("100644 file.txt\x00"), blobBytes...)
	treeID := writeGitObject(t, gitDir, object.TreeType, treeData)
	commitData := []byte("tree " + treeID + "\nauthor Git User <git@example.test> 1 +0000\ncommitter Git User <git@example.test> 1 +0000\n\ninitial commit\n")
	commitID := writeGitObject(t, gitDir, object.CommitType, commitData)
	if err := os.WriteFile(filepath.Join(gitDir, "refs", "heads", "main"), []byte(commitID+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "packed-refs"), []byte("# pack-refs with: peeled fully-peeled\n"+commitID+" refs/tags/v1.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "index"), []byte("git-index-snapshot"), 0o644); err != nil {
		t.Fatal(err)
	}
	packedObject := &object.Object{Type: object.BlobType, Data: []byte("packed object payload")}
	packedObject.ID = object.ComputeID(packedObject.Type, packedObject.Data)
	packBytes, _, err := packfile.NewPackWriter(packedObject).WritePack()
	if err != nil {
		t.Fatal(err)
	}
	packDir := filepath.Join(gitDir, "objects", "pack")
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packDir, "pack-test.pack"), packBytes, 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("working copy remains unchanged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, commitID
}

func TestAdoptPreservesGitAndImportsHistoryAndRemotes(t *testing.T) {
	root, commitID := makeGitRepo(t)
	gitConfigBefore, err := os.ReadFile(filepath.Join(root, ".git", "config"))
	if err != nil {
		t.Fatal(err)
	}
	gitIndexBefore, err := os.ReadFile(filepath.Join(root, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	workingBefore, err := os.ReadFile(filepath.Join(root, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}

	report, err := Adopt(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.LooseObjects != 3 || report.PackedObjects != 1 || report.References != 2 || report.Remotes != 1 || report.Branch != "main" || !report.GitExcludeUpdated {
		t.Fatalf("unexpected adoption report: %+v", report)
	}
	a, err := app.New(root)
	if err != nil {
		t.Fatal(err)
	}
	head, err := a.Refs.Head()
	if err != nil || head.Hex() != commitID {
		t.Fatalf("HEAD = %s, err=%v", head.Hex(), err)
	}
	for _, name := range []string{"refs/heads/main", "refs/tags/v1.0"} {
		if _, err := a.Refs.ReadRef(name); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
	cfg := config.New(a.Repo.GitPath("config"))
	if err := cfg.Load(); err != nil {
		t.Fatal(err)
	}
	values := cfg.All()
	if values["remote.origin.url"] != "https://example.test/team/project.git" ||
		values["remote.origin.fetch"] != "+refs/heads/*:refs/remotes/origin/*" ||
		values["remote.origin.pushurl"] != "https://push.example.test/team/project.git" ||
		values["branch.main.remote"] != "origin" || values["branch.main.pushremote"] != "origin" ||
		values["user.name"] != "Git User" {
		t.Fatalf("Git configuration was not migrated: %#v", values)
	}
	id, err := sha1.FromHex(commitID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Store.Load(id); err != nil {
		t.Fatal(err)
	}
	packedID := object.ComputeID(object.BlobType, []byte("packed object payload"))
	if _, err := a.Store.Load(packedID); err != nil {
		t.Fatalf("packed Git object was not imported: %v", err)
	}

	configAfter, err := os.ReadFile(filepath.Join(root, ".git", "config"))
	if err != nil {
		t.Fatal(err)
	}
	indexAfter, err := os.ReadFile(filepath.Join(root, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	workingAfter, err := os.ReadFile(filepath.Join(root, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(configAfter) != string(gitConfigBefore) || string(indexAfter) != string(gitIndexBefore) || string(workingAfter) != string(workingBefore) {
		t.Fatal("adoption modified Git metadata or the working tree")
	}
	excludeAfter, err := os.ReadFile(filepath.Join(root, ".git", "info", "exclude"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(excludeAfter), "*.local") || !strings.Contains(string(excludeAfter), "/.vstodo/") {
		t.Fatalf("Git exclude rules were not preserved and extended: %q", excludeAfter)
	}
	for _, name := range []string{"git-config.backup", "git-index.backup"} {
		if _, err := os.Stat(filepath.Join(root, ".vstodo", "migration", name)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAdoptRefusesExistingVSTodo(t *testing.T) {
	root, _ := makeGitRepo(t)
	if err := os.Mkdir(filepath.Join(root, ".vstodo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Adopt(root); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected existing repository refusal, got %v", err)
	}
}

func TestAdoptRefusesGitAlternatesAndSHA256(t *testing.T) {
	t.Run("alternates", func(t *testing.T) {
		root, _ := makeGitRepo(t)
		path := filepath.Join(root, ".git", "objects", "info", "alternates")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("../shared/objects\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Adopt(root); err == nil || !strings.Contains(err.Error(), "alternates") {
			t.Fatalf("got %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, ".vstodo")); !os.IsNotExist(err) {
			t.Fatal("created VSTodo metadata on rejected adoption")
		}
	})
	t.Run("sha256", func(t *testing.T) {
		root, _ := makeGitRepo(t)
		path := filepath.Join(root, ".git", "config")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, []byte("\n[extensions]\n\tobjectFormat = sha256\n")...)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := Adopt(root); err == nil || !strings.Contains(err.Error(), "SHA-256") {
			t.Fatalf("got %v", err)
		}
	})
}


func runGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	all := append([]string{"-C", root}, args...)
	cmd := exec.Command("git", all...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestAdoptRealGitPackedDeltaRepository(t *testing.T) {
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("Git CLI is not installed")
	}
	_ = git
	root := t.TempDir()
	init := exec.Command("git", "init", "--initial-branch=main", root)
	if out, err := init.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}
	runGit(t, root, "config", "user.name", "VSTodo Integration")
	runGit(t, root, "config", "user.email", "vstodo-integration@example.test")
	runGit(t, root, "remote", "add", "origin", "https://example.test/team/repo.git")
	runGit(t, root, "config", "branch.main.remote", "origin")
	runGit(t, root, "config", "branch.main.merge", "refs/heads/main")
	runGit(t, root, "config", "branch.main.pushRemote", "origin")

	for revision := 0; revision < 5; revision++ {
		content := strings.Repeat("stable history line for delta compression\n", 4000)
		content += fmt.Sprintf("revision marker %d\n", revision)
		if err := os.WriteFile(filepath.Join(root, "history.txt"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, root, "add", "history.txt")
		runGit(t, root, "commit", "-m", fmt.Sprintf("history revision %d", revision))
	}
	runGit(t, root, "tag", "v1.0")
	runGit(t, root, "pack-refs", "--all", "--prune")
	runGit(t, root, "gc", "--aggressive", "--prune=now")

	headBefore := runGit(t, root, "rev-parse", "HEAD")
	configBefore, err := os.ReadFile(filepath.Join(root, ".git", "config"))
	if err != nil {
		t.Fatal(err)
	}
	indexBefore, err := os.ReadFile(filepath.Join(root, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}

	report, err := Adopt(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.PackedObjects == 0 {
		t.Fatalf("expected packed Git objects after gc, report: %+v", report)
	}
	if runGit(t, root, "rev-parse", "HEAD") != headBefore {
		t.Fatal("Git HEAD changed during adoption")
	}
	if status := runGit(t, root, "status", "--porcelain"); status != "" {
		t.Fatalf("Git working tree should remain clean after adoption, got %q", status)
	}
	if runGit(t, root, "show-ref", "--verify", "refs/tags/v1.0") == "" {
		t.Fatal("Git tag disappeared after adoption")
	}
	configAfter, err := os.ReadFile(filepath.Join(root, ".git", "config"))
	if err != nil {
		t.Fatal(err)
	}
	indexAfter, err := os.ReadFile(filepath.Join(root, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	if string(configBefore) != string(configAfter) || string(indexBefore) != string(indexAfter) {
		t.Fatal("adoption changed Git config or index")
	}
	a, err := app.New(root)
	if err != nil {
		t.Fatal(err)
	}
	id, err := sha1.FromHex(headBefore)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Store.Load(id); err != nil {
		t.Fatalf("packed HEAD commit was not imported: %v", err)
	}
	if _, err := a.Refs.ReadRef("refs/tags/v1.0"); err != nil {
		t.Fatalf("packed tag was not imported: %v", err)
	}
}
