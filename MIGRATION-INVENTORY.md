# VSTodo Go-to-C Migration Inventory

## Phase A: Repository Audit

This inventory maps every Go component in the VSTodo codebase to its intended C replacement.

### Legend
- **Go File** — Original Go implementation
- **C Replacement** — Intended C implementation
- **Status** — `planned`, `in-progress`, `completed`, `blocked`
- **Notes** — Migration considerations

---

## 1. Core Data Types

| Go File | C Replacement | Status | Notes |
|---------|--------------|--------|-------|
| `internal/sha1/sha1.go` | `c/sha1.c` — SHA-1 digest type, hasher, FromHex/FromBytes | planned | Pure C implementation of SHA-1 (20-byte digest) |
| `internal/object/object.go` | `c/object.h` — Object type, Object struct, Type constants | planned | Blob/Tree/Commit/Tag types with content-addressed ID |
| `internal/object/object.go` — ComputeID | `c/object.c` — ComputeID function | planned | SHA-1 header-prefixed: `"<type> <size>\0<content>"` |
| `internal/object/object.go` — Marshal/Unmarshal | `c/object.c` — Marshal/Unmarshal functions | planned | Wire format: header + NUL + payload |
| `internal/object/tree.go` | `c/tree.h` — TreeEntry, Tree struct, Parse/Marshal | planned | Entry format: `"<mode> <name>\0<20-byte hash>"` |
| `internal/object/commit.go` | `c/commit.h` — Commit struct, Parse/Marshal | planned | Git-compatible header format: tree/parent/author/committer/message |
| `internal/object/tag.go` | `c/tag.h` — Tag struct, Parse/Marshal | planned | Annotated tag with target object and type |

---

## 2. Repository Management

| Go File | C Replacement | Status | Notes |
|---------|--------------|--------|-------|
| `internal/repo/repo.go` | `c/repo.h` — Repository struct, Find/Init functions | planned | Repository discovery walking up directory tree; Init creates .vstodo layout |
| `internal/repo/repo.go` — DefaultDir/EnvOverride | `c/repo.c` — Default dir constants | planned | `.vstodo` dir, `VSTODO_DIR` env var override |

---

## 3. Reference Management

| Go File | C Replacement | Status | Notes |
|---------|--------------|--------|-------|
| `internal/ref/ref.go` | `c/ref.h` — RefStore struct, ReadRef/WriteRef/UpdateRef | planned | Git-compatible ref storage under `refs/heads`, `refs/tags`, `HEAD` |
| `internal/ref/ref.go` — validateRefName | `c/ref.c` — Ref name validation | planned | Rejects traversal, invalid chars, `@{`, control chars |
| `internal/ref/ref.go` — ListRefs/ListAllRefs | `c/ref.c` — ListRefs functions | planned | Enumerate all refs under refs/heads and refs/tags |
| `internal/ref/ref.go` — CurrentBranch | `c/ref.c` — CurrentBranch function | planned | Resolve HEAD to short branch name |

---

## 4. Object Storage

| Go File | C Replacement | Status | Notes |
|---------|--------------|--------|-------|
| `internal/storage/storage.go` | `c/storage.h` — LooseStore struct, Store/Load/Has | planned | Loose object store: `.vstodo/objects/<2>/<38>` |
| `internal/storage/storage.go` — storeCompressed | `c/storage.c` — Atomic object storage | planned | Same-directory temp file + atomic hard link |
| `internal/storage/storage.go` — Load/Decompress | `c/storage.c` — Load/Decompress functions | planned | Zlib decompress + object header parsing |
| `internal/storage/storage.go` — Objects/ObjectsDir | `c/storage.c` — Enumeration functions | planned | List loose objects, compute object paths |
| `internal/storage/storage.go` — StoreRaw/WriteRaw | `c/storage.c` — Raw store functions | planned | Validate then atomically store compressed bytes |

---

## 5. Index (Staging Area)

| Go File | C Replacement | Status | Notes |
|---------|--------------|--------|-------|
| `internal/index/index.go` | `c/index.h` — Index struct, Add/Mode/Remove/Get | planned | Git-compatible DIRC format v2 |
| `internal/index/index.go` — Load/Save | `c/index.c` — Index load/save binary/legacy text | planned | DIRC signature, version, entries, CRC32 checksum |
| `internal/index/index.go` — buildTreeFromIndex | `c/index.c` — buildTreeFromIndex function | planned | Recursively build tree from index entries |
| `internal/index/index.go` — treeToMap | `c/index.c` — treeToMap function | planned | Flatten tree to path->blob-id mapping |

---

## 6. SHA-1 Hashing

| Go File | C Replacement | Status | Notes |
|---------|--------------|--------|-------|
| `internal/sha1/sha1.go` — All functions | `c/sha1.c` — Full SHA-1 implementation | planned | NewHasher/Write/SumDigest/FromHex/FromBytes/Hex/Prefix |
| `internal/sha1/sha1_test.go` — TestSHA1Size/Zero/FromHex/Hasher | `c/sha1_test.c` — SHA-1 test suite | planned | Equivalent C unit tests |

---

## 7. Commit Building/Parsing

| Go File | C Replacement | Status | Notes |
|---------|--------------|--------|-------|
| `internal/commit/commit.go` — All functions | `c/commit.c` — Full commit Parse/Marshal/Build | planned | Git header format, identity parsing, author/committer timestamps |
| `internal/commit/commit.go` — parseIdentity/formatIdentity | `c/commit.c` — Identity parsing functions | planned | "Name <email> timestamp tz" format |

---

## 8. Tree Building/Parsing

| Go File | C Replacement | Status | Notes |
|---------|--------------|--------|-------|
| `internal/tree/tree.go` — All functions | `c/tree.c` — Full tree Parse/Marshal/Build functions | planned | Sorted entries by name, git convention |

---

## 9. App Wiring

| Go File | C Replacement | Status | Notes |
|---------|--------------|--------|-------|
| `internal/app/app.go` | `c/app.h` — App struct, New/Init functions | planned | Wires Repository, Store (Loose+Pack), Index, RefStore together |
| `internal/app/app.go` — SaveIndex | `c/app.c` — SaveIndex function | planned | Persist index + write snapshot marker |

---

## 10. Config

| Go File | C Replacement | Status | Notes |
|---------|--------------|--------|-------|
| `internal/config/config.go` | `c/config.h` — Config struct, Load/Save/Get/Set | planned | INI-style config with sections, key-value pairs |

---

## 11. Locking

| Go File | C Replacement | Status | Notes |
|---------|--------------|--------|-------|
| `internal/lockfile/lockfile.go` | `c/lockfile.h` — Lock struct, Acquire/Commit/Close | planned | Git-style exclusive lock with .lock sibling file |

---

## 12. Repository Integrity (fsck)

| Go File | C Replacement | Status | Notes |
|---------|--------------|--------|-------|
| `internal/fsck/fsck.go` | `c/fsck.h` — Check/Report struct | planned | Verify all objects, references, HEAD integrity |
| `internal/fsck/fsck.go` — checkTree/checkCommit/checkTag | `c/fsck.c` — Tree/commit/tag validation | planned | Recursive reference validation |

---

## 13. Garbage Collection

| Go File | C Replacement | Status | Notes |
|---------|--------------|--------|-------|
| `internal/gc/gc.go` | `c/gc.h` — GC/GCWithOptions functions | planned | Mark reachable from HEAD/branches/tags/stash; prune unreachable loose objects |
| `internal/gc/gc.go` — markReachable/mark | `c/gc.c` — Reachability marking | planned | DFS traversal of commit tree, tree entries, tags |

---

## 14. Git Adoption (Migrate)

| Go File | C Replacement | Status | Notes |
|---------|--------------|--------|-------|
| `internal/migrate/git.go` | `c/migrate.h` — Adopt/importLooseObjects/importPackfiles | planned | Import existing Git repo into VSTodo without modifying .git |
| `internal/migrate/git.go` — importLooseObjects | `c/migrate.c` — Loose object import | planned | Read Git objects, store via VSTodo storage layer |
| `internal/migrate/git.go` — importPackfiles | `c/migrate.c` — Pack file import | planned | Reverse-engineered git pack format reader |
| `internal/migrate/git.go` — importReferences | `c/migrate.c` — Reference import | planned | Packed-refs and loose refs import |
| `internal/migrate/git.go` — importConfig | `c/migrate.c` — Git config import | planned | User name/email, remotes, branch config |
| `internal/migrate/git.go` — preserveMetadata | `c/migrate.c` — Metadata preservation | planned | Backup Git config and index before conversion |

---

## 15. Packfile

| Go File | C Replacement | Status | Notes |
|---------|--------------|--------|-------|
| `internal/packfile/packfile.go` | `c/packfile.h` — Pack/PackIndex structures, ReadPack/WritePack | planned | Reverse-engineered git pack format v2/v3 |
| `internal/packfile/packfile.go` — ReadPack | `c/packfile.c` — Pack file parser | planned | Header, objects, delta resolution, checksum verification |
| `internal/packfile/packfile.go` — WriteIndex | `c/packfile.c` — Index writer | planned | Pack index v2 format with fanout, IDs, CRCs, offsets |
| `internal/packfile/packfile.go` — computeDelta/applyDelta | `c/packfile.c` — Delta compression | planned | OFS_DELTA and REF_DELTA encoding/decoding |

---

## 16. CLI Commands (40+ files)

Each `cmd_*.go` file maps to a C CLI implementation. Key examples:

| Go File | C Replacement | Status | Notes |
|---------|--------------|--------|-------|
| `cmd_init.go` | `cmds/cmd_init.c` — Initialize new repository | planned | `app.Init(root)` equivalent |
| `cmd_add.go` | `cmds/cmd_add.c` — Stage file contents | planned | Read file, HashBlob, Store, Index.Add, SaveIndex |
| `cmd_commit.go` | `cmds/cmd_commit.c` — Record staged changes | planned | buildTreeFromIndex, commit.Build, Store, UpdateRef |
| `cmd_clone.go` | `cmds/cmd_clone.c` — Clone repository | planned | Copy repo, import objects/refs |
| `cmd_push.go` | `cmds/cmd_push.c` — Push to remote | planned | Native transport protocol |
| `cmd_pull.go` | `cmds/cmd_pull.c` — Pull from remote | planned | Fetch + merge |
| `cmd_branch.go` | `cmds/cmd_branch.c` — Branch operations | planned | List, create, delete branches |
| `cmd_checkout.go` | `cmds/cmd_checkout.c` — Checkout working tree | planned | Read tree, restore working files |
| `cmd_merge.go` | `cmds/cmd_merge.c` — Merge branches | planned | Three-way merge engine |
| `cmd_rebase.go` | `cmds/cmd_rebase.c` — History rewriting | planned | Interactive/rebase onto |
| `cmd_blame.go` | `cmds/cmd_blame.c` — Blame attribution | planned | Annotate each line with last commit |
| `cmd_gc.go` | `cmds/cmd_gc.c` — Garbage collection | planned | GC.WithOptions dry-run/prune |
| `cmd_fsck.go` | `cmds/cmd_fsck.c` — Repository integrity check | planned | fsck.Check equivalent |
| `cmd_tag.go` | `cmds/cmd_tag.c` — Tag operations | planned | Create/list/delete annotated tags |
| `cmd_remote.go` | `cmds/cmd_remote.c` — Remote management | planned | Add/remove/list remotes |
| `cmd_fetch.go` | `cmds/cmd_fetch.c` — Fetch from remote | planned | Download objects/refs |
| `cmd_export_git.go` | `cmds/cmd_export_git.c` — Git export | planned | Git compatibility import/export |
| `cmd_log.go` | `cmds/cmd_log.c` — Commit history | planned | Traverse commit graph |
| `cmd_diff.go` | `cmds/cmd_diff.c` — Diff engine | planned | Working tree vs index/commit diff |
| `cmd_show.go` | `cmds/cmd_show.c` — Show object content | planned | cat-file equivalent |
| `cmd_hashobject.go` | `cmds/cmd_hashobject.c` — Compute SHA-1 hash | planned | hash-object equivalent |
| `cmd_init.go` | `cmds/cmd_init.c` — Initialize repo | planned | |
| `cmd_config.go` | `cmds/cmd_config.c` — Config get/set | planned | |
| `cmd_status.go` | `cmds/cmd_status.c` — Working tree status | planned | |
| `cmd_ls_remote.go` | `cmds/cmd_ls_remote.c` — List remote refs | planned | |
| `cmd_worktree.go` | `cmds/cmd_worktree.c` — Worktree management | planned | Multiple working directories |
| `cmd_bundle.go` | `cmds/cmd_bundle.c` — Bundle creation | planned | |
| `cmd_reset.go` | `cmds/cmd_reset.c` — Reset HEAD | planned | |
| `cmd_rm.go` | `cmds/cmd_rm.c` — Remove staged files | planned | |
| `cmd_rebase.go` | `cmds/cmd_rebase.c` — Rehistory | planned | |
| `cmd_cherrypick.go` | `cmds/cmd_cherrypick.c` — Cherry-pick | planned | |
| `cmd_stash.go` | `cmds/cmd_stash.c` — Stash changes | planned | |
| `cmd_notes.go` | `cmds/cmd_notes.c` — Notes management | planned | |
| `cmd_demo.go` | `cmds/cmd_demo.c` — Demo command | planned | |

---

## 17. Tests

| Go File | C Replacement | Status | Notes |
|---------|--------------|--------|-------|
| `sha1_test.go` | `test_sha1.c` — SHA-1 test suite | planned | TestSHA1Size/Zero/FromHex/Hasher equivalents |
| `storage_test.go` | `test_storage.c` — Storage layer tests | planned |
| `fsck_test.go` | `test_fsck.c` — fsck test suite | planned |
| `gc_test.go` | `test_gc.c` — GC test suite | planned |
| `index_test.go` | `test_index.c` — Index test suite | planned |
| `object_test.go` | `test_object.c` — Object test suite | planned |
| `walkfiles_test.go` | `test_walkfiles.c` — Walkfiles test | planned |
| `repo_commit_tree_test.go` | `test_repo_tree.c` — Repo/commit/tree tests | planned |
| `index_ref_test.go` | `test_ref.c` — Index/ref tests | planned |
| `cmd_*_test.go` (each) | `cmds/cmd_*_test.c` — Per-command tests | planned |
| `benchmark/` | `bench/` — C benchmarks | planned |

---

## 18. Build System

| Go File | C Replacement | Status | Notes |
|---------|--------------|--------|-------|
| `go.mod` | `CMakeLists.txt` — Cross-platform build system | planned | Support Windows/Linux/macOS, GCC/Clang/MSVC |
| Various `.go` files | C source files in `src/` | planned | All Go logic translated to C |
| `.github/workflows/` | CI workflows for C compilation | planned | CMake build, test, sanitizer configurations |

---

## 19. Distribution (Pre-Migration)

| Component | Status |
|-----------|--------|
| `npm/` — Deleted | Already removed from repo |
| `website/` — GitHub Pages | Already deployed at `https://SneppX.github.io/vstodo/` |
| `installer/` — Inno Setup + READMEs | Already created for v0.1.2 |
| `vstodo update` command | Already added to binary |
| GitHub Release `v0.1.2` | Already pushed with 7 assets |

---

## 20. Unexplained / Requiring Investigation

The following components were referenced but may need closer inspection:

- `internal/walk/` — Walkfiles utility (purpose under investigation)
- `internal/migrate/export.go` — Git export functionality
- `internal/packfile/security tests` — Packfile security validation
- `.vstodo/` — Default repository metadata structure
- Benchmark files in `benchmark/`
- Example files in `examples/`

---

## Migration Priority Order

1. **Core data types** (sha1, object, tree, commit) — Foundation for everything else
2. **Repository management** (repo, ref, storage) — Init, refs, object storage
3. **Index** — Staging area, essential for add/commit workflow
4. **CLI commands** (starting with simplest: init, add, commit, status)
5. **Integrity tools** (fsck, gc) — Verify migration correctness
6. **Git adoption** (migrate) — Import existing repositories
7. **Packfile** — Large-object support
8. **Remaining CLI commands** — Full feature parity
9. **Build system** (CMake) — Replace go build
10. **Tests** — Full C test suite

---

## Strict Migration Rules (Per AGENTS.md #8)

1. ✅ Do not rewrite project blindly — audit first (completed)
2. ✅ Do not delete Go code before C replacement verified
3. ✅ Do not remove features to make migration easier
4. ✅ Do not introduce unnecessary third-party dependencies
5. ✅ Do not use Go as permanent backend or runtime
6. ✅ Do not use C as wrapper around existing Go code
7. ✅ Do not fabricate test results or build success
8. ✅ Do not leave unfinished work undocumented
9. ✅ Do not break existing repository data
10. ✅ Do not introduce insecure memory or filesystem ops
11. ✅ Do not make unsupported cross-platform claims
12. ✅ Do not sacrifice correctness for superficial performance

---

**Inventory completed**. Next: Begin Phase B — C Architecture Design, then Phase C — incremental migration of each component.