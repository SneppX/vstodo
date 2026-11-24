package installer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallVerifyRepairAndUninstall(t *testing.T) {
	root := filepath.Join(t.TempDir(), "VSTodo")
	source := filepath.Join(t.TempDir(), "vstodo-source")
	installerSource := filepath.Join(t.TempDir(), "installer-source")
	if err := os.WriteFile(source, []byte("vstodo binary v1"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installerSource, []byte("installer binary"), 0755); err != nil {
		t.Fatal(err)
	}

	manifest, err := Install(Options{
		Root:            root,
		Source:          source,
		InstallerSource: installerSource,
		Version:         "0.2.0",
	})
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if manifest.Version != "0.2.0" || len(manifest.Files) != 2 {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
	if _, err := Verify(root); err != nil {
		t.Fatalf("Verify() after install error = %v", err)
	}

	paths, err := ResolvePaths(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.Binary, []byte("tampered"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := Verify(root); err == nil {
		t.Fatal("Verify() accepted a modified binary")
	}

	if _, err := Install(Options{
		Root:            root,
		Source:          source,
		InstallerSource: installerSource,
		Version:         "0.2.0",
	}); err != nil {
		t.Fatalf("repair via Install() error = %v", err)
	}
	if _, err := Verify(root); err != nil {
		t.Fatalf("Verify() after repair error = %v", err)
	}
	if _, err := Uninstall(root); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	if _, err := os.Stat(paths.Binary); !os.IsNotExist(err) {
		t.Fatalf("binary still exists after uninstall, stat error = %v", err)
	}
	if _, err := os.Stat(paths.Manifest); !os.IsNotExist(err) {
		t.Fatalf("manifest still exists after uninstall, stat error = %v", err)
	}
}

func TestInstallRequiresSource(t *testing.T) {
	if _, err := Install(Options{Root: t.TempDir()}); err == nil {
		t.Fatal("Install() without source succeeded")
	}
}

func TestUninstallRejectsManifestPathTraversal(t *testing.T) {
	root := filepath.Join(t.TempDir(), "VSTodo")
	source := filepath.Join(t.TempDir(), "vstodo-source")
	installerSource := filepath.Join(t.TempDir(), "installer-source")
	outside := filepath.Join(t.TempDir(), "keep-me.txt")
	for path, data := range map[string]string{
		source:          "vstodo binary",
		installerSource: "installer binary",
		outside:         "do not delete",
	} {
		if err := os.WriteFile(path, []byte(data), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Install(Options{Root: root, Source: source, InstallerSource: installerSource, Version: "test"}); err != nil {
		t.Fatal(err)
	}
	paths, err := ResolvePaths(root)
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := readManifest(paths.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Files[0].Path = outside
	if err := writeManifest(paths.Manifest, manifest); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(root); err == nil {
		t.Fatal("Uninstall() accepted a path outside the install root")
	}
	if data, err := os.ReadFile(outside); err != nil || string(data) != "do not delete" {
		t.Fatalf("outside file was changed or removed: data=%q err=%v", data, err)
	}
}
