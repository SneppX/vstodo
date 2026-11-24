# VSTodo

**An independent version control system, built from the ground up in Go.**

VSTodo is an independent version control system with its own implementation of version-control fundamentals and Git-compatible interoperability.

## Installation

Download a verified installer or portable archive from the [latest release](https://github.com/SneppX/vstodo/releases/latest).

| Platform | Available release formats |
| --- | --- |
| Windows x64 | EXE installer, MSI installer, ZIP archive |
| Linux x64 / ARM64 | DEB, RPM, APK, Arch package, TAR.GZ |
| macOS Intel / Apple Silicon | PKG installer, DMG, TAR.GZ |

See [Linux installation](linux-Install-README.md) and [macOS installation](macOS-Install-README.md) for platform-specific instructions.

Verify downloaded assets using the SHA-256 entries in `checksums.txt`. Packages are not yet code-signed or notarized.

## Core capabilities

- Repository initialization and Git repository adoption
- File staging, commits, status and history
- Branches, checkout, diff, show, merge and rebase
- Tags, reset, stash, cherry-pick, notes and blame
- Garbage collection, worktrees and repository bundles
- Git-compatible object formats and SHA-1 object hashing
- HTTP smart-transport reference discovery, clone, fetch, push and pull
- GitHub authentication using `VSTODO_GITHUB_TOKEN` or `GITHUB_TOKEN`
- A native VSTodo installer CLI for user-scoped install, upgrade, repair, verification and uninstall

## Adopt an existing Git repository

Run `vstodo adopt` from an existing Git working tree to add VSTodo alongside Git. Adoption imports supported Git objects, refs and configuration while preserving the original Git repository. See the command documentation before adopting repositories with unusual layouts.

## Build from source

Requires the Go version declared in `go.mod`.

```sh
go test ./...
go build -o vstodo .\ngo build -o vstodo-installer ./cmd/vstodo-installer
```

## Use the VSTodo installer

The standalone installer manages a user-scoped installation, verifies installed file hashes, upgrades or repairs binaries, updates the user PATH, and uninstalls only files recorded in its manifest.

```sh
vstodo-installer install --source /path/to/vstodo
vstodo-installer verify
vstodo-installer upgrade --source /path/to/new/vstodo
vstodo-installer uninstall
```

The installer is included in native packages and portable release archives.

## Project status

VSTodo is under active development. Its CLI implements a broad set of local version-control operations and an initial HTTP remote transport. Git protocol coverage, security hardening, interoperability testing and installer signing remain active work.

## License

MIT. See [LICENSE](LICENSE).
