package main

import (
	"fmt"
	"os"
	"path/filepath"

	"vstodo/internal/app"
	"vstodo/internal/commit"
	"vstodo/internal/index"
	"vstodo/internal/object"
	"vstodo/internal/sha1"
)

// cmdDemo initializes a vstodo repo in the current directory and populates it
// with a small demo project: multiple files, several commits, a feature branch,
// and a merge, so users can immediately see vstodo working.
func cmdDemo(args []string) error {
	target := "."
	if len(args) > 0 {
		target = args[0]
	}

	a, err := app.Init(target)
	if err != nil {
		return err
	}

	write := func(rel, data string) error {
		full := a.Repo.Path(rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		return os.WriteFile(full, []byte(data), 0o644)
	}

	doCommit := func(msg string) error {
		files, err := walkFiles(a.Repo.Root)
		if err != nil {
			return err
		}
		for _, f := range files {
			data, err := os.ReadFile(a.Repo.Path(f))
			if err != nil {
				return err
			}
			id := object.HashBlob(data)
			if _, err := a.Store.Store(&object.Object{Type: object.BlobType, Data: data}); err != nil {
				return err
			}
			a.Index.Add(f, id)
		}
		if err := a.SaveIndex(); err != nil {
			return err
		}
		treeID, err := buildTreeFromIndex(a, a.Index.Entries(), "")
		if err != nil {
			return err
		}
		var parents []sha1.Digest
		head, err := currentCommitID(a)
		if err == nil && head != sha1.Zero {
			parents = append(parents, head)
		}
		cb := commit.Build(treeID, parents, "vstodo", "vstodo", msg)
		id := object.HashCommit(cb)
		if _, err := a.Store.Store(&object.Object{Type: object.CommitType, Data: cb}); err != nil {
			return err
		}
		branch, err := a.Refs.CurrentBranch()
		if err != nil || branch == "" {
			branch = "main"
		}
		if err := a.Refs.WriteRef("refs/heads/"+branch, id); err != nil {
			return err
		}
		if err := a.Refs.SetHead("ref: refs/heads/" + branch); err != nil {
			return err
		}
		a.Index = index.NewIndex(a.Repo.GitPath("index"))
		if err := a.Index.Load(); err != nil {
			return err
		}
		return nil
	}

	if err := write("README.md", "# Demo\n\nWelcome to vstodo.\n"); err != nil {
		return err
	}
	if err := write("src/hello.go", "package main\n\nfunc main() {\n\tprintln(\"hello, vstodo\")\n}\n"); err != nil {
		return err
	}
	if err := doCommit("chore: initial demo skeleton"); err != nil {
		return err
	}

	if err := write("src/hello.go", "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello, vstodo!\")\n}\n"); err != nil {
		return err
	}
	if err := doCommit("feat: use fmt.Println"); err != nil {
		return err
	}

	if err := write("src/utils.go", "package utils\n\nfunc Greet(name string) string {\n\treturn \"hello, \" + name\n}\n"); err != nil {
		return err
	}
	if err := write("src/hello.go", "package main\n\nimport (\n\t\"fmt\"\n\t\"vstodo/utils\"\n)\n\nfunc main() {\n\tfmt.Println(utils.Greet(\"vstodo\"))\n}\n"); err != nil {
		return err
	}
	if err := doCommit("feat: add utils.Greet"); err != nil {
		return err
	}

	// feature branch
	if err := a.Refs.WriteRef("refs/heads/feature", headOrZero(a)); err != nil {
		return err
	}
	if err := a.Refs.WriteRefSymbolic("HEAD", "refs/heads/feature"); err != nil {
		return err
	}
	a.Index = index.NewIndex(a.Repo.GitPath("index"))
	if err := a.Index.Load(); err != nil {
		return err
	}
	if err := write("src/feature.go", "package feature\n\nfunc New() string {\n\treturn \"feature\"\n}\n"); err != nil {
		return err
	}
	if err := doCommit("feat(feature): add feature.go"); err != nil {
		return err
	}

	// merge feature into main
	if err := a.Refs.WriteRefSymbolic("HEAD", "refs/heads/main"); err != nil {
		return err
	}
	a.Index = index.NewIndex(a.Repo.GitPath("index"))
	if err := a.Index.Load(); err != nil {
		return err
	}
	if err := write("MERGE_INFO.md", "Feature merged into main.\n"); err != nil {
		return err
	}
	if err := doCommit("merge: merge feature into main"); err != nil {
		return err
	}

	fmt.Printf("Demo vstodo repository initialized in %s\n", a.Repo.Root)
	fmt.Println("Try: vstodo log, vstodo branch, vstodo show HEAD")
	return nil
}

func headOrZero(a *app.App) sha1.Digest {
	d, err := a.Refs.Head()
	if err != nil {
		return sha1.Zero
	}
	return d
}
