//go:build ignore

// gen_demo generates a large demo vstodo repository with 50+ commits across
// multiple branches, merges, and file evolution. It uses the vstodo API
// directly (package app/storage/commit/object/index/ref/sha1). Run with:
//
//	go run internal/testdata/gen_demo.go [output-dir]
//
// Default output: internal/testdata/demo
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/index"
	"vstodo/internal/object"
	"vstodo/internal/ref"
	"vstodo/internal/sha1"
	"vstodo/internal/tree"
)

func main() {
	out := "internal/testdata/demo"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	if err := os.RemoveAll(out); err != nil {
		fmt.Fprintf(os.Stderr, "gen_demo: remove: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "gen_demo: mkdir: %v\n", err)
		os.Exit(1)
	}

	a, err := app.Init(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gen_demo: init: %v\n", err)
		os.Exit(1)
	}

	writeFile := func(rel, data string) {
		full := filepath.Join(out, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			panic(err)
		}
		if err := os.WriteFile(full, []byte(data), 0o644); err != nil {
			panic(err)
		}
	}

	commitMsg := func(msg string, branch string) {
		files := listFiles(out)
		for _, f := range files {
			data, err := os.ReadFile(filepath.Join(out, f))
			if err != nil {
				panic(err)
			}
			id := object.HashBlob(data)
			if _, err := a.Store.Store(&object.Object{Type: object.BlobType, Data: data}); err != nil {
				panic(err)
			}
			a.Index.Add(f, id)
		}
		if err := a.SaveIndex(); err != nil {
			panic(err)
		}
		treeID, err := buildTreeFromIndex(a, a.Index.Entries(), "")
		if err != nil {
			panic(err)
		}
		var parents []sha1.Digest
		head, err := headID(a)
		if err == nil && head != sha1.Zero {
			parents = append(parents, head)
		}
		cb := commit.Build(treeID, parents, "dev <dev@example.com>", "dev <dev@example.com>", msg)
		id := object.HashCommit(cb)
		if _, err := a.Store.Store(&object.Object{Type: object.CommitType, Data: cb}); err != nil {
			panic(err)
		}
		if err := a.Refs.WriteRef("refs/heads/"+branch, id); err != nil {
			panic(err)
		}
		if err := a.Refs.SetHead("ref: refs/heads/" + branch); err != nil {
			panic(err)
		}
		a.Index = index.NewIndex(a.Repo.GitPath("index"))
		if err := a.Index.Load(); err != nil {
			panic(err)
		}
	}

	headID := func(a *app.App) (sha1.Digest, error) { return a.Refs.Head() }
	currentBranch := func(a *app.App) string {
		b, _ := a.Refs.CurrentBranch()
		if b == "" {
			b = "main"
		}
		return b
	}

	// ---- main branch: 30 commits ----
	writeFile("README.md", "# Project Alpha\n\nA demo vstodo repository.\n")
	writeFile("src/main.go", "package main\n\nfunc main() {\n\tprintln(\"hello, alpha\")\n}\n")
	writeFile("src/utils.go", "package utils\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n")
	writeFile("docs/design.md", "# Design\n\nInitial design.\n")
	commitMsg("chore: initial project skeleton", "main")

	writeFile("src/main.go", "package main\n\nimport \"vstodo/utils\"\n\nfunc main() {\n\tprintln(utils.Add(1, 2))\n}\n")
	commitMsg("feat: use utils.Add in main", "main")

	writeFile("src/utils.go", "package utils\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n\nfunc Sub(a, b int) int {\n\treturn a - b\n}\n")
	commitMsg("feat: add Sub to utils", "main")

	writeFile("docs/api.md", "# API\n\n- Add\n- Sub\n")
	commitMsg("docs: add api docs", "main")

	writeFile("src/main.go", "package main\n\nimport \"vstodo/utils\"\n\nfunc main() {\n\tprintln(utils.Add(10, 20))\n\tprintln(utils.Sub(20, 10))\n}\n")
	commitMsg("feat: print Add and Sub results", "main")

	for i := 6; i <= 30; i++ {
		writeFile("src/main.go", fmt.Sprintf("package main\n\nimport \"vstodo/utils\"\n\nfunc main() {\n\tprintln(\"iteration %d\")\n\tprintln(utils.Add(%d, %d))\n}\n", i, i, i*2))
		writeFile("CHANGELOG.md", fmt.Sprintf("- v%d: iteration %d\n", i, i))
		commitMsg(fmt.Sprintf("feat: iteration %d", i), "main")
	}

	// ---- feature branch ----
	mainHead := headOrZero(a)
	if err := a.Refs.WriteRef("refs/heads/feature", mainHead); err != nil {
		panic(err)
	}
	if err := switchBranch(a, "feature"); err != nil {
		panic(err)
	}
	writeFile("src/feature.go", "package feature\n\nfunc NewFeature() string {\n\treturn \"feature branch\"\n}\n")
	commitMsg("feat(feature): add feature.go", "feature")

	writeFile("src/main.go", "package main\n\nimport (\n\t\"vstodo/feature\"\n\t\"vstodo/utils\"\n)\n\nfunc main() {\n\tprintln(feature.NewFeature())\n\tprintln(utils.Add(1, 1))\n}\n")
	commitMsg("feat(feature): integrate feature into main", "feature")

	writeFile("tests/feature_test.go", "package tests\n\nimport \"testing\"\n\nfunc TestFeature(t *testing.T) {\n\tif feature.NewFeature() != \"feature branch\" {\n\t\tt.Fatal(\"unexpected\")\n\t}\n}\n")
	commitMsg("test(feature): add feature test", "feature")

	// ---- hotfix branch from older main ----
	if err := switchBranch(a, "main"); err != nil {
		panic(err)
	}
	writeFile("src/hotfix.go", "package hotfix\n\nfunc Patch() bool {\n\treturn true\n}\n")
	commitMsg("fix(hotfix): critical security patch", "main")

	writeFile("src/main.go", "package main\n\nimport \"vstodo/hotfix\"\n\nfunc main() {\n\tprintln(hotfix.Patch())\n}\n")
	commitMsg("fix(hotfix): apply patch in main", "main")

	// ---- merge feature into main ----
	writeFile("MERGE_NOTES.md", "Merged feature and hotfix into main.\n")
	commitMsg("merge: combine feature and hotfix branches", "main")

	// ---- more main evolution: 15 commits ----
	for i := 31; i <= 45; i++ {
		writeFile("src/main.go", fmt.Sprintf("package main\n\nfunc main() {\n\tprintln(\"release %d\")\n}\n", i))
		writeFile("docs/release.md", fmt.Sprintf("# Release %d\n\nReleased on day %d.\n", i, i))
		commitMsg(fmt.Sprintf("feat: release %d", i), "main")
	}

	// ---- experiment branch ----
	if err := a.Refs.WriteRef("refs/heads/experiment", headOrZero(a)); err != nil {
		panic(err)
	}
	if err := switchBranch(a, "experiment"); err != nil {
		panic(err)
	}
	writeFile("src/experiment.go", "package experiment\n\nfunc Run() string {\n\treturn \"experiment\"\n}\n")
	commitMsg("feat(experiment): start experiment", "experiment")

	writeFile("src/main.go", "package main\n\nimport \"vstodo/experiment\"\n\nfunc main() {\n\tprintln(experiment.Run())\n}\n")
	commitMsg("feat(experiment): use experiment in main", "experiment")

	writeFile("large/data.bin", strings.Repeat("binary-data-chunk-", 500))
	commitMsg("feat(experiment): add large binary asset", "experiment")

	writeFile("large/data.bin", strings.Repeat("binary-data-chunk-v2-", 500))
	commitMsg("feat(experiment): update large binary asset", "experiment")

	// ---- merge experiment into main ----
	if err := switchBranch(a, "main"); err != nil {
		panic(err)
	}
	writeFile("EXPERIMENT_MERGE.md", "Experiment merged into main.\n")
	commitMsg("merge: merge experiment into main", "main")

	// ---- final commits ----
	for i := 46; i <= 55; i++ {
		writeFile("src/main.go", fmt.Sprintf("package main\n\nfunc main() {\n\tprintln(\"final %d\")\n}\n", i))
		commitMsg(fmt.Sprintf("feat: final %d", i), "main")
	}

	fmt.Printf("Demo repo generated at %s\n", out)
}

func headOrZero(a *app.App) sha1.Digest {
	d, err := a.Refs.Head()
	if err != nil {
		return sha1.Zero
	}
	return d
}

func switchBranch(a *app.App, branch string) error {
	id, err := a.Refs.ReadRef("refs/heads/" + branch)
	if err != nil {
		return err
	}
	if err := a.Refs.WriteRef("HEAD", id); err != nil {
		return err
	}
	return a.Refs.WriteRefSymbolic("HEAD", "refs/heads/"+branch)
}

func buildTreeFromIndex(a *app.App, entries map[string]sha1.Digest, prefix string) (sha1.Digest, error) {
	type dirEntry struct {
		mode string
		name string
		id   sha1.Digest
	}
	subdirs := make(map[string]map[string]sha1.Digest)
	var tops []dirEntry
	for path, id := range entries {
		rel := strings.TrimPrefix(path, prefix)
		rel = strings.TrimLeft(rel, "/")
		parts := strings.SplitN(rel, string(filepath.Separator), 2)
		name := parts[0]
		if len(parts) == 1 {
			tops = append(tops, dirEntry{mode: "100644", name: name, id: id})
		} else {
			sub := parts[0]
			if subdirs[sub] == nil {
				subdirs[sub] = make(map[string]sha1.Digest)
			}
			subdirs[sub][prefix+rel] = id
		}
	}
	var te []tree.Entry
	for _, t := range tops {
		te = append(te, tree.Entry{Mode: t.mode, Name: t.name, ID: t.id})
	}
	for sub, subents := range subdirs {
		subID, err := buildTreeFromIndex(a, subents, prefix+sub+string(filepath.Separator))
		if err != nil {
			return sha1.Zero, err
		}
		te = append(te, tree.Entry{Mode: "040000", Name: sub, ID: subID})
	}
	t := &tree.Tree{Entries: te}
	data := t.Marshal()
	if _, err := a.Store.Store(&object.Object{Type: object.TreeType, Data: data}); err != nil {
		return sha1.Zero, err
	}
	return object.HashTree(data), nil
}

// listFiles lists regular files under root (excluding .vstodo), relative paths.
func listFiles(root string) []string {
	var out []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if strings.HasPrefix(rel, ".vstodo"+string(filepath.Separator)) || rel == ".vstodo" {
			return nil
		}
		out = append(out, rel)
		return nil
	})
	return out
}
