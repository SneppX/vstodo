# Architecture

`vstodo` is an independently implemented version-control system written in Go. It does not copy or reverse-engineer Git's source code. It uses Git-compatible object serialization and a subset of Git smart-HTTP for interoperability. It is a small, self-contained project with no external dependencies. The library lives under `internal/` and the command-line interface lives in package `main` at the repository root.

## Design goals

- **Content-addressed storage**: every object is named by the SHA-1 of its
  content, exactly as Git does. This gives deduplication, integrity, and
  immutability for free.
- **Git-compatible object model**: blobs, trees, commits, and annotated tags use
  the same header format and serialization as Git, so `vstodo` objects can be
  read by Git tooling (and vice versa) for the core object types.
- **Git-compatible index**: the staging area reads and writes Git index v2 files, preserving file modes and validating checksums. Legacy VSTodo text indexes are still accepted for migration.
- **Pluggable storage**: the object store is an interface (`object.Store`) with
  a loose-files implementation (`storage.LooseStore`). The storage package also
  provides a durable indexed-pack backend (`storage.PackStore`) for validated
  pack publication and object lookup.
- **HTTP remote interoperability**: the transport layer supports smart-HTTP
  reference discovery, clone, fetch, pull, and fast-forward-only branch push.
- **Embeddable library**: the `app.App` type wires together repo, store, index,
  and refs so Go programs can use `vstodo` as an embedded VCS.

## Module layout

```
vstodo/                      # module root, package main (CLI)
├── main.go                  # CLI entry point and command dispatch
├── util.go                  # shared helpers used by commands
├── cmd_*.go                 # one file per command (init, add, commit, ...)
├── go.mod                   # module vstodo, stdlib only
├── README.md
├── docs/                    # project documentation
├── examples/                # standalone embedded-VCS example programs
├── benchmark/               # Go benchmarks (go test -bench=.)
└── internal/                # library packages
    ├── app/                 # wires Repo, Store, Index, Refs into an App
    ├── commit/              # commit object build/parse
    ├── index/               # staging area (path -> blob digest)
    ├── object/              # object model, hashing, marshaling, zlib
    ├── packfile/            # packfile reader/writer and delta resolution
    ├── ref/                 # HEAD, branches, tags, remote-tracking refs
    ├── repo/                # repository discovery/init and path helpers
    ├── sha1/                # SHA-1 digest type and hashing helpers
    ├── storage/             # loose and durable indexed-pack storage
    ├── transport/           # HTTP smart-transport discovery/fetch/push
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

where the header is ASCII and `\0` is a single NUL byte. The full object is
zlib-compressed and stored at `.vstodo/objects/<first 2 hex>/<remaining 38 hex>`
when stored as a loose object.

## Storage layout

```
.vstodo/                     # repository metadata (configurable via VSTODO_DIR)
├── HEAD
├── config
├── description
├── index
├── objects/
│   ├── <2-hex>/<38-hex>     # immutable zlib-compressed loose objects
│   └── pack/
│       ├── pack-<sha1>.pack # immutable Git-compatible pack
│       └── pack-<sha1>.idx  # Git pack index v2
└── refs/
    ├── heads/
    └── tags/
```

Loose objects are read-only after creation (mode `0444`). The store is
append-only: storing the same content twice yields the same path and is a no-op
from an addressing perspective.

`storage.PackStore` validates a complete pack before publishing it. It writes
the checksum-named pack first and its v2 index second; readers only discover
packs through index files, so an interrupted publication cannot expose an
unindexed pack. Both files are published through synced temporary files and
atomic hard links. Existing immutable files are accepted only if their bytes
match. New object writes continue to use loose storage.

The initial pack-backed reader uses the validated pack decoder and caches a
decoded pack in memory after its first object lookup. This establishes durable
pack storage and restart-safe lookup, but it is not yet a bounded-memory,
random-access pack reader. The application fetch path and garbage collector
remain on the loose-store backend until the storage interface is integrated
end-to-end; received packs are not yet retained by the normal fetch command.

## Index

The index is `vstodo`'s staging area. It reads and writes Git index version 2
(`DIRC`) files, including the entry checksum, file modes, and optional
extension framing. Legacy VSTodo line-oriented indexes remain readable and are
converted to DIRC format on the next save. Index writes use an exclusive
sibling `.lock` file and atomic publication. Each loaded index retains its
original bytes; a save rejects a stale snapshot rather than overwriting another
process's newer staging state.

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
| Object store | `.vstodo/objects/<2>/<38>` and `.vstodo/objects/pack/*` | `.git/objects/*` |
| Object envelope | `<type> <size>\0<content>` | same |
| Compression | zlib (deflate) | zlib (deflate) |
| Hashing | SHA-1 | SHA-1 |
| Index | `.vstodo/index` (Git DIRC v2) | `.git/index` (binary) |
| HEAD | `.vstodo/HEAD` | `.git/HEAD` |
| Branches | `.vstodo/refs/heads/*` | `.git/refs/heads/*` |
| Tags | `.vstodo/refs/tags/*` | `.git/refs/tags/*` |
| Commit format | `tree`/`parent`/`author`/`committer`/blank/message | same |
| Tree entry | `<mode> <name>\0<20 bytes>` | same |

Because `vstodo` reuses Git's object formats byte-for-byte, its loose objects
and packfiles are compatible with Git's core object model. VSTodo currently
uses SHA-1 object IDs. It does not yet implement Git's complete configuration,
hooks, reflog, packed-refs, or every index extension and transport capability.
Compatibility is substantial but is not yet complete.

## Command families

Commands are split into three groups (see `vstodo help` and
`docs/COMMANDS.md`):

- **Core**: `init`, `add`, `commit`, `status`, `log`, `branch`, `checkout`,
  `diff`, `show`, `merge`, `tag`, `reset`, `rm`, `rebase`
- **Advanced**: `blame`, `bisect`, `stash`, `cherry-pick`, `notes`, `fsck`, `gc`,
  `worktree`, `bundle`
- **Plumbing**: `hash-object`, `cat-file`

The CLI entry point (`main.go`) routes the first argument to the matching
`cmd_*` function. Non-`init` commands require a repository to be present in the
current directory (or an ancestor).
