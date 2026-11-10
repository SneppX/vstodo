# Hacking

This document explains how to build, test, extend, and contribute to `vstodo`.

## Prerequisites

- Go 1.26.3 or later (see `go.mod`).
- No external dependencies. `GOPROXY=off` works because only the standard
  library is used.

## Build

Build the CLI binary:

```sh
go build -o vstodo .
```

Run it:

```sh
./vstodo init myrepo
cd myrepo
echo hello > hello.txt
./vstodo add hello.txt
./vstodo commit -m "Add hello"
./vstodo log
```

Build all packages (including docs-adjacent tooling if any):

```sh
go build ./...
```

## Test

There are no committed unit tests in the core packages at this time. The
repository ships benchmarks under `benchmark/` that double as integration
coverage. Run them:

```sh
go test -bench=. ./benchmark/
```

To run benchmarks across the whole module:

```sh
go test -bench=. ./...
```

To benchmark a single function (e.g. object hashing):

```sh
go test -bench=BenchmarkHashBlob -run=^$ ./benchmark/
```

Benchmark output includes `ns/op`, `allocs/op`, and `MB/s` where applicable.

## Lint / typecheck

There is no separate linter configured. The Go compiler's typecheck is the
primary static check:

```sh
go vet ./...
go build ./...
```

If you add third-party lint tooling, add it here and to CI.

## Repository layout for contributors

```
internal/app      # App wiring: repo, store, index, refs
internal/commit   # commit object build/parse
internal/index    # staging area (path -> digest)
internal/object   # object model, marshaling, hashing, zlib
internal/ref      # HEAD, branches, tags
internal/repo     # discovery, init, path helpers
internal/sha1     # SHA-1 digest type
internal/storage  # loose object store
internal/tree     # tree object build/parse
internal/walk     # commit graph traversal
```

`cmd_*.go` files (package `main`) implement individual commands. `util.go`
holds helpers shared across commands. `main.go` dispatches.

### Adding a new command

1. Create `cmd_<name>.go` at the module root in `package main`:

```go
package main

import (
    "fmt"
    "vstodo/internal/app"
)

func cmd<Name>(a *app.App, args []string) error {
    // ...
    return nil
}
```

2. Add a `case "<name>":` branch to the switch in `main.go`, plus a usage line
   in `printUsage`.
3. Document it in `docs/COMMANDS.md`.

### Adding a storage backend

Implement `object.Store`:

```go
type Store interface {
    Store(obj *object.Object) (sha1.Digest, error)
    Load(id sha1.Digest) (*object.Object, error)
    Has(id sha1.Digest) (bool, error)
}
```

Then construct your `App` manually instead of using `app.New`/`app.Init`:

```go
store := &MyStore{...}
idx := index.NewIndex(repo.GitPath("index"))
refs := ref.NewStore(repo.GitDir)
app := &app.App{Repo: repo, Store: store, Index: idx, Refs: refs}
```

### Adding an object type

The current types are `blob`, `tree`, `commit`, `tag`. Adding a fifth type
requires:

1. Add a constant to `object.Type` and register it in `validTypes`.
2. Update `object.Unmarshal`'s validity check (it consults `validTypes`).
3. Add serialization helpers if the type needs structured build/parse (like
   `commit.Build`/`commit.Parse`).
4. Update `docs/INTERNALS.md` and `docs/ARCHITECTURE.md`.

## Code style

- Follow the conventions of the surrounding file: package doc comment, exported
  types/functions with doc comments, no unnecessary comments inside function
  bodies.
- Standard library only. Do not add `require` directives to `go.mod`.
- Errors are returned, not panicked. Use `fmt.Errorf` with `%w` for wrapping.
- File paths use `path/filepath` and are displayed with forward slashes in
  CLI output where appropriate.
- Tests and benchmarks use the standard `testing` package.

## Releasing

`vstodo` is versioned by the `go.mod` Go directive and git tags (if a remote is
added). There is no automated release process yet.

## Contributing

1. Fork the repo, create a feature branch.
2. Add code, docs, examples, or benchmarks.
3. Ensure `go build ./...`, `go vet ./...`, and `go test -bench=. ./...`
   pass.
4. Open a pull request with a summary of changes and which docs were updated.

## Directory references

- `docs/ARCHITECTURE.md` — overall design and object model
- `docs/INTERNALS.md` — on-disk formats and deep dive
- `docs/COMMANDS.md` — command reference
- `docs/COMPARISON.md` — vstodo vs Git
- `examples/` — embedded VCS example programs
- `benchmark/` — Go benchmarks
