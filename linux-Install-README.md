# VSTodo Linux Installer

## Manual Installation (Recommended)

1. Download `vstodo-1.0.0-linux-amd64.tar.gz` from GitHub Releases
2. Extract the archive:
   ```bash
   tar -xzvf vstodo-1.0.0-linux-amd64.tar.gz
   ```
3. Copy the binary to `/usr/local/bin`:
   ```bash
   sudo cp vstodo /usr/local/bin/vstodo
   ```
4. Make it executable:
   ```bash
   sudo chmod +x /usr/local/bin/vstodo
   ```
5. Verify installation:
   ```bash
   vstodo --version
   # Output: vstodo version 1.0.0
   ```

## Alternative: DEB Package

If you want a proper .deb package:

1. Install dependencies:
   ```bash
   sudo apt-get install checkinstall
   ```
2. Create the package:
   ```bash
   checkinstall -D -y --pkgname=vstodo --pkgversion=1.0.0 --maintainer="SneppX" --default
   ```
3. Or manually create .deb using dpkg-deb

The `checkinstall` method creates a .deb that tracks the installation and can be removed with `apt-get remove vstodo`.

## Alternative: apt Repository

Add the VSTodo repository:
```bash
echo "deb [trusted=yes] https://package.sneppx.io/apt/ /" | sudo tee /etc/apt/sources.list.d/vstodo.list
sudo apt-get update
sudo apt-get install vstodo
```