package migrate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"vstodo/internal/app"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

// GitStatusReport describes the two branch tips without changing either repository.
type GitStatusReport struct {
	GitBranch    string
	VSTodoBranch string
	GitCommit    string
	VSTodoCommit string
	SameCommit   bool
	BranchMatch  bool
}

// InspectGit reads the checked-out Git branch and its matching VSTodo branch.
// It is read-only and requires a repository created by migrate.Adopt.
func InspectGit(root string) (GitStatusReport, error) {
	var report GitStatusReport
	a, gitDir, err := openAdoptedGit(root, "git-status")
	if err != nil {
		return report, err
	}
	gitBranch, err := readGitHeadBranch(gitDir)
	if err != nil {
		return report, fmt.Errorf("git-status: %w", err)
	}
	vstodoBranch, err := a.Refs.CurrentBranch()
	if err != nil {
		return report, fmt.Errorf("git-status: read VSTodo HEAD: %w", err)
	}
	if vstodoBranch == "" {
		return report, errors.New("git-status: VSTodo HEAD is detached; check out a branch before comparing")
	}
	gitHead, err := readGitRef(gitDir, "refs/heads/"+gitBranch)
	if err != nil {
		return report, fmt.Errorf("git-status: read Git branch: %w", err)
	}
	vstodoHead, err := a.Refs.ReadRef("refs/heads/" + vstodoBranch)
	if err != nil {
		return report, fmt.Errorf("git-status: read VSTodo branch: %w", err)
	}
	report.GitBranch = gitBranch
	report.VSTodoBranch = vstodoBranch
	report.GitCommit = gitHead.Hex()
	report.VSTodoCommit = vstodoHead.Hex()
	report.BranchMatch = gitBranch == vstodoBranch
	report.SameCommit = report.BranchMatch && gitHead == vstodoHead
	return report, nil
}

// SyncGitReport describes a fast-forward reconciliation from Git into VSTodo.
type SyncGitReport struct {
	Branch          string
	GitCommit       string
	PreviousCommit  string
	ObjectsImported int
	Updated         bool
	VSTodoAhead     bool
}

// SyncGit imports Git objects and fast-forwards the matching VSTodo branch when
// Git is ahead. It never updates Git refs, either index, or working-tree files.
func SyncGit(root, requestedBranch string) (SyncGitReport, error) {
	var report SyncGitReport
	a, gitDir, err := openAdoptedGit(root, "sync-git")
	if err != nil {
		return report, err
	}
	gitBranch, err := readGitHeadBranch(gitDir)
	if err != nil {
		return report, fmt.Errorf("sync-git: %w", err)
	}
	if requestedBranch != "" && requestedBranch != gitBranch {
		return report, fmt.Errorf("sync-git: Git HEAD is on %q, not requested branch %q", gitBranch, requestedBranch)
	}
	vstodoBranch, err := a.Refs.CurrentBranch()
	if err != nil {
		return report, fmt.Errorf("sync-git: read VSTodo HEAD: %w", err)
	}
	if vstodoBranch == "" {
		return report, errors.New("sync-git: VSTodo HEAD is detached; check out a branch before syncing")
	}
	if vstodoBranch != gitBranch {
		return report, fmt.Errorf("sync-git: Git is on %q but VSTodo is on %q; check out the matching VSTodo branch first", gitBranch, vstodoBranch)
	}
	refName := "refs/heads/" + gitBranch
	vstodoHead, err := a.Refs.ReadRef(refName)
	if err != nil {
		return report, fmt.Errorf("sync-git: read VSTodo branch %q: %w", gitBranch, err)
	}
	gitHead, err := readGitRef(gitDir, refName)
	if err != nil {
		return report, fmt.Errorf("sync-git: read Git branch %q: %w", gitBranch, err)
	}
	report.Branch = gitBranch
	report.GitCommit = gitHead.Hex()
	report.PreviousCommit = vstodoHead.Hex()
	if gitHead == sha1.Zero {
		return report, fmt.Errorf("sync-git: Git branch %q has no commit", gitBranch)
	}
	if gitHead == vstodoHead {
		return report, nil
	}

	loose, err := importLooseObjects(filepath.Join(gitDir, "objects"), a)
	if err != nil {
		return report, fmt.Errorf("sync-git: import Git loose objects: %w", err)
	}
	packed, err := importPackfiles(filepath.Join(gitDir, "objects", "pack"), a)
	if err != nil {
		return report, fmt.Errorf("sync-git: import Git packfiles: %w", err)
	}
	report.ObjectsImported = loose + packed

	gitObject, err := a.Store.Load(gitHead)
	if err != nil {
		return report, fmt.Errorf("sync-git: Git branch tip %s is unavailable after object import: %w", gitHead.Hex(), err)
	}
	if gitObject.Type != object.CommitType {
		return report, fmt.Errorf("sync-git: Git branch tip %s is not a commit", gitHead.Hex())
	}
	if _, err := reachableObjectIDs(a, gitHead); err != nil {
		return report, fmt.Errorf("sync-git: Git branch history is incomplete or invalid: %w", err)
	}

	gitIsAncestor, err := isCommitAncestorInStore(a, gitHead, vstodoHead)
	if err != nil {
		return report, fmt.Errorf("sync-git: inspect VSTodo history: %w", err)
	}
	if gitIsAncestor {
		report.VSTodoAhead = true
		return report, nil
	}
	vstodoIsAncestor, err := isCommitAncestorInStore(a, vstodoHead, gitHead)
	if err != nil {
		return report, fmt.Errorf("sync-git: inspect Git history: %w", err)
	}
	if !vstodoIsAncestor {
		return report, fmt.Errorf("sync-git: Git and VSTodo histories diverged; no branch was changed (Git %s, VSTodo %s)", gitHead.Hex(), vstodoHead.Hex())
	}

	// Re-read Git HEAD and its branch ref before updating VSTodo. If Git moved
	// while objects were imported, leave both branch tips untouched and retry.
	latestBranch, err := readGitHeadBranch(gitDir)
	if err != nil {
		return report, fmt.Errorf("sync-git: recheck Git HEAD: %w", err)
	}
	latestGitHead, err := readGitRef(gitDir, refName)
	if err != nil {
		return report, fmt.Errorf("sync-git: recheck Git branch: %w", err)
	}
	if latestBranch != gitBranch || latestGitHead != gitHead {
		return report, errors.New("sync-git: Git branch changed during import; no VSTodo ref was updated, retry the command")
	}
	if err := a.Refs.UpdateRef(refName, &vstodoHead, gitHead); err != nil {
		return report, fmt.Errorf("sync-git: VSTodo branch changed during import; no ref was updated, retry the command: %w", err)
	}
	report.Updated = true
	return report, nil
}

func openAdoptedGit(root, operation string) (*app.App, string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, "", err
	}
	a, err := app.New(abs)
	if err != nil {
		return nil, "", fmt.Errorf("%s: open VSTodo repository: %w", operation, err)
	}
	gitDir := filepath.Join(abs, ".git")
	info, err := os.Lstat(gitDir)
	if err != nil {
		return nil, "", fmt.Errorf("%s: inspect .git: %w", operation, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, "", fmt.Errorf("%s: expected a standard .git directory; linked worktrees and submodules are not supported", operation)
	}
	backup := filepath.Join(a.Repo.GitPath("migration"), "git-config.backup")
	backupInfo, err := os.Lstat(backup)
	if err != nil {
		return nil, "", fmt.Errorf("%s: this VSTodo repository has no Git adoption snapshot; run vstodo adopt first", operation)
	}
	if backupInfo.Mode()&os.ModeSymlink != 0 || !backupInfo.Mode().IsRegular() {
		return nil, "", fmt.Errorf("%s: refusing a non-regular Git adoption snapshot", operation)
	}
	return a, gitDir, nil
}
