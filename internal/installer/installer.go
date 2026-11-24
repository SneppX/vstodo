// Package installer implements VSTodo's user-scoped installer.
package installer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	ManifestName  = "installer.json"
	SchemaVersion = 1
)

type Paths struct {
	Root      string
	BinDir    string
	Binary    string
	Installer string
	Manifest  string
}

type Options struct {
	Root            string
	Source          string
	InstallerSource string
	Version         string
}

type FileEntry struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type Manifest struct {
	SchemaVersion int         `json:"schema_version"`
	Product       string      `json:"product"`
	Version       string      `json:"version"`
	OS            string      `json:"os"`
	Architecture  string      `json:"architecture"`
	InstalledAt   time.Time   `json:"installed_at"`
	Files         []FileEntry `json:"files"`
}

func ResolvePaths(root string) (Paths, error) {
	if root == "" {
		if runtime.GOOS == "windows" {
			base := os.Getenv("LOCALAPPDATA")
			if base == "" {
				var err error
				base, err = os.UserConfigDir()
				if err != nil {
					return Paths{}, fmt.Errorf("resolve user configuration directory: %w", err)
				}
			}
			root = filepath.Join(base, "Programs", "VSTodo")
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return Paths{}, fmt.Errorf("resolve home directory: %w", err)
			}
			root = filepath.Join(home, ".local", "share", "vstodo")
		}
	}
	root, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return Paths{}, fmt.Errorf("resolve install root: %w", err)
	}
	binDir := filepath.Join(root, "bin")
	binaryName := "vstodo"
	installerName := "vstodo-installer"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
		installerName += ".exe"
	}
	return Paths{
		Root:      root,
		BinDir:    binDir,
		Binary:    filepath.Join(binDir, binaryName),
		Installer: filepath.Join(binDir, installerName),
		Manifest:  filepath.Join(root, ManifestName),
	}, nil
}

func ensureInstallDirectories(paths Paths) error {
	if err := os.MkdirAll(paths.Root, 0755); err != nil {
		return err
	}
	if err := requireDirectory(paths.Root); err != nil {
		return err
	}
	if err := os.MkdirAll(paths.BinDir, 0755); err != nil {
		return err
	}
	return requireDirectory(paths.BinDir)
}

func validateInstallLayout(paths Paths, requireManifest bool) error {
	if err := requireDirectory(paths.Root); err != nil {
		return fmt.Errorf("unsafe installation root: %w", err)
	}
	if err := requireDirectory(paths.BinDir); err != nil {
		return fmt.Errorf("unsafe installation bin directory: %w", err)
	}
	if requireManifest {
		if err := requireRegularFile(paths.Manifest); err != nil {
			return fmt.Errorf("unsafe installer manifest: %w", err)
		}
	}
	return nil
}

func requireDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%q is not a real directory", path)
	}
	return nil
}

func requireRegularFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%q is not a regular file", path)
	}
	return nil
}

func Install(opts Options) (Manifest, error) {
	if strings.TrimSpace(opts.Source) == "" {
		return Manifest{}, errors.New("a VSTodo binary source is required")
	}
	paths, err := ResolvePaths(opts.Root)
	if err != nil {
		return Manifest{}, err
	}
	installerSource := opts.InstallerSource
	if installerSource == "" {
		installerSource, err = os.Executable()
		if err != nil {
			return Manifest{}, fmt.Errorf("resolve installer executable: %w", err)
		}
	}
	if err := ensureInstallDirectories(paths); err != nil {
		return Manifest{}, fmt.Errorf("prepare install directory: %w", err)
	}

	binaryHash, err := copyAtomic(opts.Source, paths.Binary)
	if err != nil {
		return Manifest{}, fmt.Errorf("install VSTodo binary: %w", err)
	}
	installerHash, err := copyAtomic(installerSource, paths.Installer)
	if err != nil {
		return Manifest{}, fmt.Errorf("install VSTodo installer: %w", err)
	}

	version := strings.TrimSpace(opts.Version)
	if version == "" {
		version = "dev"
	}
	manifest := Manifest{
		SchemaVersion: SchemaVersion,
		Product:       "VSTodo",
		Version:       version,
		OS:            runtime.GOOS,
		Architecture:  runtime.GOARCH,
		InstalledAt:   time.Now().UTC(),
		Files: []FileEntry{
			{Name: filepath.Base(paths.Binary), Path: paths.Binary, SHA256: binaryHash},
			{Name: filepath.Base(paths.Installer), Path: paths.Installer, SHA256: installerHash},
		},
	}
	if err := writeManifest(paths.Manifest, manifest); err != nil {
		return Manifest{}, fmt.Errorf("write installer manifest: %w", err)
	}
	return manifest, nil
}

func Verify(root string) (Manifest, error) {
	paths, err := ResolvePaths(root)
	if err != nil {
		return Manifest{}, err
	}
	if err := validateInstallLayout(paths, true); err != nil {
		return Manifest{}, err
	}
	manifest, err := readManifest(paths.Manifest)
	if err != nil {
		return Manifest{}, err
	}
	if manifest.SchemaVersion != SchemaVersion || manifest.Product != "VSTodo" {
		return Manifest{}, fmt.Errorf("unsupported VSTodo installer manifest schema or product")
	}
	expected := map[string]string{
		filepath.Base(paths.Binary):    paths.Binary,
		filepath.Base(paths.Installer): paths.Installer,
	}
	if len(manifest.Files) != len(expected) {
		return Manifest{}, fmt.Errorf("installer manifest contains an unexpected file count")
	}
	seen := make(map[string]bool, len(expected))
	for _, entry := range manifest.Files {
		expectedPath, ok := expected[entry.Name]
		if !ok || seen[entry.Name] || !samePath(entry.Path, expectedPath) {
			return Manifest{}, fmt.Errorf("installer manifest contains an unsafe or duplicate path for %q", entry.Name)
		}
		seen[entry.Name] = true
		if err := requireRegularFile(entry.Path); err != nil {
			return Manifest{}, fmt.Errorf("verify %s: %w", entry.Name, err)
		}
		actual, err := fileSHA256(entry.Path)
		if err != nil {
			return Manifest{}, fmt.Errorf("verify %s: %w", entry.Name, err)
		}
		if !strings.EqualFold(actual, entry.SHA256) {
			return Manifest{}, fmt.Errorf("checksum mismatch for %s", entry.Name)
		}
	}
	if len(seen) != len(expected) {
		return Manifest{}, errors.New("installer manifest is missing an installed file")
	}
	return manifest, nil
}

func Uninstall(root string) (Paths, error) {
	paths, err := ResolvePaths(root)
	if err != nil {
		return Paths{}, err
	}
	if err := validateInstallLayout(paths, true); err != nil {
		return Paths{}, err
	}
	manifest, err := readManifest(paths.Manifest)
	if err != nil {
		return Paths{}, err
	}
	expected := map[string]string{
		filepath.Base(paths.Binary):    paths.Binary,
		filepath.Base(paths.Installer): paths.Installer,
	}
	if manifest.SchemaVersion != SchemaVersion || manifest.Product != "VSTodo" || len(manifest.Files) != len(expected) {
		return Paths{}, errors.New("refusing to uninstall an invalid VSTodo manifest")
	}
	seen := make(map[string]bool, len(expected))
	for _, entry := range manifest.Files {
		expectedPath, ok := expected[entry.Name]
		if !ok || seen[entry.Name] || !samePath(entry.Path, expectedPath) {
			return Paths{}, fmt.Errorf("refusing to remove unexpected or duplicate path for %q", entry.Name)
		}
		seen[entry.Name] = true
	}
	if len(seen) != len(expected) {
		return Paths{}, errors.New("refusing to uninstall an incomplete VSTodo manifest")
	}
	currentExecutable, executableErr := os.Executable()
	if executableErr != nil {
		return Paths{}, fmt.Errorf("resolve running installer executable: %w", executableErr)
	}
	deferredSelfRemoval := false
	for _, entry := range manifest.Files {
		if runtime.GOOS == "windows" && samePath(entry.Path, currentExecutable) {
			deferredSelfRemoval = true
			continue
		}
		if err := os.Remove(entry.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return Paths{}, fmt.Errorf("remove %s: %w", entry.Name, err)
		}
	}
	if err := os.Remove(paths.Manifest); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Paths{}, fmt.Errorf("remove installer manifest: %w", err)
	}
	if deferredSelfRemoval {
		command := fmt.Sprintf("timeout /t 2 /nobreak >nul & del /f /q \"%s\" & rmdir \"%s\" & rmdir \"%s\"", paths.Installer, paths.BinDir, paths.Root)
		if err := exec.Command("cmd.exe", "/C", command).Start(); err != nil {
			return Paths{}, fmt.Errorf("schedule removal of running installer: %w", err)
		}
	} else {
		_ = os.Remove(paths.BinDir)
		_ = os.Remove(paths.Root)
	}
	return paths, nil
}

func AddToUserPath(dir string) error {
	dir, err := filepath.Abs(filepath.Clean(dir))
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return updateWindowsPath(dir, true)
	}
	return updateShellPath(dir, true)
}

func RemoveFromUserPath(dir string) error {
	dir, err := filepath.Abs(filepath.Clean(dir))
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return updateWindowsPath(dir, false)
	}
	return updateShellPath(dir, false)
}

func copyAtomic(source, destination string) (string, error) {
	if samePath(source, destination) {
		return fileSHA256(source)
	}
	if runtime.GOOS == "windows" {
		if currentExecutable, err := os.Executable(); err == nil && samePath(currentExecutable, destination) {
			return fileSHA256(destination)
		}
	}
	src, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer src.Close()

	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(destination), ".vstodo-install-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, hash), src); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := tmp.Chmod(0755); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}

	backup := destination + ".vstodo-backup"
	_ = os.Remove(backup)
	hadDestination := false
	if _, err := os.Stat(destination); err == nil {
		if err := os.Rename(destination, backup); err != nil {
			return "", fmt.Errorf("move previous installation aside: %w", err)
		}
		hadDestination = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := os.Rename(tmpName, destination); err != nil {
		if hadDestination {
			_ = os.Rename(backup, destination)
		}
		return "", fmt.Errorf("activate new binary: %w", err)
	}
	if hadDestination {
		_ = os.Remove(backup)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func writeManifest(path string, manifest Manifest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(path, append(data, '\n'), 0644)
}

func readManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read installer manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode installer manifest: %w", err)
	}
	return manifest, nil
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".vstodo-manifest-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// Windows cannot rename over an existing file. Move the old manifest
	// aside first and restore it if activation of the new manifest fails.
	backup := path + ".vstodo-backup"
	_ = os.Remove(backup)
	hadExisting := false
	if _, err := os.Lstat(path); err == nil {
		if err := os.Rename(path, backup); err != nil {
			return fmt.Errorf("preserve previous manifest: %w", err)
		}
		hadExisting = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		if hadExisting {
			_ = os.Rename(backup, path)
		}
		return fmt.Errorf("activate new manifest: %w", err)
	}
	if hadExisting {
		_ = os.Remove(backup)
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func samePath(a, b string) bool {
	if abs, err := filepath.Abs(a); err == nil {
		a = abs
	}
	if abs, err := filepath.Abs(b); err == nil {
		b = abs
	}
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func updateShellPath(dir string, add bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	shell := strings.ToLower(filepath.Base(os.Getenv("SHELL")))
	profile := filepath.Join(home, ".profile")
	if shell == "zsh" {
		profile = filepath.Join(home, ".zshrc")
	} else if shell == "fish" {
		profile = filepath.Join(home, ".config", "fish", "conf.d", "vstodo.fish")
	}
	data, err := os.ReadFile(profile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	content := string(data)
	const begin = "# >>> VSTodo managed PATH >>>"
	const end = "# <<< VSTodo managed PATH <<<"
	start := strings.Index(content, begin)
	if start >= 0 {
		finish := strings.Index(content[start:], end)
		if finish >= 0 {
			finish += start + len(end)
			content = strings.TrimRight(content[:start]+content[finish:], "\r\n")
		}
	}
	if add {
		quoted := strings.ReplaceAll(dir, "\\", "\\\\")
		quoted = strings.ReplaceAll(quoted, "\"", "\\\"")
		pathLine := "export PATH=\"" + quoted + ":$PATH\""
		if shell == "fish" {
			pathLine = "set -gx PATH \"" + quoted + "\" $PATH"
		}
		block := begin + "\n" + pathLine + "\n" + end + "\n"
		if content != "" {
			content += "\n"
		}
		content += block
	}
	mode := os.FileMode(0644)
	if info, err := os.Stat(profile); err == nil {
		mode = info.Mode().Perm()
	}
	return writeAtomic(profile, []byte(content), mode)
}

func updateWindowsPath(dir string, add bool) error {
	key := `HKCU\Environment`
	out, err := exec.Command("reg.exe", "query", key, "/v", "Path").CombinedOutput()
	current := ""
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "REG_EXPAND_SZ") {
				parts := strings.SplitN(line, "REG_EXPAND_SZ", 2)
				current = strings.TrimSpace(parts[1])
				break
			}
			if strings.Contains(line, "REG_SZ") {
				parts := strings.SplitN(line, "REG_SZ", 2)
				current = strings.TrimSpace(parts[1])
				break
			}
		}
	} else if !add {
		return nil
	}
	parts := strings.Split(current, ";")
	filtered := make([]string, 0, len(parts)+1)
	found := false
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.EqualFold(strings.TrimRight(part, `\/`), strings.TrimRight(dir, `\/`)) {
			found = true
			if !add {
				continue
			}
		}
		filtered = append(filtered, part)
	}
	if add && !found {
		filtered = append(filtered, dir)
	}
	value := strings.Join(filtered, ";")
	if value == current {
		return nil
	}
	if value == "" {
		_, err = exec.Command("reg.exe", "delete", key, "/v", "Path", "/f").CombinedOutput()
		if err != nil && add {
			return fmt.Errorf("remove user PATH entry: %w", err)
		}
		return nil
	}
	out, err = exec.Command("reg.exe", "add", key, "/v", "Path", "/t", "REG_EXPAND_SZ", "/d", value, "/f").CombinedOutput()
	if err != nil {
		return fmt.Errorf("update user PATH: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
