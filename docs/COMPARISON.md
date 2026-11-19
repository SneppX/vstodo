# Comparison: VSTodo and Git

VSTodo is an independently implemented version-control system written in Go. It does not copy or reverse-engineer Git's source code. It deliberately uses Git-compatible object serialization and a subset of Git smart-HTTP so repositories can interoperate where the supported formats and commands overlap.

## Compatibility surface

- **Objects:** blobs, trees, commits, and tags use Git-compatible object headers, SHA-1 IDs, and payload encodings.
- **Packfiles:** VSTodo reads and writes Git packfiles for remote transfer, including the supported delta forms. Received objects are expanded into the local loose-object store.
- **Remote protocol:** HTTP smart-transport supports reference discovery, clone, fetch, pull, and fast-forward-only branch push.
- **Not full Git compatibility:** VSTodo does not yet implement every Git command, transport, extension, object format, or repository metadata convention.

## Design comparison

| Area | Git | VSTodo today |
|------|-----|--------------|
| Implementation | Mature C and scripting codebase | Independent Go implementation; standard library only |
| Index | Binary, versioned, checksummed index | Human-readable line-oriented index |
| Local object storage | Loose objects, packfiles, alternates | Loose objects; packfiles are used for transport, not persistent local storage |
| Object hashing | SHA-1; SHA-256 repositories supported | SHA-1 only |
| Refs | Loose and packed refs, transactions, reflogs | Loose text refs; no reflog or packed-refs |
| Branch operations | Extensive branch and worktree semantics | Branches, checkout, merge, rebase, and basic linked worktrees |
| Remote discovery | Local, file, SSH, and HTTP transports | HTTP/HTTPS smart-HTTP only |
| Fetch and clone | Full protocol and feature set | HTTP clone and fetch; fetched objects are stored loosely |
| Pull | Fetch plus merge or rebase strategies | Fetch plus fast-forward or conservative three-way merge; clean working tree required |
| Push | Multiple refs, tags, force updates, options, hooks | One branch per push; fast-forward only; no tag or force push |
| Hooks | Extensive client and server hooks | Not implemented |
| Configuration | System, global, local, worktree, and command scopes | Minimal repository-local configuration |
| Submodules | Supported | Not implemented |

## Remote workflow currently supported

```sh
vstodo remote add origin https://github.com/owner/repository.git
vstodo ls-remote origin
vstodo clone https://github.com/owner/repository.git checkout
vstodo fetch origin
vstodo pull origin main
vstodo push origin main
```

For GitHub authentication, provide `VSTODO_GITHUB_TOKEN` or `GITHUB_TOKEN` through the environment. Remote URLs must not contain embedded credentials, query strings, or fragments.

Push is intentionally conservative: it updates one branch, requires the advertised remote tip to be present locally and be an ancestor of the local tip, and relies on the server's receive-pack status before updating the local remote-tracking ref. Force pushes, tag pushes, multi-ref pushes, SSH transport, partial clones, shallow clones, and Git protocol v2 are not implemented.

## Storage and repository trade-offs

The local object store remains append-only loose storage under `.vstodo/objects`. The packfile codec is used to exchange objects with HTTP remotes; fetch resolves incoming pack objects and writes them to the loose store, while push builds a pack from the objects reachable from the selected branch. VSTodo does not yet retain packfiles as a persistent local storage backend, use alternates, or provide Git's repack/maintenance behavior.

The index and refs favor inspectability over Git's mature binary and transactional formats. VSTodo has no reflog, hooks, submodules, SHA-256 repository format, or complete Git configuration model. Its linked-worktree command is intentionally smaller than Git's worktree administration and locking semantics.

## Summary

VSTodo is an independent VCS with a compatible core object model and an initial Git smart-HTTP interoperability layer. It is not a drop-in replacement for every Git workflow yet. The current goal is to make the local repository operations and the supported HTTP clone/fetch/pull/push path dependable before expanding protocol and compatibility coverage.
