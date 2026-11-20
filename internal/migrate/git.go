// Package migrate imports an existing Git repository into VSTodo without
// modifying the source .git directory or the working tree.
package migrate

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"vstodo/internal/app"
	"vstodo/internal/config"
	"vstodo/internal/object"
	"vstodo/internal/packfile"
	"vstodo/internal/ref"
	"vstodo/internal/repo"
	"vstodo/internal/sha1"
)

type Report struct {
	LooseObjects      int
	PackedObjects     int
	References        int
	Remotes           int
	Branch            string
	GitExcludeUpdated bool
}

type manifest struct {
	SchemaVersion   int      `json:"schema_version"`
	Source          string   `json:"source"`
	ObjectFormat    string   `json:"object_format"`
	Branch          string   `json:"branch,omitempty"`
	Remotes         []string `json:"remotes"`
	GitMetadata     []string `json:"preserved_metadata"`
	LocalGitChanges []string `json:"local_git_changes,omitempty"`
}

// Adopt adds a VSTodo repository beside a standard Git .git directory.
// It preserves Git objects, refs, config, index, remotes, and working files.
// The only source-Git metadata change is a local exclude rule for .vstodo/.
func Adopt(root string) (Report, error) {
	var report Report
	abs, err := filepath.Abs(root)
	if err != nil {
		return report, err
	}

	gitDir := filepath.Join(abs, ".git")
	info, err := os.Lstat(gitDir)
	if err != nil {
		return report, fmt.Errorf("adopt: inspect %s: %w", gitDir, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return report, errors.New("adopt: expected a standard .git directory; linked worktrees and submodules are not supported yet")
	}
	vstodoName := os.Getenv(repo.EnvOverride)
	if vstodoName == "" {
		vstodoName = repo.DefaultDir
	}
	vstodoDir := filepath.Join(abs, vstodoName)
	relativeVSTodoDir, err := filepath.Rel(abs, vstodoDir)
	if err != nil || relativeVSTodoDir == "." || relativeVSTodoDir == ".." ||
		strings.HasPrefix(relativeVSTodoDir, ".."+string(filepath.Separator)) {
		return report, errors.New("adopt: VSTODO_DIR must resolve inside the Git working tree")
	}
	if _, err := os.Lstat(vstodoDir); err == nil {
		return report, errors.New("adopt: VSTodo metadata directory already exists; refusing to overwrite it")
	} else if !errors.Is(err, os.ErrNotExist) {
		return report, err
	}

	gitConfig, err := os.ReadFile(filepath.Join(gitDir, "config"))
	if err != nil {
		return report, fmt.Errorf("adopt: read Git config: %w", err)
	}
	lowerConfig := strings.ToLower(string(gitConfig))
	if strings.Contains(lowerConfig, "objectformat = sha256") || strings.Contains(lowerConfig, "objectformat=sha256") {
		return report, errors.New("adopt: SHA-256 Git repositories are not supported; VSTodo currently uses SHA-1 object IDs")
	}

	alternates := filepath.Join(gitDir, "objects", "info", "alternates")
	if data, readErr := os.ReadFile(alternates); readErr == nil && strings.TrimSpace(string(data)) != "" {
		return report, errors.New("adopt: Git object alternates are present; consolidate the source object database before adoption")
	} else if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return report, readErr
	}

	headBytes, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return report, fmt.Errorf("adopt: read Git HEAD: %w", err)
	}
	headText := strings.TrimSpace(string(headBytes))
	if !strings.HasPrefix(headText, "ref: ") {
		if _, err := sha1.FromHex(headText); err != nil {
			return report, fmt.Errorf("adopt: unsupported Git HEAD: %w", err)
		}
	}

	a, err := app.Init(abs)
	if err != nil {
		return report, fmt.Errorf("adopt: initialize VSTodo metadata: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(a.Repo.GitDir)
		}
	}()

	looseCount, err := importLooseObjects(filepath.Join(gitDir, "objects"), a)
	if err != nil {
		return report, err
	}
	packedCount, err := importPackfiles(filepath.Join(gitDir, "objects", "pack"), a)
	if err != nil {
		return report, err
	}

	refs := ref.NewStore(a.Repo.GitDir)
	refCount, err := importReferences(gitDir, refs)
	if err != nil {
		return report, err
	}
	if strings.HasPrefix(headText, "ref: ") {
		target := strings.TrimSpace(strings.TrimPrefix(headText, "ref: "))
		if !validRefName(target) {
			return report, fmt.Errorf("adopt: unsafe HEAD target %q", target)
		}
		if _, readErr := refs.ReadRef(target); readErr != nil && !errors.Is(readErr, ref.ErrNotFound) {
			return report, fmt.Errorf("adopt: resolve HEAD target: %w", readErr)
		}
		if err := refs.SetHead("ref: " + target); err != nil {
			return report, err
		}
		if strings.HasPrefix(target, "refs/heads/") {
			report.Branch = strings.TrimPrefix(target, "refs/heads/")
		}
	} else {
		headID, _ := sha1.FromHex(headText)
		if _, loadErr := a.Store.Load(headID); loadErr != nil {
			return report, fmt.Errorf("adopt: detached HEAD object %s is unavailable: %w", headID.Hex(), loadErr)
		}
		if err := refs.SetHead(headID.Hex()); err != nil {
			return report, err
		}
	}

	remoteNames, err := importConfig(gitConfig, a)
	if err != nil {
		return report, err
	}
	if err := preserveMetadata(gitDir, a); err != nil {
		return report, err
	}

	m := manifest{
		SchemaVersion:   1,
		Source:          ".git",
		ObjectFormat:    "sha1",
		Branch:          report.Branch,
		Remotes:         remoteNames,
		GitMetadata:     []string{"config", "index"},
		LocalGitChanges: []string{"info/exclude"},
	}
	manifestBytes, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return report, err
	}
	if err := os.WriteFile(a.Repo.GitPath("migration", "git-adoption.json"), append(manifestBytes, '\n'), 0o644); err != nil {
		return report, err
	}
	// Keep VSTodo's private metadata out of ordinary Git status output. This
	// is deliberately the final source-repository operation so a failed
	// adoption cannot leave a partially imported VSTodo directory behind.
	changed, err := addVSTodoExclude(gitDir)
	if err != nil {
		return report, fmt.Errorf("adopt: update Git info/exclude: %w", err)
	}

	report.LooseObjects = looseCount
	report.PackedObjects = packedCount
	report.References = refCount
	report.Remotes = len(remoteNames)
	report.GitExcludeUpdated = changed
	committed = true
	return report, nil
}

// addVSTodoExclude preserves existing Git ignore rules and adds a local rule
// for VSTodo's metadata directory. Symlinks and non-regular files are refused.
func addVSTodoExclude(gitDir string) (bool, error) {
	infoDir := filepath.Join(gitDir, "info")
	info, err := os.Lstat(infoDir)
	createdInfo := false
	if errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(infoDir, 0o700); err != nil {
			return false, err
		}
		createdInfo = true
	} else if err != nil {
		return false, err
	} else if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return false, errors.New("Git info path must be a real directory")
	}
	success := false
	defer func() {
		if createdInfo && !success {
			_ = os.Remove(infoDir)
		}
	}()

	excludePath := filepath.Join(infoDir, "exclude")
	mode := os.FileMode(0o644)
	var original []byte
	fileInfo, err := os.Lstat(excludePath)
	if err == nil {
		if fileInfo.Mode()&os.ModeSymlink != 0 || !fileInfo.Mode().IsRegular() {
			return false, errors.New("Git info/exclude must be a regular file")
		}
		mode = fileInfo.Mode().Perm()
		original, err = os.ReadFile(excludePath)
		if err != nil {
			return false, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	for _, line := range strings.Split(string(original), "\n") {
		rule := strings.TrimSpace(line)
		if rule == "/.vstodo/" || rule == ".vstodo/" {
			success = true
			return false, nil
		}
	}

	updated := append([]byte(nil), original...)
	if len(updated) > 0 && updated[len(updated)-1] != '\n' {
		updated = append(updated, '\n')
	}
	updated = append(updated, []byte("# VSTodo metadata\n/.vstodo/\n")...)
	tmp, err := os.CreateTemp(infoDir, ".vstodo-exclude-*")
	if err != nil {
		return false, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return false, err
	}
	if _, err := tmp.Write(updated); err != nil {
		_ = tmp.Close()
		return false, err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return false, err
	}
	if err := tmp.Close(); err != nil {
		return false, err
	}
	if err := os.Rename(tmpName, excludePath); err != nil {
		return false, err
	}
	success = true
	return true, nil
}

func importLooseObjects(objectsDir string, a *app.App) (int, error) {
	count := 0
	objectInfo, err := os.Lstat(objectsDir)
	if err != nil {
		return 0, err
	}
	if objectInfo.Mode()&os.ModeSymlink != 0 || !objectInfo.IsDir() {
		return 0, errors.New("adopt: Git object directory must be a real directory")
	}
	entries, err := os.ReadDir(objectsDir)
	if err != nil {
		return 0, err
	}
	for _, dir := range entries {
		if !dir.IsDir() || len(dir.Name()) != 2 {
			continue
		}
		if _, err := strconv.ParseUint(dir.Name(), 16, 8); err != nil {
			continue
		}
		dirPath := filepath.Join(objectsDir, dir.Name())
		dirInfo, err := os.Lstat(dirPath)
		if err != nil {
			return count, err
		}
		if dirInfo.Mode()&os.ModeSymlink != 0 || !dirInfo.IsDir() {
			return count, fmt.Errorf("adopt: refusing unsafe object directory %s", dirPath)
		}
		files, err := os.ReadDir(dirPath)
		if err != nil {
			return count, err
		}
		for _, file := range files {
			if file.IsDir() || len(file.Name()) != 38 {
				continue
			}
			id, err := sha1.FromHex(dir.Name() + file.Name())
			if err != nil {
				continue
			}
			path := filepath.Join(objectsDir, dir.Name(), file.Name())
			fileInfo, err := os.Lstat(path)
			if err != nil {
				return count, err
			}
			if fileInfo.Mode()&os.ModeSymlink != 0 || !fileInfo.Mode().IsRegular() {
				return count, fmt.Errorf("adopt: refusing non-regular Git object %s", path)
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return count, err
			}
			if err := a.Store.StoreRaw(id, raw); err != nil {
				return count, err
			}
			if _, err := a.Store.Load(id); err != nil {
				return count, fmt.Errorf("adopt: invalid loose object %s: %w", id.Hex(), err)
			}
			count++
		}
	}
	return count, nil
}

func importPackfiles(packDir string, a *app.App) (int, error) {
	entries, err := os.ReadDir(packDir)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".pack") {
			continue
		}
		packPath := filepath.Join(packDir, entry.Name())
		info, err := os.Lstat(packPath)
		if err != nil {
			return count, err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return count, fmt.Errorf("adopt: refusing non-regular Git pack %s", packPath)
		}
		data, err := os.ReadFile(packPath)
		if err != nil {
			return count, err
		}
		pack, err := packfile.ReadPack(data)
		if err != nil {
			return count, fmt.Errorf("adopt: parse Git pack %s: %w", entry.Name(), err)
		}
		for _, packed := range pack.Objects {
			id, err := a.Store.Store(&object.Object{Type: packed.Type, Data: packed.Data})
			if err != nil {
				return count, err
			}
			if id != packed.ID {
				return count, fmt.Errorf("adopt: object hash mismatch in %s: got %s, expected %s", entry.Name(), id.Hex(), packed.ID.Hex())
			}
			count++
		}
	}
	return count, nil
}

func importReferences(gitDir string, refs *ref.Store) (int, error) {
	count := 0
	packedPath := filepath.Join(gitDir, "packed-refs")
	file, err := os.Open(packedPath)
	if err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "^") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) != 2 || !validRefName(fields[1]) {
				_ = file.Close()
				return count, fmt.Errorf("adopt: malformed packed ref %q", line)
			}
			id, err := sha1.FromHex(fields[0])
			if err != nil {
				_ = file.Close()
				return count, err
			}
			if err := refs.WriteRef(fields[1], id); err != nil {
				_ = file.Close()
				return count, err
			}
			count++
		}
		if err := scanner.Err(); err != nil {
			_ = file.Close()
			return count, err
		}
		if err := file.Close(); err != nil {
			return count, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return count, err
	}

	root := filepath.Join(gitDir, "refs")
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("adopt: refusing non-regular Git ref %s", path)
		}
		rel, err := filepath.Rel(gitDir, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		if !validRefName(name) {
			return fmt.Errorf("adopt: unsafe Git ref %q", name)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		value := strings.TrimSpace(string(data))
		if strings.HasPrefix(value, "ref: ") {
			target := strings.TrimSpace(strings.TrimPrefix(value, "ref: "))
			if !validRefName(target) {
				return fmt.Errorf("adopt: unsafe symbolic ref target %q", target)
			}
			if err := refs.WriteRefSymbolic(name, target); err != nil {
				return err
			}
		} else {
			id, err := sha1.FromHex(value)
			if err != nil {
				return fmt.Errorf("adopt: malformed ref %s: %w", name, err)
			}
			if err := refs.WriteRef(name, id); err != nil {
				return err
			}
		}
		count++
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return count, err
	}
	return count, nil
}

func validRefName(name string) bool {
	if !strings.HasPrefix(name, "refs/") || strings.Contains(name, "\\") ||
		strings.Contains(name, "//") || strings.Contains(name, "..") || strings.HasSuffix(name, "/") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") {
			return false
		}
	}
	return true
}

func importConfig(data []byte, a *app.App) ([]string, error) {
	cfg := config.New(a.Repo.GitPath("config"))
	if err := cfg.Load(); err != nil {
		return nil, err
	}
	section := ""
	remoteSet := make(map[string]bool)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if strings.HasPrefix(value, "\"") {
			if unquoted, err := strconv.Unquote(value); err == nil {
				value = unquoted
			}
		}
		if section == "user" && (key == "name" || key == "email") {
			if err := cfg.Set("user."+key, value); err != nil {
				return nil, err
			}
		}
		if strings.HasPrefix(section, "remote \"") && strings.HasSuffix(section, "\"") {
			name := strings.TrimSuffix(strings.TrimPrefix(section, "remote \""), "\"")
			if name == "" || strings.ContainsAny(name, "\r\n") {
				continue
			}
			if key == "url" || key == "pushurl" || key == "fetch" {
				if err := cfg.Set("remote."+name+"."+key, value); err != nil {
					return nil, err
				}
				remoteSet[name] = true
			}
		}
		if strings.HasPrefix(section, "branch \"") && strings.HasSuffix(section, "\"") {
			name := strings.TrimSuffix(strings.TrimPrefix(section, "branch \""), "\"")
			if name != "" && (key == "remote" || key == "merge" || key == "rebase" || key == "pushremote") {
				if err := cfg.Set("branch."+name+"."+key, value); err != nil {
					return nil, err
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if err := cfg.Save(); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(remoteSet))
	for name := range remoteSet {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func preserveMetadata(gitDir string, a *app.App) error {
	migrationDir := a.Repo.GitPath("migration")
	if err := os.MkdirAll(migrationDir, 0o700); err != nil {
		return err
	}
	for _, name := range []string{"config", "index"} {
		src := filepath.Join(gitDir, name)
		data, err := os.ReadFile(src)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(migrationDir, "git-"+name+".backup"), data, 0o600); err != nil {
			return err
		}
	}
	return nil
}
