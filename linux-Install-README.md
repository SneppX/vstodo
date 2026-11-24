# Install VSTodo on Linux

Download native packages or standalone archives from the [latest VSTodo release](https://github.com/SneppX/vstodo/releases/latest).

## Debian and Ubuntu

Download the matching `.deb` package, then run:

```sh
sudo apt install ./vstodo_*_linux_*.deb
vstodo --version
```

Use the `arm64` package on supported ARM64 systems.

## Fedora, RHEL and compatible distributions

```sh
sudo dnf install ./vstodo_*_linux_*.rpm
vstodo --version
```

## Alpine Linux

```sh
sudo apk add --allow-untrusted ./vstodo_*_linux_*.apk
vstodo --version
```

## Arch Linux

Install the generated Arch package with:

```sh
sudo pacman -U ./vstodo_*_linux_*.pkg.tar.zst
vstodo --version
```

## Portable installation

Download the matching Linux `.tar.gz` archive, extract it, and place the binary on your PATH:

```sh
tar -xzf vstodo_*_linux_*.tar.gz
sudo install -m 0755 vstodo /usr/local/bin/vstodo
sudo install -m 0755 vstodo-installer /usr/local/bin/vstodo-installer
vstodo --version
```

## Package verification

Each release includes `checksums.txt`. Verify downloaded assets before installation:

```sh
sha256sum --check checksums.txt
```

Packages are currently unsigned. Verify release checksums and publisher provenance before installing.
