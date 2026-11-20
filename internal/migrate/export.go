package migrate

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
	"vstodo/internal/tree"
)

// ExportReport describes the Git branch update performed by ExportGit.
type ExportReport struct {
	Branch         string
	Commit         string
	ObjectsWritten int
	UpToDate       bool
}

// ExportGit publishes the current VSTodo branch into the matching branch of
// the adopted Git repository. It writes Git-compatible loose objects first,
// then updates the Git ref through the standard .lock-file protocol. The Git
// index, config, and working tree are never modified.
func ExportGit(root, requestedBranch string) (ExportReport, error) {
	var report ExportReport
	abs, err := filepath.Abs(root)
	if err != nil {
		return report, err
	}
	a, err := app.New(abs)
	if err != nil {
		return report, fmt.Errorf("export-git: open VSTodo repository: %w", err)
	}

	gitDir := filepath.Join(abs, ".git")
	info, err := os.Lstat(gitDir)
	if err != nil {
		return report, fmt.Errorf("export-git: inspect .git: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return report, errors.New("export-git: expected a standard .git directory; linked worktrees and submodules are not supported")
	}
	if _, err := os.Stat(a.Repo.GitPath("migration", "git-config.backup")); err != nil {
		return report, errors.New("export-git: this VSTodo repository has no Git adoption snapshot; run vstodo adopt first")
	}

	gitBranch, err := readGitHeadBranch(gitDir)
	if err != nil {
		return report, err
	}
	branch := gitBranch
	if requestedBranch != "" && requestedBranch != gitBranch {
		return report, fmt.Errorf("export-git: Git HEAD is on %q, not requested branch %q", gitBranch, requestedBranch)
	}
	if !validRefName("refs/heads/" + branch) {
		return report, fmt.Errorf("export-git: invalid branch name %q", branch)
	}
	vstodoBranch, err := a.Refs.CurrentBranch()
	if err != nil {
		return report, err
	}
	if vstodoBranch != branch {
		return report, fmt.Errorf("export-git: Git is on %q but VSTodo HEAD is on %q; check out the matching VSTodo branch first", branch, vstodoBranch)
	}

	localHead, err := a.Refs.ReadRef("refs/heads/" + branch)
	if err != nil {
		return report, fmt.Errorf("export-git: read VSTodo branch %q: %w", branch, err)
	}
	if localHead == sha1.Zero {
		return report, fmt.Errorf("export-git: VSTodo branch %q has no commit", branch)
	}
	gitHead, err := readGitRef(gitDir, "refs/heads/"+branch)
	if err != nil {
		return report, err
	}
	report.Branch = branch
	report.Commit = localHead.Hex()
	if gitHead == localHead {
		report.UpToDate = true
		return report, nil
	}
	if gitHead != sha1.Zero {
		ancestor, err := isCommitAncestorInStore(a, gitHead, localHead)
		if err != nil {
			return report, fmt.Errorf("export-git: current Git tip %s is not available in VSTodo history; import the newer Git history before exporting: %w", gitHead.Hex(), err)
		}
		if !ancestor {
			return report, fmt.Errorf("export-git: refusing non-fast-forward update of Git branch %q", branch)
		}
	}

	ids, err := reachableObjectIDs(a, localHead)
	if err != nil {
		return report, fmt.Errorf("export-git: collect reachable objects: %w", err)
	}
	for _, id := range ids {
		raw, err := a.Store.ReadRaw(id)
		if err != nil {
			return report, fmt.Errorf("export-git: read VSTodo object %s: %w", id.Hex(), err)
		}
		written, err := writeGitLooseObject(gitDir, id, raw)
		if err != nil {
			return report, fmt.Errorf("export-git: write Git object %s: %w", id.Hex(), err)
		}
		if written {
			report.ObjectsWritten++
		}
	}

	if err := updateGitBranchRef(gitDir, "refs/heads/"+branch, gitHead, localHead); err != nil {
		return report, err
	}
	return report, nil
}

func readGitHeadBranch(gitDir string) (string, error) {
	path := filepath.Join(gitDir, "HEAD")
	info, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("export-git: inspect Git HEAD: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", errors.New("export-git: refusing non-regular Git HEAD")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(data))
	const prefix = "ref: refs/heads/"
	if !strings.HasPrefix(value, prefix) {
		return "", errors.New("export-git: Git HEAD is detached; check out a branch before exporting")
	}
	branch := strings.TrimPrefix(value, prefix)
	if !validRefName("refs/heads/" + branch) {
		return "", fmt.Errorf("export-git: unsafe Git HEAD branch %q", branch)
	}
	return branch, nil
}

func readGitRef(gitDir, refName string) (sha1.Digest, error) {
	if !validRefName(refName) {
		return sha1.Zero, fmt.Errorf("export-git: unsafe Git ref %q", refName)
	}
	refPath := filepath.Join(gitDir, filepath.FromSlash(refName))
	if err := checkGitDirectoryPath(gitDir, filepath.Dir(refPath)); err != nil {
		return sha1.Zero, err
	}
	info, err := os.Lstat(refPath)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return sha1.Zero, fmt.Errorf("export-git: refusing non-regular Git ref %s", refName)
		}
		data, err := os.ReadFile(refPath)
		if err != nil {
			return sha1.Zero, err
		}
		id, err := sha1.FromHex(strings.TrimSpace(string(data)))
		if err != nil {
			return sha1.Zero, fmt.Errorf("export-git: malformed Git ref %s: %w", refName, err)
		}
		return id, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return sha1.Zero, err
	}

	packedPath := filepath.Join(gitDir, "packed-refs")
	packedInfo, err := os.Lstat(packedPath)
	if errors.Is(err, os.ErrNotExist) {
		return sha1.Zero, nil
	}
	if err != nil {
		return sha1.Zero, err
	}
	if packedInfo.Mode()&os.ModeSymlink != 0 || !packedInfo.Mode().IsRegular() {
		return sha1.Zero, errors.New("export-git: refusing non-regular packed-refs file")
	}
	file, err := os.Open(packedPath)
	if err != nil {
		return sha1.Zero, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "^") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == refName {
			id, err := sha1.FromHex(fields[0])
			if err != nil {
				return sha1.Zero, fmt.Errorf("export-git: malformed packed ref %s: %w", refName, err)
			}
			return id, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return sha1.Zero, err
	}
	return sha1.Zero, nil
}

func checkGitDirectoryPath(root, target string) error {
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("export-git: Git metadata path escapes .git: %s", target)
	}
	current := root
	if rel == "." {
		return nil
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("export-git: refusing non-directory or symlink in Git metadata path %s", current)
		}
	}
	return nil
}

func ensureGitDirectory(root, target string) error {
	if err := checkGitDirectoryPath(root, target); err != nil {
		return err
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return err
	}
	current := root
	if rel == "." {
		return nil
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		if err := os.Mkdir(current, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("export-git: refusing non-directory or symlink in Git metadata path %s", current)
		}
	}
	return nil
}

func decodeGitLooseObject(raw []byte, expected sha1.Digest) error {
	decoded, err := object.Decompress(raw)
	if err != nil {
		return err
	}
	typ, data, err := object.Unmarshal(decoded)
	if err != nil {
		return err
	}
	actual := object.ComputeID(typ, data)
	if actual != expected {
		return fmt.Errorf("object hashes to %s, expected %s", actual.Hex(), expected.Hex())
	}
	return nil
}

func writeGitLooseObject(gitDir string, id sha1.Digest, raw []byte) (bool, error) {
	if err := decodeGitLooseObject(raw, id); err != nil {
		return false, fmt.Errorf("invalid VSTodo object data: %w", err)
	}
	objectsDir := filepath.Join(gitDir, "objects")
	prefixDir := filepath.Join(objectsDir, id.Hex()[:2])
	if err := ensureGitDirectory(gitDir, prefixDir); err != nil {
		return false, err
	}
	dest := filepath.Join(prefixDir, id.Hex()[2:])
	info, err := os.Lstat(dest)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return false, fmt.Errorf("refusing non-regular Git object %s", id.Hex())
		}
		existing, err := os.ReadFile(dest)
		if err != nil {
			return false, err
		}
		if err := decodeGitLooseObject(existing, id); err != nil {
			return false, fmt.Errorf("existing Git object %s is corrupt: %w", id.Hex(), err)
		}
		return false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}

	tmp, err := os.CreateTemp(prefixDir, ".vstodo-object-*")
	if err != nil {
		return false, err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return false, err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return false, err
	}
	if err := tmp.Chmod(0o444); err != nil {
		_ = tmp.Close()
		return false, err
	}
	if err := tmp.Close(); err != nil {
		return false, err
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		return false, err
	}
	return true, nil
}

func updateGitBranchRef(gitDir, refName string, expected, next sha1.Digest) error {
	if !validRefName(refName) {
		return fmt.Errorf("export-git: unsafe Git branch ref %q", refName)
	}
	refPath := filepath.Join(gitDir, filepath.FromSlash(refName))
	if err := ensureGitDirectory(gitDir, filepath.Dir(refPath)); err != nil {
		return err
	}
	lockPath := refPath + ".lock"
	lock, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("export-git: Git ref %s is locked; remove the stale lock only after checking no Git process is active", refName)
		}
		return err
	}
	defer os.Remove(lockPath)

	current, err := readGitRef(gitDir, refName)
	if err != nil {
		_ = lock.Close()
		return err
	}
	if current != expected {
		_ = lock.Close()
		return fmt.Errorf("export-git: Git ref %s changed during export; no ref was updated", refName)
	}
	if _, err := fmt.Fprintf(lock, "%s\n", next.Hex()); err != nil {
		_ = lock.Close()
		return err
	}
	if err := lock.Sync(); err != nil {
		_ = lock.Close()
		return err
	}
	if err := lock.Close(); err != nil {
		return err
	}
	if err := os.Rename(lockPath, refPath); err != nil {
		return fmt.Errorf("export-git: atomically update %s: %w", refName, err)
	}
	return nil
}

func isCommitAncestorInStore(a *app.App, ancestor, descendant sha1.Digest) (bool, error) {
	if ancestor == sha1.Zero {
		return true, nil
	}
	if descendant == sha1.Zero {
		return false, nil
	}
	queue := []sha1.Digest{descendant}
	seen := make(map[sha1.Digest]struct{})
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if id == ancestor {
			return true, nil
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		obj, err := a.Store.Load(id)
		if err != nil {
			return false, err
		}
		if obj.Type != object.CommitType {
			return false, fmt.Errorf("object %s is not a commit", id.Hex())
		}
		parsed, err := commit.Parse(id, obj.Data)
		if err != nil {
			return false, err
		}
		queue = append(queue, parsed.Parents...)
	}
	return false, nil
}

func reachableObjectIDs(a *app.App, tip sha1.Digest) ([]sha1.Digest, error) {
	queue := []sha1.Digest{tip}
	seen := make(map[sha1.Digest]struct{})
	ids := make([]sha1.Digest, 0)
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		obj, err := a.Store.Load(id)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
		switch obj.Type {
		case object.CommitType:
			parsed, err := commit.Parse(id, obj.Data)
			if err != nil {
				return nil, err
			}
			queue = append(queue, parsed.Tree)
			queue = append(queue, parsed.Parents...)
		case object.TreeType:
			parsed, err := tree.Parse(id, obj.Data)
			if err != nil {
				return nil, err
			}
			for _, entry := range parsed.Entries {
				if entry.Mode != "160000" {
					queue = append(queue, entry.ID)
				}
			}
		case object.BlobType:
		default:
			return nil, fmt.Errorf("unsupported reachable object type %q", obj.Type)
		}
	}
	return ids, nil
}
