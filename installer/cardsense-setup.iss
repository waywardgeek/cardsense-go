; CardSense Go - Windows Installer Script
; Uses Inno Setup: https://jrsoftware.org/isinfo.php

#define MyAppName "CardSense"
#define MyAppVersion "0.3.0"
#define MyAppPublisher "Bill Cox"
#define MyAppURL "https://github.com/waywardgeek/cardsense-go"
#define MyAppExeName "cardsense-gui.exe"

[Setup]
; NOTE: The value of AppId uniquely identifies this application.
; Do not use the same AppId value in installers for other applications.
AppId={{8F9E4C2D-1B3A-4E5F-9C7D-2A8B6E4F1C3D}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}/issues
AppUpdatesURL={#MyAppURL}/releases
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
AllowNoIcons=yes
LicenseFile=..\LICENSE
OutputDir=..\dist
OutputBaseFilename=cardsense-setup-{#MyAppVersion}
Compression=lzma
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=lowest
ArchitecturesInstallIn64BitMode=x64compatible

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked
Name: "quicklaunchicon"; Description: "{cm:CreateQuickLaunchIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked; OnlyBelowVersion: 6.1; Check: not IsAdminInstallMode

[Files]
; Main executable
Source: "..\cardsense-gui.exe"; DestDir: "{app}"; Flags: ignoreversion

; OpenCV DLLs (will be populated by GitHub Actions or manual build)
Source: "..\dist\windows-libs\opencv\*.dll"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs

; Tesseract
Source: "..\dist\windows-libs\tesseract\*"; DestDir: "{app}\tesseract"; Flags: ignoreversion recursesubdirs

; Hash files (26MB - for instant startup)
Source: "..\hashindex\data\phash_index.npz"; DestDir: "{app}\data"; Flags: ignoreversion
Source: "..\hashindex\data\phash_meta.json"; DestDir: "{app}\data"; Flags: ignoreversion

; Documentation
Source: "..\README.md"; DestDir: "{app}"; Flags: ignoreversion isreadme
Source: "..\LICENSE"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{group}\{cm:UninstallProgram,{#MyAppName}}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon
Name: "{userappdata}\Microsoft\Internet Explorer\Quick Launch\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: quicklaunchicon

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent

[Code]
function InitializeSetup(): Boolean;
var
  Version: String;
begin
  Result := True;
  
  // Check Windows version (require Windows 10+)
  if not IsWin64 then
  begin
    MsgBox('CardSense requires 64-bit Windows 10 or later.', mbError, MB_OK);
    Result := False;
    Exit;
  end;
  
  // Get Windows version
  Version := '';
  if GetWindowsVersion < $0A00 then
  begin
    MsgBox('CardSense requires Windows 10 or later.', mbError, MB_OK);
    Result := False;
  end;
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssPostInstall then
  begin
    // Create data directory if it doesn't exist
    if not DirExists(ExpandConstant('{app}\data')) then
      CreateDir(ExpandConstant('{app}\data'));
  end;
end;
