package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"vstodo/internal/repo"
)

func TestWalkFilesExcludesConfiguredMetadataDirectory(t *testing.T) {
	root := t.TempDir()
	t.Setenv(repo.EnvOverride, filepath.Join(".metadata", "vstodo"))

	for path, content := range map[string]string{
		"README.md":               "project files stay visible",
		".metadata/vstodo/config": "private VSTodo metadata",
		".git/config":             "Git metadata",
	} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	files, err := walkFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(files, []string{"README.md"}) {
		t.Fatalf("walkFiles returned %v, want [README.md]", files)
	}
}

func TestWalkFilesExcludesDefaultMetadataDirectory(t *testing.T) {
	root := t.TempDir()
	t.Setenv(repo.EnvOverride, "")
	for path, content := range map[string]string{
		"main.go":        "package main",
		".vstodo/config": "private VSTodo metadata",
	} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files, err := walkFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(files, []string{"main.go"}) {
		t.Fatalf("walkFiles returned %v, want [main.go]", files)
	}
}

func TestWalkFilesRespectsVSTodoIgnore(t *testing.T) {
	root := t.TempDir()
	t.Setenv(repo.EnvOverride, "")
	for path, content := range map[string]string{
		".vstodoignore": "ignored.log\nbuild/\n",
		"keep.txt":      "keep",
		"ignored.log":   "skip",
		"build/out.o":   "skip",
		"src/app.go":    "keep",
	} {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files, err := walkFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".vstodoignore", "keep.txt", "src/app.go"}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("walkFiles returned %v, want %v", files, want)
	}
}
