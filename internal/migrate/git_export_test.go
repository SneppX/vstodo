package migrate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
	"vstodo/internal/tree"
)

func makeRealGitRepoForExport(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git CLI is not installed")
	}
	root := t.TempDir()
	init := exec.Command("git", "init", "--initial-branch=main", root)
	if out, err := init.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}
	runGit(t, root, "config", "user.name", "Git Export Test")
	runGit(t, root, "config", "user.email", "git-export@example.test")
	runGit(t, root, "remote", "add", "origin", "https://example.test/team/project.git")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("original Git content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-m", "initial Git commit")
	return root
}

func createVSTodoCommitForExport(t *testing.T, root string) sha1.Digest {
	t.Helper()
	a, err := app.New(root)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := a.Refs.ReadRef("refs/heads/main")
	if err != nil {
		t.Fatal(err)
	}
	blobData := []byte("content committed by VSTodo\n")
	blobID, err := a.Store.Store(&object.Object{Type: object.BlobType, Data: blobData})
	if err != nil {
		t.Fatal(err)
	}
	treeData := tree.Build([]tree.Entry{{Mode: "100644", Name: "README.md", ID: blobID}})
	treeID, err := a.Store.Store(&object.Object{Type: object.TreeType, Data: treeData})
	if err != nil {
		t.Fatal(err)
	}
	identity := "VSTodo Export <vstodo-export@example.test>"
	commitData := commit.Build(treeID, []sha1.Digest{parent}, identity, identity, "VSTodo-created commit")
	commitID, err := a.Store.Store(&object.Object{Type: object.CommitType, Data: commitData})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Refs.WriteRef("refs/heads/main", commitID); err != nil {
		t.Fatal(err)
	}
	return commitID
}

func TestExportGitUpdatesPackedBranchAndPreservesGitState(t *testing.T) {
	root := makeRealGitRepoForExport(t)
	if _, err := Adopt(root); err != nil {
		t.Fatal(err)
	}
	commitID := createVSTodoCommitForExport(t, root)

	stagedPath := filepath.Join(root, "staged.txt")
	if err := os.WriteFile(stagedPath, []byte("staged independently\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "staged.txt")
	runGit(t, root, "pack-refs", "--all", "--prune")

	configBefore, err := os.ReadFile(filepath.Join(root, ".git", "config"))
	if err != nil {
		t.Fatal(err)
	}
	indexBefore, err := os.ReadFile(filepath.Join(root, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	workingBefore, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}

	report, err := ExportGit(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if report.Branch != "main" || report.Commit != commitID.Hex() || report.ObjectsWritten == 0 || report.UpToDate {
		t.Fatalf("unexpected export report: %+v", report)
	}
	if got := runGit(t, root, "rev-parse", "HEAD"); got != commitID.Hex() {
		t.Fatalf("Git HEAD = %s, want %s", got, commitID.Hex())
	}
	if got := runGit(t, root, "cat-file", "-t", commitID.Hex()); got != "commit" {
		t.Fatalf("Git object type = %q, want commit", got)
	}
	if got := runGit(t, root, "show", "HEAD:README.md"); got != "content committed by VSTodo" {
		t.Fatalf("Git did not read exported tree content: %q", got)
	}
	if got := runGit(t, root, "remote", "get-url", "origin"); got != "https://example.test/team/project.git" {
		t.Fatalf("Git remote changed: %q", got)
	}

	configAfter, err := os.ReadFile(filepath.Join(root, ".git", "config"))
	if err != nil {
		t.Fatal(err)
	}
	indexAfter, err := os.ReadFile(filepath.Join(root, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	workingAfter, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(configBefore) != string(configAfter) {
		t.Fatal("export modified .git/config")
	}
	if string(indexBefore) != string(indexAfter) {
		t.Fatal("export modified the Git index")
	}
	if string(workingBefore) != string(workingAfter) {
		t.Fatal("export modified working files")
	}
	if staged := runGit(t, root, "diff", "--cached", "--name-only"); !strings.Contains(staged, "staged.txt") {
		t.Fatalf("independently staged Git work was lost: %q", staged)
	}
}

func TestExportGitRejectsDivergedGitBranch(t *testing.T) {
	root := makeRealGitRepoForExport(t)
	if _, err := Adopt(root); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("external Git commit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-m", "external Git commit")
	headBefore := runGit(t, root, "rev-parse", "HEAD")

	if _, err := ExportGit(root, ""); err == nil || !strings.Contains(err.Error(), "non-fast-forward") {
		t.Fatalf("expected non-fast-forward refusal, got %v", err)
	}
	if headAfter := runGit(t, root, "rev-parse", "HEAD"); headAfter != headBefore {
		t.Fatalf("Git HEAD changed after rejected export: %s != %s", headAfter, headBefore)
	}
}

func TestExportGitUpdatesLooseBranchRef(t *testing.T) {
	root := makeRealGitRepoForExport(t)
	if _, err := Adopt(root); err != nil {
		t.Fatal(err)
	}
	commitID := createVSTodoCommitForExport(t, root)

	report, err := ExportGit(root, "main")
	if err != nil {
		t.Fatal(err)
	}
	if report.Commit != commitID.Hex() || report.UpToDate {
		t.Fatalf("unexpected loose-ref export report: %+v", report)
	}
	if got := runGit(t, root, "rev-parse", "HEAD"); got != commitID.Hex() {
		t.Fatalf("Git HEAD = %s, want %s", got, commitID.Hex())
	}
}

func TestExportGitRefLockPreventsConcurrentUpdate(t *testing.T) {
	root := makeRealGitRepoForExport(t)
	if _, err := Adopt(root); err != nil {
		t.Fatal(err)
	}
	commitID := createVSTodoCommitForExport(t, root)
	lockPath := filepath.Join(root, ".git", "refs", "heads", "main.lock")
	if err := os.WriteFile(lockPath, []byte("another Git process owns this lock"), 0o600); err != nil {
		t.Fatal(err)
	}
	headBefore := runGit(t, root, "rev-parse", "HEAD")

	if _, err := ExportGit(root, ""); err == nil || !strings.Contains(err.Error(), "locked") {
		t.Fatalf("expected locked-ref refusal, got %v", err)
	}
	if got := runGit(t, root, "rev-parse", "HEAD"); got != headBefore || got == commitID.Hex() {
		t.Fatalf("Git HEAD changed despite an existing ref lock: %s", got)
	}
}

func TestExportGitReportsAlreadyUpToDate(t *testing.T) {
	root := makeRealGitRepoForExport(t)
	if _, err := Adopt(root); err != nil {
		t.Fatal(err)
	}
	report, err := ExportGit(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if !report.UpToDate || report.ObjectsWritten != 0 {
		t.Fatalf("expected an up-to-date no-op, got %+v", report)
	}
}
