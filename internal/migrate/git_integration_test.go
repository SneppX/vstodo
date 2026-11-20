package migrate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"vstodo/internal/app"
	"vstodo/internal/sha1"
)

func TestAdoptAddsVSTodoGitExcludeWithoutDroppingRules(t *testing.T) {
	root, _ := makeGitRepo(t)
	excludePath := filepath.Join(root, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(excludePath, []byte("# user rules\n*.local\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := Adopt(root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "*.local") || !strings.Contains(string(data), "/.vstodo/") {
		t.Fatalf("Git exclude rules were not preserved and extended: %q", data)
	}
}

func TestAddVSTodoExcludeUsesCustomMetadataDirectory(t *testing.T) {
	gitDir := filepath.Join(t.TempDir(), ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	changed, err := addVSTodoExclude(gitDir, filepath.Join(".metadata", "vstodo[cache]"))
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected custom metadata exclude rule to be added")
	}
	data, err := os.ReadFile(filepath.Join(gitDir, "info", "exclude"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "/.metadata/vstodo\\[cache\\]/") {
		t.Fatalf("custom metadata path was not escaped and excluded: %q", data)
	}
	if strings.Contains(string(data), "/.vstodo/") {
		t.Fatalf("default metadata path was unexpectedly excluded: %q", data)
	}
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
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git CLI is not installed")
	}
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
