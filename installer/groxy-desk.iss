; ==============================================================================
; Groxy Desk v1.2.0 — Branded Native Windows Inno Setup 6 Installer Script
; Format Invariant (AGENTS.md): Inno Setup .exe only (Groxy-Desk-Setup-1.2.0.exe), NEVER .msi
; ==============================================================================

#ifndef MyAppVersion
  #define MyAppVersion "1.2.0"
#endif

#ifndef GrabBinSource
  #define GrabBinSource "C:\Development\GRAB\dist\grab-windows-amd64.exe"
#endif

#ifndef GrabmanBinSource
  #define GrabmanBinSource "C:\Development\grabman\dist\grabman-windows-amd64.exe"
#endif

#ifndef DeskBuildDir
  #define DeskBuildDir "..\dist"
#endif

#define MyAppName "Groxy Desk"
#define MyAppPublisher "Urbanspress — Post-1.0 GRAB Ecosystem"
#define MyAppURL "https://github.com/urbanspress/groxy-desk.stable"
#define MyAppExeName "groxy-desk.exe"
#define MyDaemonExeName "groxy-deskd.exe"

[Setup]
AppId={{7B6E3A8D-9C41-4E50-A612-8F03D4C91E06}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppVerName={#MyAppName} v{#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}
DefaultDirName={localappdata}\{#MyAppName}
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
OutputDir={#DeskBuildDir}
OutputBaseFilename=Groxy-Desk-Setup-{#MyAppVersion}
Compression=lzma2/fast
SolidCompression=yes
WizardStyle=modern
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
UninstallDisplayName={#MyAppName} v{#MyAppVersion}
UninstallDisplayIcon={app}\{#MyAppExeName}

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
; Native Windows WebView2 Desktop Shell & Loopback Host Daemon (127.0.0.1:9490, CGO_ENABLED=0)
Source: "{#DeskBuildDir}\groxy-desk-windows-amd64.exe"; DestDir: "{app}"; DestName: "{#MyAppExeName}"; Flags: ignoreversion
Source: "{#DeskBuildDir}\groxy-deskd-windows-amd64.exe"; DestDir: "{app}"; DestName: "{#MyDaemonExeName}"; Flags: ignoreversion

; Bundled Side-by-Side Frozen Upstream Ecosystem CLIs (GRAB v1.1.0 & grabman v1.0.0)
Source: "{#GrabBinSource}"; DestDir: "{app}\bin"; DestName: "grab.exe"; Flags: ignoreversion
Source: "{#GrabmanBinSource}"; DestDir: "{app}\bin"; DestName: "grabman.exe"; Flags: ignoreversion

; Pure Grex JS v1.0.0 Runtime, 20-Element Schema & Zero-<div> Workbench Assets
Source: "..\public\grex\*"; DestDir: "{app}\public\grex"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "..\public\desk\*"; DestDir: "{app}\public\desk"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "..\src\desk.grab"; DestDir: "{app}\src"; Flags: ignoreversion
Source: "..\views\workbench.grab.html"; DestDir: "{app}\views"; Flags: ignoreversion

; Walkthrough PDF Guide & Documentation
Source: "..\docs\releases\pdf\Groxy-Desk-v{#MyAppVersion}-Guide.pdf"; DestDir: "{app}\docs\releases\pdf"; Flags: ignoreversion
Source: "..\README.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\CHANGELOG.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\AGENTS.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\GROXY-DESK-EXPERT.md"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; WorkingDir: "{app}"; Comment: "Groxy Desk v{#MyAppVersion} — Native Desktop Vibe-Coding IDE & Unified Groxy AI Studio"
Name: "{autoprograms}\{#MyAppName} Walkthrough Guide (PDF)"; Filename: "{app}\docs\releases\pdf\Groxy-Desk-v{#MyAppVersion}-Guide.pdf"; Comment: "Plain-English Walkthrough & Architecture Guide for Groxy Desk v{#MyAppVersion}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; WorkingDir: "{app}"; Tasks: desktopicon; Comment: "Launch Groxy Desk v{#MyAppVersion}"

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent

[Code]
// Verify Microsoft Edge WebView2 Runtime presence (non-blocking informational check)
// and preserve all user workspaces, encrypted vaults (v1:...), and GROXY-SAFE-001 snapshots on uninstall.
function IsWebView2RuntimeInstalled(): Boolean;
var
  Version: String;
begin
  Result :=
    RegQueryStringValue(HKLM, 'SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}', 'pv', Version) or
    RegQueryStringValue(HKLM, 'SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}', 'pv', Version) or
    RegQueryStringValue(HKCU, 'Software\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}', 'pv', Version);
end;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssPostInstall then
  begin
    // User state directory ({localappdata}\groxy-desk) is intentionally separate from {app} ({localappdata}\Groxy Desk)
    // so uninstalling or upgrading Groxy Desk never deletes user workspaces, encrypted vaults, or snapshots.
  end;
end;
