# Internals

This document is a deep dive into `vstodo`'s on-disk formats, object header,
storage layout, and addressing scheme. It assumes you have read `docs/ARCHITECTURE.md`.

## Object header format

Every vstodo object is serialized with a small ASCII header followed by a NUL
byte and the raw payload. The header has the form:

```
<type> <size>\0<content>
```

Where:

- `<type>` is one of `blob`, `tree`, `commit`, `tag` (ASCII).
- `<size>` is the decimal byte length of `<content>` (ASCII, no leading zeros
  except for zero itself).
- `\0` is a single NUL byte (0x00), separating header from payload.
- `<content>` is the object's raw bytes.

For example, a blob containing `hello\n` (6 bytes) serializes to:

```
blob 6\0hello\n
```

The complete byte sequence (showing the NUL explicitly) is:

```
62 6c 6f 62 20 36 00 68 65 6c 6c 6f 0a
   ^^^^^ ^^ ^  ^^^^^^^^^
   type  sp size NUL   content
```

This header is **not** stored separately; it is part of the hashed and compressed
byte stream.

## Hashing and addressing

The object ID (SHA-1 digest) is computed over the full serialized form including
the header:

```
id = SHA1("<type> <size>\0<content>")
```

In code:

```go
id := object.ComputeID(object.BlobType, data)
```

`ComputeID` writes `type`, a space, the decimal length, a NUL, then the content
into a SHA-1 hasher and returns the 20-byte digest. Because the header is part of
the hash, objects of different types with identical content get different IDs,
matching Git behavior.

`sha1.Digest` is a fixed `[20]byte` array. Use `Digest.Hex()` for the 40-character
lowercase hex representation used in refs, index, and CLI output.

## Compression and storage

After computing the ID, the serialized object is compressed with zlib (RFC 1950,
deflate) and written to a loose object file:

```
.vstodo/objects/<first 2 hex chars>/<remaining 38 hex chars>
```

For object `dae86e3...9f2`, the path is:

```
.vstodo/objects/da/e86e3...9f2
```

The file contains only the zlib-compressed bytes (no header wrapper). Files are
created read-only (`0444`) to emphasize immutability.

The store is append-only and content-addressed: storing the same object twice
writes the same path and returns the same digest. There are no packfiles, no
deltas, and no alternate object stores in the current implementation.

## Object type details

### Blob

A blob holds raw file content, with no encoding or transformation:

```
blob <len>\0<file bytes>
```

A file's blob ID is `SHA1("blob <len>\0<file bytes>")`. Empty files hash to the
SHA1 of `blob 0\0`.

### Tree

A tree maps a directory's contents. Its payload is a sequence of entries, each
encoded as:

```
<mode> <name>\0<20-byte raw SHA-1>
```

Fields:

- `<mode>` — ASCII file mode, e.g. `100644` (regular file), `100755` (executable),
  `040000` (directory), `120000` (symlink).
- `<name>` — ASCII basename (no path separators).
- `\0` — NUL separator.
- `<20-byte raw SHA-1>` — the binary digest of the referenced object (blob or
  subtree), NOT hex.

Entries are sorted by name when marshaled (Git convention). A tree's ID is:

```
SHA1("tree <len>\0<entry bytes...>")
```

Subdirectories are represented by nested tree objects referenced with mode
`040000`.

Example tree payload (displayed conceptually):

```
100644 foo.c\0<20 bytes>
040000 sub\0<20 bytes>
100755 run.sh\0<20 bytes>
```

### Commit

A commit records a snapshot plus history and metadata. Its payload is:

```
tree <40-hex>
parent <40-hex>       (zero or more lines)
author <name> <email> <unix-ts> <tz>
committer <name> <email> <unix-ts> <tz>

<message>
```

- `tree` — root tree digest (exactly one).
- `parent` — parent commit digest; absent for root commits, repeated for merge
  commits.
- `author` / `committer` — free-form identity lines.
- A blank line separates headers from the message.
- The message runs to the end of the payload.

A commit's ID is `SHA1("commit <len>\0<headers><blank><message>")`. Commit IDs
are the primary addresses users and refs work with.

### Tag

An annotated tag points at another object:

```
object <40-hex>
type <commit|tree|blob|tag>
tagger <name> <email> <unix-ts> <tz>

<tag message>
```

The tag's own ID is the SHA-1 of this serialized payload. Lightweight tags (a
ref pointing directly at a commit) are represented by a branch-style ref and do
not require a tag object.

## Loose object naming

Loose objects use two-level directory sharding to avoid putting tens of
thousands of files in one directory:

- Directory name: first 2 hex characters of the digest.
- File name: remaining 38 hex characters.

Full path: `.vstodo/objects/XX/YYYY...Y` (40 hex chars total). This matches Git's
loose-object layout exactly, so a `vstodo` object directory can be dropped into a
Git repository as `.git/objects` (ignoring packfiles).

## How a commit is created

1. Blobs are created for each file's content (`object.ComputeID(BlobType, data)`,
   then `store.Store`).
2. A tree object is built from path→blob mappings (`tree.Build`, then hashed and
   stored).
3. A commit object is built referencing that tree and its parent(s)
   (`commit.Build`, then hashed and stored).
4. The current branch ref is updated to the new commit digest.
5. HEAD is pointed at the branch (or set directly for detached HEAD).
6. The index is cleared/reset to match the new tree.

## The index format

`.vstodo/index` is line-oriented text:

```
<40-hex blob digest>\t<relative path>
```

One line per staged file. Blank lines and `#` comments are skipped. There is no
binary header, no checksum, no entry count, and no extension blocks. This makes
the index trivial to parse, regenerate, and hand-edit, at the cost of lookup
being O(n) over file contents rather than O(log n) over a packed index. For the
project's scope this is an intentional simplicity win (see `docs/COMPARISON.md`).

## The refs format

Refs are plain text files:

- `.vstodo/HEAD` — either `ref: refs/heads/main\n` (symbolic) or `<40-hex>\n`
  (detached).
- `.vstodo/refs/heads/<name>` — `<40-hex>\n`.
- `.vstodo/refs/tags/<name>` — `<40-hex>\n`.

Reading a ref follows `ref: ` pointers recursively. There is no packed-refs
file, no reflog, and no transactional ref updates.

## Walk / graph traversal

The `walk` package provides a `Walker` with a `Load` callback for traversing the
commit graph. The current `History` implementation walks first-parent history.
The package is deliberately minimal so that custom traversals (ancestry,
reachability, merge-base) can be implemented by consumers without touching the
core.

## Summary of key invariants

- Object ID = SHA-1 of `<type> <size>\0<content>` (header included).
- Loose object path = `.vstodo/objects/<hex[:2]>/<hex[2:]>`.
- Object content on disk = zlib-deflate of the serialized object.
- Objects are immutable; addresses are content-based and collision-resistant
  (within SHA-1's bounds).
- Index is `<hex>\t<path>` text, one line per staged path.
- Refs are text files holding either `ref: <target>` or a 40-hex digest.
