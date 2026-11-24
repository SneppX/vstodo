# Install VSTodo on macOS

Download the installer for your CPU architecture from the [latest VSTodo release](https://github.com/SneppX/vstodo/releases/latest).

- Apple Silicon: asset ending in `macos-arm64.pkg` or `macos-arm64.dmg`.
- Intel: asset ending in `macos-amd64.pkg` or `macos-amd64.dmg`.

## PKG installer

Open the matching `.pkg` and follow the Installer prompts. The package installs `vstodo` and `vstodo-installer` into `/usr/local/bin`.

Verify:

```sh
vstodo --version
```

## DMG installer

Open the matching `.dmg`, then open the included PKG installer.

## Portable installation

A standalone `.tar.gz` archive is also available:

```sh
# Apple Silicon
tar -xzf vstodo_*_darwin_arm64.tar.gz

# Intel
tar -xzf vstodo_*_darwin_amd64.tar.gz

sudo install -m 0755 vstodo /usr/local/bin/vstodo
sudo install -m 0755 vstodo-installer /usr/local/bin/vstodo-installer
vstodo --version
```

## Security note

Packages are currently unsigned and are not notarized by Apple. macOS may warn before opening them. Do not bypass security prompts unless you have verified the release source and SHA-256 checksum.
