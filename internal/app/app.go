// Package app wires together a repository, its object store, index, and ref store.
// Commands receive an *App to perform reads/writes.
package app

import (
	"os"

	"vstodo/internal/index"
	"vstodo/internal/ref"
	"vstodo/internal/repo"
	"vstodo/internal/storage"
)

// App is the wired vstodo context.
type App struct {
	Repo  *repo.Repository
	Store *storage.LooseStore
	Index *index.Index
	Refs  *ref.Store
}

// New opens (or creates) the repo at root and wires all subsystems.
func New(root string) (*App, error) {
	r, err := repo.Find(root)
	if err != nil {
		return nil, err
	}
	store := storage.NewLooseStore(r.ObjectsDir())
	idx := index.NewIndex(r.GitPath("index"))
	if err := idx.Load(); err != nil {
		return nil, err
	}
	refs := ref.NewStore(r.GitDir)
	return &App{Repo: r, Store: store, Index: idx, Refs: refs}, nil
}

// Init creates a fresh repo at root and returns a wired App.
func Init(root string) (*App, error) {
	r, err := repo.Init(root)
	if err != nil {
		return nil, err
	}
	store := storage.NewLooseStore(r.ObjectsDir())
	idx := index.NewIndex(r.GitPath("index"))
	refs := ref.NewStore(r.GitDir)
	return &App{Repo: r, Store: store, Index: idx, Refs: refs}, nil
}

// SaveIndex persists the staging snapshot and records that the repository
// uses the full-snapshot index semantics introduced by current VSTodo.
func (a *App) SaveIndex() error {
	if err := a.Index.Save(); err != nil {
		return err
	}
	return os.WriteFile(a.Repo.GitPath("index.snapshot-v1"), []byte("1\\n"), 0o644)
}
