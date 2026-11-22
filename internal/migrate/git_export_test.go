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

func TestInspectGitReportsSynchronizedBranchWithoutMutation(t *testing.T) {
	root := makeRealGitRepoForExport(t)
	if _, err := Adopt(root); err != nil {
		t.Fatal(err)
	}
	report, err := InspectGit(root)
	if err != nil {
		t.Fatal(err)
	}
	if !report.BranchMatch || !report.SameCommit || report.GitCommit == "" || report.VSTodoCommit != report.GitCommit {
		t.Fatalf("unexpected Git status: %+v", report)
	}
}

func TestSyncGitFastForwardsVSTodoAndPreservesGitState(t *testing.T) {
	root := makeRealGitRepoForExport(t)
	if _, err := Adopt(root); err != nil {
		t.Fatal(err)
	}
	gitConfigBefore, err := os.ReadFile(filepath.Join(root, ".git", "config"))
	if err != nil {
		t.Fatal(err)
	}
	vstodoIndexPath := filepath.Join(root, ".vstodo", "index")
	vstodoIndexBefore, _ := os.ReadFile(vstodoIndexPath)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("new Git-side commit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-m", "Git-side commit after adoption")
	gitIndexBefore, err := os.ReadFile(filepath.Join(root, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	gitHead := runGit(t, root, "rev-parse", "HEAD")
	workingBefore, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}

	report, err := SyncGit(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if !report.Updated || report.VSTodoAhead || report.GitCommit != gitHead || report.ObjectsImported == 0 {
		t.Fatalf("unexpected sync report: %+v", report)
	}
	a, err := app.New(root)
	if err != nil {
		t.Fatal(err)
	}
	vstodoHead, err := a.Refs.ReadRef("refs/heads/main")
	if err != nil || vstodoHead.Hex() != gitHead {
		t.Fatalf("VSTodo HEAD = %s, err=%v; want %s", vstodoHead.Hex(), err, gitHead)
	}
	if got := runGit(t, root, "rev-parse", "HEAD"); got != gitHead {
		t.Fatalf("Git HEAD changed: %s != %s", got, gitHead)
	}
	gitConfigAfter, _ := os.ReadFile(filepath.Join(root, ".git", "config"))
	gitIndexAfter, _ := os.ReadFile(filepath.Join(root, ".git", "index"))
	vstodoIndexAfter, _ := os.ReadFile(vstodoIndexPath)
	workingAfter, _ := os.ReadFile(filepath.Join(root, "README.md"))
	if string(gitConfigBefore) != string(gitConfigAfter) || string(gitIndexBefore) != string(gitIndexAfter) {
		t.Fatal("sync-git modified Git config or index")
	}
	if string(vstodoIndexBefore) != string(vstodoIndexAfter) {
		t.Fatal("sync-git modified the VSTodo index")
	}
	if string(workingBefore) != string(workingAfter) {
		t.Fatal("sync-git modified working files")
	}
}

func TestSyncGitLeavesVSTodoAheadUnchanged(t *testing.T) {
	root := makeRealGitRepoForExport(t)
	if _, err := Adopt(root); err != nil {
		t.Fatal(err)
	}
	vstodoHead := createVSTodoCommitForExport(t, root)
	gitHeadBefore := runGit(t, root, "rev-parse", "HEAD")
	report, err := SyncGit(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if report.Updated || !report.VSTodoAhead {
		t.Fatalf("expected VSTodo-ahead no-op, got %+v", report)
	}
	if got := runGit(t, root, "rev-parse", "HEAD"); got != gitHeadBefore {
		t.Fatalf("Git HEAD changed: %s != %s", got, gitHeadBefore)
	}
	a, err := app.New(root)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := a.Refs.ReadRef("refs/heads/main"); err != nil || got != vstodoHead {
		t.Fatalf("VSTodo HEAD changed: %s, err=%v", got.Hex(), err)
	}
}

func TestSyncGitRejectsDivergedHistoryWithoutChangingRefs(t *testing.T) {
	root := makeRealGitRepoForExport(t)
	if _, err := Adopt(root); err != nil {
		t.Fatal(err)
	}
	vstodoHead := createVSTodoCommitForExport(t, root)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("independent Git-side commit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-m", "independent Git-side commit")
	gitHead := runGit(t, root, "rev-parse", "HEAD")
	if _, err := SyncGit(root, ""); err == nil || !strings.Contains(err.Error(), "diverged") {
		t.Fatalf("expected divergence refusal, got %v", err)
	}
	a, err := app.New(root)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := a.Refs.ReadRef("refs/heads/main"); err != nil || got != vstodoHead {
		t.Fatalf("VSTodo ref changed after rejected sync: %s, err=%v", got.Hex(), err)
	}
	if got := runGit(t, root, "rev-parse", "HEAD"); got != gitHead {
		t.Fatalf("Git ref changed after rejected sync: %s != %s", got, gitHead)
	}
}

func TestSyncGitRejectsMismatchedBranches(t *testing.T) {
	root := makeRealGitRepoForExport(t)
	if _, err := Adopt(root); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "checkout", "-b", "git-side")
	if _, err := SyncGit(root, ""); err == nil || !strings.Contains(err.Error(), "but VSTodo is on") {
		t.Fatalf("expected branch mismatch refusal, got %v", err)
	}
}

func TestSyncGitRefLockPreventsConcurrentUpdate(t *testing.T) {
	root := makeRealGitRepoForExport(t)
	if _, err := Adopt(root); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("new Git-side commit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-m", "Git-side commit after adoption")
	gitHead := runGit(t, root, "rev-parse", "HEAD")
	a, err := app.New(root)
	if err != nil {
		t.Fatal(err)
	}
	vstodoHead, err := a.Refs.ReadRef("refs/heads/main")
	if err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(root, ".git", "refs", "heads", "main.lock")
	if err := os.WriteFile(lockPath, []byte("another Git process owns this lock"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncGit(root, ""); err == nil || !strings.Contains(err.Error(), "locked") {
		t.Fatalf("expected locked Git ref refusal, got %v", err)
	}
	if got, err := a.Refs.ReadRef("refs/heads/main"); err != nil || got != vstodoHead {
		t.Fatalf("VSTodo ref changed despite Git lock: %s, err=%v", got.Hex(), err)
	}
	if got := runGit(t, root, "rev-parse", "HEAD"); got != gitHead {
		t.Fatalf("Git ref changed despite lock: %s != %s", got, gitHead)
	}
}
