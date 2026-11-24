; VSTodo per-machine Windows installer.
; Built in CI after the versioned Windows binary is downloaded.

#ifndef AppVersion
  #define AppVersion "0.2.0"
#endif

[Setup]
AppId={{8E16A7E4-8B7D-4A2C-8E66-0F95EAC6E5A1}
AppName=VSTodo
AppVersion={#AppVersion}
AppPublisher=SneppX
AppPublisherURL=https://github.com/SneppX/vstodo
AppSupportURL=https://github.com/SneppX/vstodo/issues
DefaultDirName={autopf}\VSTodo
DefaultGroupName=VSTodo
UninstallDisplayIcon={app}\vstodo.exe
OutputDir=dist
OutputBaseFilename=vstodo-setup-{#AppVersion}-windows-amd64
ArchitecturesAllowed=x64
ArchitecturesInstallIn64BitMode=x64
PrivilegesRequired=admin
WizardStyle=modern
Compression=lzma2
SolidCompression=yes
CloseApplications=yes
RestartApplications=no
Uninstallable=yes

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Additional shortcuts:"; Flags: unchecked

[Files]
Source: "dist\vstodo.exe"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\VSTodo"; Filename: "{app}\vstodo.exe"; Parameters: "--help"; WorkingDir: "{userdocs}"
Name: "{autodesktop}\VSTodo"; Filename: "{app}\vstodo.exe"; Parameters: "--help"; Tasks: desktopicon

[Run]
Filename: "{app}\vstodo.exe"; Parameters: "--version"; Description: "Verify VSTodo installation"; Flags: postinstall nowait skipifsilent
