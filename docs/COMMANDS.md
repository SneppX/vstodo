# Command reference

This document describes every `vstodo` command. Commands are dispatched from
`main.go` to the matching `cmd_*` function. Most commands require a `vstodo`
repository in the current directory or an ancestor; `init` is the exception.

See `vstodo help` and `vstodo help <command>` for terse usage strings.

## Core commands

### init

```
vstodo init [path]
```

Initialize a new, empty `vstodo` repository at `path` (default `.`). Creates
`.vstodo/` with `objects/`, `refs/heads/`, `refs/tags/`, `HEAD` (pointing at
`refs/heads/main`), `config`, and `description`. Prints the repository location.

### add

```
vstodo add <file> [file...]
```

Read each file's content, store it as a blob object, and stage its path in the
index mapped to the blob digest. Multiple files may be given. The index is saved
after all files are processed.

### commit

```
vstodo commit -m <message>
```

Build a tree from the staged index entries, create a commit object referencing
that tree and the current HEAD as parent, store the commit, advance the current
branch ref to the new commit, reset the index, and print a summary line.
Requires `-m <message>`.

### status

```
vstodo status
```

Show the working tree status: the current branch, whether the index is clean or
has staged changes, and a list of staged files with their blob digests.

### log

```
vstodo log
```

Show first-parent commit history starting at HEAD. For each commit prints the
full SHA-1, author, date, and message.

### branch

```
vsteto branch [name]
vstodo branch -d <name>
```

List branches when no name is given. Create a branch pointing at the current
HEAD when a name is given. Delete a branch with `-d`.

### checkout

```
vstodo checkout <branch>
vstodo checkout <commit>
```

Switch to a branch (updating HEAD and working tree to the branch tip) or check
out a specific commit (detached HEAD). The working tree is rewritten to match
the target tree.

### diff

```
vstodo diff [commit]
```

Show changes between the index/working tree and an optional commit, or between
two commits if supported. Output lists changed paths and their before/after blob
digests.

### show

```
vstodo show <object>
```

Display the content or type of an object (commit, tree, blob, or tag). For
commits prints headers and message; for trees lists entries; for blobs prints
raw content.

### merge

```
vstodo merge <branch>
```

Merge the given branch into the current branch. Finds a merge base, applies
combined changes, and creates a merge commit when both sides diverged.

### tag

```
vstodo tag [name]
vstodo tag -d <name>
```

List tags with no arguments. Create a lightweight tag at HEAD when a name is
given. Delete a tag with `-d`.

### reset

```
vstodo reset [--hard] <commit>
```

Move HEAD (and the current branch) to `commit`. With `--hard`, also rewrite the
working tree and reset the index to match.

### rm

```
vstodo rm <file> [file...]
```

Remove files from the working tree and unstage them from the index.

### rebase

```
vstodo rebase <branch>
```

Reapply the current branch's commits on top of `<branch>`. Rewrites history by
creating new commits with the same trees and messages on the new base.

## Advanced commands

### blame

```
vstodo blame <file>
```

Show, for each line of `<file>`, the commit that last modified it and the
author. Output format: `<commit> <author> <line content>`.

### bisect

```
vstodo bisect start
vstodo bisect good <commit>
vstodo bisect bad <commit>
vstodo bisect reset
```

Binary-search the commit graph to find the change that introduced a bug. Mark
commits as good/bad; `vstodo` checks out candidates and narrows the range until
the first bad commit is identified.

### stash

```
vstodo stash
vstodo stash list
vstodo stash apply
vstodo stash pop
```

Stash away dirty working-tree changes so the tree can be reset to a clean state,
then reapply or discard the stash later.

### cherry-pick

```
vstodo cherry-pick <commit>
```

Apply the changes introduced by a single existing commit onto the current HEAD,
creating a new commit with the same message.

### notes

```
vstodo notes add <object> <message>
vstodo notes show <object>
```

Add or inspect free-form notes attached to arbitrary objects (commits, blobs,
etc.). Notes are stored as separate objects referenced by a notes ref.

### gc

```
vstodo gc
```

Garbage-collect unreachable objects and optimize the repository. With only loose
objects this is a no-op placeholder; it is the hook where packfile compaction
and object pruning would be implemented.

### worktree

```
vstodo worktree add <path> <commit>
vstodo worktree list
vstodo worktree remove <path>
```

Manage multiple working trees attached to the same repository, each checked out
at a different commit.

### bundle

```
vstodo bundle create <file> <commit-range>
vstodo bundle unbundle <file>
```

Package objects and refs into a single file for transfer by archive (email,
USB, etc.), and unbundle them into another repository.

## Plumbing commands

### hash-object

```
vstodo hash-object <file>
```

Compute and print the SHA-1 object ID for `<file>`'s content (as a blob) without
storing it. Useful for scripting and verifying content addressing.

### cat-file

```
vstodo cat-file -t <object>
vstodo cat-file -p <object>
```

Print the type (`-t`) or pretty content (`-p`) of a repository object.

## Plugin/demo command

### demo

```
vstodo demo
```

Run a built-in end-to-end demonstration: initialize a scratch repository, add
files, make commits, and print the resulting history and object graph. Intended
for newcomers to see `vstodo` in action without creating real files.
