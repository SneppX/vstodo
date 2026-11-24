; VSTodo per-machine Windows installer.
; Built in CI after the versioned Windows binaries are downloaded.

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
OutputDir=..\dist
OutputBaseFilename=vstodo-setup-{#AppVersion}-windows-amd64
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64
PrivilegesRequired=admin
WizardStyle=modern
Compression=lzma2
SolidCompression=yes
CloseApplications=yes
RestartApplications=no
Uninstallable=yes
ChangesEnvironment=yes

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Additional shortcuts:"; Flags: unchecked

[Files]
Source: "..\dist\vstodo.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\dist\vstodo-installer.exe"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\VSTodo"; Filename: "{app}\vstodo.exe"; Parameters: "--help"; WorkingDir: "{app}"
Name: "{autodesktop}\VSTodo"; Filename: "{app}\vstodo.exe"; Parameters: "--help"; Tasks: desktopicon

[Registry]
Root: HKLM; Subkey: "SYSTEM\CurrentControlSet\Control\Session Manager\Environment"; ValueType: expandsz; ValueName: "Path"; ValueData: "{app};{olddata}"; Flags: preservestringtype; Check: NeedsAddPath(ExpandConstant('{app}'))

[Run]
Filename: "{app}\vstodo.exe"; Parameters: "--version"; Description: "Verify VSTodo installation"; Flags: postinstall nowait skipifsilent

[Code]
function NormalizePath(const Value: string): string;
begin
  Result := Trim(Value);
  while (Length(Result) > 0) and ((Result[Length(Result)] = '\') or (Result[Length(Result)] = '/')) do
    Delete(Result, Length(Result), 1);
end;

function NeedsAddPath(Param: string): Boolean;
var
  OrigPath, Part: string;
  P: Integer;
begin
  Result := True;
  if RegQueryStringValue(HKLM, 'SYSTEM\CurrentControlSet\Control\Session Manager\Environment', 'Path', OrigPath) then
  begin
    while OrigPath <> '' do
    begin
      P := Pos(';', OrigPath);
      if P = 0 then
      begin
        Part := OrigPath;
        OrigPath := '';
      end
      else
      begin
        Part := Copy(OrigPath, 1, P - 1);
        Delete(OrigPath, 1, P);
      end;
      if CompareText(NormalizePath(Part), NormalizePath(Param)) = 0 then
      begin
        Result := False;
        Exit;
      end;
    end;
  end;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  OrigPath, Part, NewPath: string;
  P: Integer;
begin
  if CurUninstallStep = usUninstall then
  begin
    if RegQueryStringValue(HKLM, 'SYSTEM\CurrentControlSet\Control\Session Manager\Environment', 'Path', OrigPath) then
    begin
      NewPath := '';
      while OrigPath <> '' do
      begin
        P := Pos(';', OrigPath);
        if P = 0 then
        begin
          Part := OrigPath;
          OrigPath := '';
        end
        else
        begin
          Part := Copy(OrigPath, 1, P - 1);
          Delete(OrigPath, 1, P);
        end;
        if CompareText(NormalizePath(Part), NormalizePath(ExpandConstant('{app}'))) <> 0 then
        begin
          if NewPath <> '' then
            NewPath := NewPath + ';';
          NewPath := NewPath + Part;
        end;
      end;
      RegWriteExpandStringValue(HKLM, 'SYSTEM\CurrentControlSet\Control\Session Manager\Environment', 'Path', NewPath);
    end;
  end;
end;
