# Install VSTodo on Windows

Download the installer from the [latest VSTodo release](https://github.com/SneppX/vstodo/releases/latest).

## Windows x64

Choose either installer:

- **EXE setup:** `vstodo-setup-<version>-windows-amd64.exe` — interactive setup.
- **MSI:** `vstodo-<version>-windows-amd64.msi` — Windows Installer package.

Both install `vstodo.exe` and `vstodo-installer.exe`, add VSTodo to the system PATH, and provide an uninstall entry.

Open a new terminal after installation:

```powershell
vstodo --version
vstodo-installer version
```

## Windows ARM64

Use the `windows_arm64.zip` portable archive. Extract it and add the extracted directory to your user PATH. Native EXE and MSI installers are currently x64 only.

## Verify downloads

Every release includes `checksums.txt`. Verify the downloaded asset before running it. Windows installers are currently unsigned, so SmartScreen may display an unknown-publisher warning.
