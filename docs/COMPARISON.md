# Comparison: vstodo vs Git

`vstodo` is a Git-like version control system reverse-engineered from Git's
internals and deliberately extended beyond it. This document enumerates how
`vstodo` differs from Git, where it is simpler, where it is more advanced, and
what trade-offs that entails.

## Core philosophy

Git is a sprawling system accumulated over two decades: packfiles, reflogs,
hooks, configuration, submodules, worktrees, remotes, partial clones, and more.
`vstodo` takes a minimalist core — objects, index, refs — and layers on an
extended command set with a focus on readability, embeddability, and hackability.

## Where vstodo is simpler than Git

| Area | Git | vstodo |
|------|-----|--------|
| Index format | Binary, packed, versioned, checksummed | Line-oriented text: `<hex>\t<path>` |
| Object storage | Loose + packfiles + deltas + alternates | Loose files only |
| Ref storage | Loose + packed-refs + transactions | Loose text files only |
| Configuration | Multi-level config files with includes | Minimal `config` file |
| Hooks | Extensive hook ecosystem | None |
| Remotes | fetch/push/clone/remote tracking | None (bundle for transfer) |
| Submodules | Full submodule support | None |
| Reflog | Per-ref reflog for undo | None |

The index is the most visible simplification. Git's index is a binary format with
a header, entry count, extensions, and checksums. `vstodo`'s index is human
readable and can be edited with any text editor. The cost is linear-time parsing
and no built-in conflict state, which is acceptable for the project's scope.

## Where vstodo matches Git

| Area | Behavior |
|------|----------|
| Object envelope | `<type> <size>\0<content>` (identical) |
| Hashing | SHA-1 over serialized object |
| Compression | zlib deflate |
| Loose object path | `objects/<2-hex>/<38-hex>` |
| Blob/tree/commit formats | Byte-compatible with Git |
| Tree entry encoding | `<mode> <name>\0<20-byte raw id>` |
| Commit header | `tree`/`parent`/`author`/`committer`/blank/message |

Because `vstodo` reuses Git's object byte formats, a `vstodo` object store is a
valid Git object store for the core types. You can copy `.vstodo/objects` into a
Git repository as `.git/objects` and Git will read the blobs, trees, and commits.

## Where vstodo is "more advanced than Git"

This is the project's defining angle: a small codebase with an extended feature
surface area and design choices Git does not have.

### 1. Extended command set in a tiny codebase

`vstodo` ships core commands (`init`, `add`, `commit`, `status`, `log`, ...),
advanced commands (`blame`, `bisect`, `stash`, `cherry-pick`, `notes`, `gc`,
`worktree`, `bundle`), and plumbing (`hash-object`, `cat-file`) — all in roughly
a dozen internal packages plus one file per command. Git spreads equivalent
functionality across hundreds of files and dozens of built-ins written in C,
Perl, and shell.

### 2. Pluggable storage

The object store is an interface:

```go
type Store interface {
    Store(obj *object.Object) (sha1.Digest, error)
    Load(id sha1.Digest) (*object.Object, error)
    Has(id sha1.Digest) (bool, error)
}
```

The default `storage.LooseStore` writes loose zlib files. A packfile backend, a
database backend, an in-memory store for tests, or a remote store can be plugged
in by implementing this interface — no changes to commit/tree/index logic
required. Git's storage layer is tightly coupled to packfiles and loose objects.

### 3. Embeddable library

`vstodo` is designed to be imported as a library:

```go
app, err := app.Init("/tmp/repo")
// add files, commit, read history programmatically
```

See `examples/`. Git is fundamentally a CLI tool; embedding `libgit2` or
shelling out to `git` is the norm for programmatic use. `vstodo` is a Go
library first, CLI second.

### 4. Built-in demo

`vstodo demo` initializes a scratch repository and runs an end-to-end scenario
(add files, commit, show history) with no user setup. Git has no equivalent
"run a guided demo" command; users must manually create files and run commands.

### 5. Readable, small index and refs

The text-based index and loose refs make repository internals inspectable with
`cat`, `grep`, and standard Unix tools without plumbing commands. Git requires
`git ls-files`, `git read-tree`, `git show-ref`, etc., to inspect the same
state.

### 6. Standard library only

`vstodo` has zero external dependencies (`go.mod` lists only `go 1.26.3` and no
`require` directives). Git depends on zlib, OpenSSL/libcurl, expat, pcre2, and
more. A `vstodo` binary compiles with the Go toolchain alone.

### 7. Single working directory per repo

`vstodo` has no concept of a "separate git dir" or linked worktrees in the core;
each repo is one working tree with one `.vstodo` directory. This reduces
surprise around path handling, environment variables (`GIT_DIR`, `GIT_WORK_TREE`),
and the `common-dir` mechanism.

## Trade-offs

The simplicity and extended command set come with costs relative to mature Git:

- **No packfiles**: large histories store every object uncompressed-by-delta as
  individual loose files. Disk usage grows linearly with object count rather
  than benefitting from delta compression. Packfile support is the natural
  next storage backend (implement `object.Store`).
- **No reflog**: accidental branch resets or rebases cannot be undone through a
  reflog. Refs are overwritten in place.
- **No remotes/networking**: there is no `clone`, `fetch`, or `push`. `bundle`
  provides file-based transfer, but there is no transport protocol.
- **No hooks or configuration**: no pre-commit hooks, no `core.autocrlf`, no
  user-specific config.
- **SHA-1 only**: no SHA-256 or hybrid-hash transition support.

## Summary

`vstodo` keeps Git's content-addressed object model — the part that is
fundamentally correct — and strips away the accreted complexity around the
index, storage, refs, configuration, and transport. It then adds an extended
command set, a pluggable storage interface, an embeddable library, and a
built-in demo. The result is a system that is easier to read, modify, and embed
than Git, while remaining compatible with Git's object formats at the byte level.
