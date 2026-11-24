// Package installer implements VSTodo's user-scoped, transactional installer.
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
	ManifestName = "installer.json"
	SchemaVersion = 1
)

type Paths struct {
	Root     string
	BinDir   string
	Binary   string
	Installer string
	Manifest string
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
		var err error
		if runtime.GOOS == "windows" {
			base, e := os.UserConfigDir()
			if e != nil {
				return Paths{}, fmt.Errorf("resolve user configuration directory: %w", e)
			}
			root = filepath.Join(base, "Programs", "VSTodo")
		} else {
			home, e := os.UserHomeDir()
			if e != nil {
				return Paths{}, fmt.Errorf("resolve home directory: %w", e)
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
		Root: root,
		BinDir: binDir,
		Binary: filepath.Join(binDir, binaryName),
		Installer: filepath.Join(binDir, installerName),
		Manifest: filepath.Join(root, ManifestName),
	}, nil
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
	if err := os.MkdirAll(paths.BinDir, 0755); err != nil {
		return Manifest{}, fmt.Errorf("create install directory: %w", err)
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
		Product: "VSTodo",
		Version: version,
		OS: runtime.GOOS,
		Architecture: runtime.GOARCH,
		InstalledAt: time.Now().UTC(),
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
	manifest, err := readManifest(paths.Manifest)
	if err != nil {
		return Manifest{}, err
	}
	if manifest.SchemaVersion != SchemaVersion || manifest.Product != "VSTodo" {
		return Manifest{}, fmt.Errorf("unsupported VSTodo installer manifest schema or product")
	}
	expected := map[string]string{
		filepath.Base(paths.Binary): paths.Binary,
		filepath.Base(paths.Installer): paths.Installer,
	}
	if len(manifest.Files) != len(expected) {
		return Manifest{}, fmt.Errorf("installer manifest contains an unexpected file count")
	}
	for _, entry := range manifest.Files {
		expectedPath, ok := expected[entry.Name]
		if !ok || !samePath(entry.Path, expectedPath) {
			return Manifest{}, fmt.Errorf("installer manifest contains an unsafe path for %q", entry.Name)
		}
		actual, err := fileSHA256(entry.Path)
		if err != nil {
			return Manifest{}, fmt.Errorf("verify %s: %w", entry.Name, err)
		}
		if !strings.EqualFold(actual, entry.SHA256) {
			return Manifest{}, fmt.Errorf("checksum mismatch for %s", entry.Name)
		}
	}
	return manifest, nil
}

func Uninstall(root string) (Paths, error) {
	paths, err := ResolvePaths(root)
	if err != nil {
		return Paths{}, err
	}
	manifest, err := readManifest(paths.Manifest)
	if err != nil {
		return Paths{}, err
	}
	expected := map[string]string{
		filepath.Base(paths.Binary): paths.Binary,
		filepath.Base(paths.Installer): paths.Installer,
	}
	for _, entry := range manifest.Files {
		expectedPath, ok := expected[entry.Name]
		if !ok || !samePath(entry.Path, expectedPath) {
			return Paths{}, fmt.Errorf("refusing to remove unexpected path for %q", entry.Name)
		}
		if err := os.Remove(expectedPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return Paths{}, fmt.Errorf("remove %s: %w", entry.Name, err)
		}
	}
	if err := os.Remove(paths.Manifest); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Paths{}, fmt.Errorf("remove installer manifest: %w", err)
	}
	_ = os.Remove(paths.BinDir)
	_ = os.Remove(paths.Root)
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
	if runtime.GOOS == "windows" {
		_ = os.Remove(path)
	}
	return os.Rename(tmpName, path)
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
	profile := filepath.Join(home, ".profile")
	if strings.HasSuffix(strings.ToLower(os.Getenv("SHELL")), "zsh") {
		profile = filepath.Join(home, ".zshrc")
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
		block := begin + "\nexport PATH=\"" + quoted + ":$PATH\"\n" + end + "\n"
		if content != "" {
			content += "\n"
		}
		content += block
	}
	return writeAtomic(profile, []byte(content), 0644)
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
