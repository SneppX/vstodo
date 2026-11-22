# VSTodo

**A new version control system, built from the ground up in Go.**

VSTodo is an independent version control system inspired by Git. Its goal is to build a powerful, flexible, and developer-friendly tool with its own implementation of version control fundamentals.

## Vision

The long-term vision is to build a complete version control ecosystem.

- Repository management
- Read-only object and reference integrity verification (`vstodo fsck`)
- File tracking and staging
- Commits and history
- Branching and merging
- Diff and change detection
- Object storage and packfiles
- Remote repositories
- Push, pull, fetch, and clone
- GitHub interoperability
- A dedicated web platform

## Built With

- Go
- Git-compatible object formats and smart-HTTP protocols for interoperability

## Adopt an existing Git repository

Run `vstodo adopt` from an existing Git working tree to add VSTodo alongside Git.
The command imports SHA-1 loose objects, Git v2/v3 packfiles, branches, tags,
remote-tracking refs, HEAD, remote URLs and fetch settings, branch upstream
settings, and local user identity. It preserves exact copies of `.git/config`
and `.git/index` under `.vstodo/migration/`. The only source-Git metadata
change is a local `.git/info/exclude` rule for `.vstodo/`, which keeps
VSTodo's private metadata out of ordinary Git status output. Existing exclude
rules are preserved. Git objects, refs, config, index, remotes, and working
files remain unchanged, and Git remains fully usable during the adoption
period. Use `vstodo git-status` to compare the checked-out Git branch with the matching VSTodo branch. If Git has new commits, `vstodo sync-git` imports loose and packed Git objects and advances VSTodo only when Git is a fast-forward of the VSTodo branch. The command uses Git's branch lock and VSTodo's compare-and-swap ref update; it does not change Git refs, either index, config, or working files. If VSTodo is ahead, use `vstodo export-git`. Diverged histories are rejected for manual review. VSTodo also imports branch upstream (`remote`/`merge`), branch
`pushRemote`, and remote `pushurl` settings; default pull and push use those
settings when present. Existing staged changes are preserved in the Git index backup but are
not yet represented as VSTodo staged changes; inspect `vstodo status` before
making VSTodo commits.

The synchronization commands reconcile committed branch history only. They do not import Git's staged or unstaged changes, and they preserve the VSTodo index and working tree so `vstodo status` can show differences against the newly advanced HEAD. The initial adoption flow supports standard `.git` directories and SHA-1
repositories. Linked worktrees, submodules, SHA-256 repositories, and object
alternates are rejected safely rather than partially imported. VSTodo's remote
transport currently supports HTTP(S); SSH remote settings are preserved but
cannot yet be used by VSTodo transport.

## Remote transport

VSTodo currently supports HTTP smart-transport reference discovery, fetch, clone,
fast-forward-only branch push, and fetch-plus-merge pull. For GitHub authentication, set
`VSTODO_GITHUB_TOKEN` or `GITHUB_TOKEN` in the environment. Pushes do not yet
support force updates, tags, or multiple refs in one operation.

## Project Status

VSTodo is under active development. Its local version-control workflows and
initial GitHub-compatible HTTP transport are implemented; broader protocol and
interoperability coverage is still in progress.

## Development Philosophy

- Build from the fundamentals.
- Prioritize correctness and reliability.
- Maintain clean, understandable code.
- Test features as they are implemented.
- Expand incrementally.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development guidelines and pull request instructions.

## License

VSTodo is licensed under the [MIT License](LICENSE).

---

**VSTodo — Version control, built differently.**


