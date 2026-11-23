# vstodo (npm wrapper)

This package provides an npm-distributed wrapper for the **vstodo** Go CLI.

## How it works

- On install, `install.js` downloads the matching GitHub Release binary for your OS/arch.
- The `vstodo` bin launcher runs the downloaded binary from `vendor/`.
- Release asset names follow the GoReleaser format configured in this repo.

## Install (after release)

```bash
npm install -g vstodo
vstodo --help
```

## Publishing plan

`vstodo` is a Go project; npm publishing is a **wrapper distribution channel**, not a native JS implementation.

To publish:

1. Create a GitHub release tag `vX.Y.Z` with GoReleaser artifacts (see `.github/workflows/release.yml`).
2. Set `npm/package.json` version to `X.Y.Z` (matching the tag without `v`).
3. Authenticate to npm (`npm login` or `NPM_TOKEN` in CI).
4. Publish from `npm/`:

```bash
cd npm
npm publish --access public
```

## Required CI secret for automated npm publish

If you add an npm publish workflow, configure repository secret `NPM_TOKEN` with an npm automation/publish token.

## Notes

- The npm package name `vstodo` must be available on npm; if taken, use a scoped name like `@sneppx/vstodo`.
- Version `0.0.0` skips binary download on install; released versions download binaries.
- Supports Linux/macOS/Windows on amd64/arm64, matching GoReleaser targets.
