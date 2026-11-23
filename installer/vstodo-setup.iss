; Inno Setup Script for VSTodo
; Generates: vstodo-setup.exe

[Setup]
; Basic info
AppName=VSTodo
AppVersion=1.0.0
AppPublisher=SneppX
AppPublisherURL=https://github.com/SneppX/vstodo
SupportURL=https://github.com/SneppX/vstodo/issues
DefaultDirName={localappdata}\VSTodo
DisableDirPage=yes
DefaultGroupName=VSTodo
UninstallDisplayIcon={localappdata}\VSTodo\vstodo.exe
OutputDir=..
OutputExecutable=vstodo-setup.exe
Compression=lzma
SolidCompression=yes

; Windows settings
PrivilegesRequired=low
WindowsVersion=Win10
EnableAdminPage=no

; Tasks
[Tasks]
Name="desktopicon"; Description="Create a desktop shortcut"; Flags=checkboxchecked
Name="startmenu"; Description="Create a Start Menu shortcut"; Flags=checked

; Files
[Files]
Source: "..\vstodo.exe"; DestDir:{localappdata}\VSTodo; Flags=ignorearch
Source: "..\..\website\style.css"; DestDir:{localappdata}\VSTodo; Flags=ignorearch
; Don't copy npm/README.md or any npm related files

; [Icons]
; Name: "{group}\VSTodo"; Filename: "{localappdata}\VSTodo\vstodo.exe"
; Name: "{commondesktop}\VSTodo"; Filename: "{localappdata}\VSTodo\vstodo.exe"

; [Registry]
; Writes PATH entry to add vstodo to system PATH
[Registry]
Root: HKLM; Subkey: SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\VSTodo; ValueType: string; ValueName: DisplayIcon; ValueData: "{localappdata}\VSTodo\vstodo.exe"
Root: HKLM; Subkey: SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\VSTodo; ValueType: string; ValueName: DisplayName; ValueData: VSTodo
Root: HKLM; Subkey: SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\VSTodo; ValueType: string; ValueName: UninstallString; ValueData: "{uninstallexe}"
Root: HKCU; Subkey: SOFTWARE\Microsoft\Windows\CurrentVersion\Explorer\Shell Folders\Personal; ValueType: string; ValueName: PATH; ValueData: "{app}\vstodo.exe"

; [Run]
; Run after installation
[Run]
Filename: "{localappdata}\VSTodo\vstodo.exe"; Parameters: "--version"; Status messages