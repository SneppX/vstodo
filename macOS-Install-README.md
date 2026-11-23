# VSTodo macOS Installer

## Manual Installation (Recommended)

1. Download `vstodo-1.0.0-darwin-arm64.tar.gz` from GitHub Releases
2. Extract the archive:
   ```bash
   tar -xzvf vstodo-1.0.0-darwin-arm64.tar.gz
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

## Alternative: Homebrew Formula

If you have Homebrew, you can tap the VSTodo repository:
```bash
brew tap sneppx/brews
brew install vstodo
```

Or manually install:
```bash
brew install ./vstodo.rb
```

The Homebrew formula would copy the binary to `/usr/local/bin` and create the PATH entry automatically.