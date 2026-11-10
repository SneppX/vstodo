# Architecture

`vstodo` is a Git-like version control system, reverse-engineered from Git's
internals and extended beyond Git. It is a small, self-contained Go project with
no external dependencies. The library lives under `internal/` and the command-line
interface lives in package `main` at the repository root.

## Design goals

- **Content-addressed storage**: every object is named by the SHA-1 of its
  content, exactly as Git does. This gives deduplication, integrity, and
  immutability for free.
- **Git-compatible object model**: blobs, trees, commits, and annotated tags use
  the same header format and serialization as Git, so `vstodo` objects can be
  read by Git tooling (and vice versa) for the core object types.
- **Simple, human-readable index**: the staging area is a line-oriented text
  file (`<hex digest>\t<path>`) at `.vstodo/index` rather than Git's binary
  packed index.
- **Pluggable storage**: the object store is an interface (`object.Store`) with
  a loose-files implementation (`storage.LooseStore`). A packfile or database
  backend can be added without touching the rest of the code.
- **Embeddable library**: the `app.App` type wires together repo, store, index,
  and refs so Go programs can use `vstodo` as an embedded VCS.

## Module layout

```
vstodo/                      # module root, package main (CLI)
├── main.go                  # CLI entry point and command dispatch
├── util.go                  # shared helpers used by commands
├── cmd_*.go                 # one file per command (init, add, commit, ...)
├── go.mod                   # module vstodo, go 1.26.3, stdlib only
├── README.md
├── docs/                    # (this directory) project documentation
├── examples/                # standalone embedded-VCS example programs
└── benchmark/               # Go benchmarks (go test -bench=.)
└── internal/                # library packages
    ├── app/                 # wires Repo, Store, Index, Refs into an App
    ├── commit/              # commit object build/parse
    ├── index/               # staging area (path -> blob digest)
    ├── object/              # object model, hashing, marshaling, zlib
    ├── ref/                 # HEAD, branches, tags
    ├── repo/                # repository discovery/init and path helpers
    ├── sha1/                # SHA-1 digest type and hashing helpers
    ├── storage/             # loose object store (.vstodo/objects/<2>/<38>)
    ├── tree/                # tree object build/parse
    └── walk/                # commit-graph traversal
```

## Object model

`vstodo` has four object types, mirroring Git:

| Type | Purpose | Contains |
|------|---------|----------|
| `blob` | File content | raw file bytes |
| `tree` | Directory | list of `(mode, name, sha1)` entries |
| `commit` | Snapshot | pointer to a root tree, zero or more parent commits, author/committer, message |
| `tag` | Annotated reference | target object, target type, tagger, message |

All objects share the same on-disk envelope:

```
<type> <size in bytes>\0<content>
```

For example a blob holding `"hello\n"` is addressed by:

```
SHA1("blob 6\0hello\n")
```

where the header is ASCII and `\0` is a single NUL byte. The full object is then
zlib-compressed and stored at `.vstodo/objects/<first 2 hex>/<remaining 38 hex>`.

## Storage layout

```
.vstodo/                     # repository metadata (configurable via VSTODO_DIR)
├── HEAD                     # symbolic ref: ref: refs/heads/main
├── config                   # repository config
├── description              # repository description
├── index                    # staging area (text: <hex>\t<path>)
├── objects/                 # content-addressed object store
│   └── <2-hex>/             # loose object directories
│       └── <38-hex>         # zlib-compressed object file
└── refs/
    ├── heads/               # branches
    └── tags/                # tags
```

Loose objects are read-only after creation (mode `0444`). The store is
append-only: storing the same content twice yields the same path and is a
no-op from an addressing perspective.

## Index

The index is `vstodo`'s staging area. It maps working-tree paths to staged
blob digests. It is stored as a simple line-oriented text file:

```
<40-hex digest>\t<relative path>
```

One line per staged file. This format is intentionally trivial to read, write,
edit by hand, and diff — unlike Git's binary index format. Blank lines and
lines starting with `#` are ignored.

## Refs

Refs are plain text files under `.vstodo/refs/`:

- `HEAD` — either `ref: refs/heads/main` (symbolic) or a raw 40-hex digest
  (detached HEAD).
- `refs/heads/<name>` — branch tips (40-hex digest).
- `refs/tags/<name>` — annotated tags (40-hex digest).

Reading a ref follows symbolic pointers recursively.

## How it maps to Git

| Concept | `vstodo` | Git equivalent |
|---------|----------|----------------|
| Object store | `.vstodo/objects/<2>/<38>` | `.git/objects/<2>/<38>` |
| Object envelope | `<type> <size>\0<content>` | same |
| Compression | zlib (deflate) | zlib (deflate) |
| Hashing | SHA-1 | SHA-1 |
| Index | `.vstodo/index` (text) | `.git/index` (binary) |
| HEAD | `.vstodo/HEAD` | `.git/HEAD` |
| Branches | `.vstodo/refs/heads/*` | `.git/refs/heads/*` |
| Tags | `.vstodo/refs/tags/*` | `.git/refs/tags/*` |
| Commit format | `tree`/`parent`/`author`/`committer`/blank/message | same |
| Tree entry | `<mode> <name>\0<20 bytes>` | same |

Because `vstodo` reuses Git's object formats byte-for-byte, a `vstodo` repository's
`.vstodo/objects` is, for the core object types, a valid Git object store. The
main divergence is the index (text vs binary) and the absence of packfiles,
reflog, and the full Git configuration/hook ecosystem.

## Command families

Commands are split into three groups (see `vstodo help` and `docs/COMMANDS.md`):

- **Core**: `init`, `add`, `commit`, `status`, `log`, `branch`, `checkout`,
  `diff`, `show`, `merge`, `tag`, `reset`, `rm`, `rebase`
- **Advanced**: `blame`, `bisect`, `stash`, `cherry-pick`, `notes`, `gc`,
  `worktree`, `bundle`
- **Plumbing**: `hash-object`, `cat-file`

The CLI entry point (`main.go`) routes the first argument to the matching
`cmd_*` function. Non-`init` commands require a repository to be present in the
current directory (or an ancestor).
