param(
  [string]$Server = $env:LOCALCLIP_SERVER,
  [string]$Token = $env:LOCALCLIP_TOKEN,
  [string]$Name = $env:COMPUTERNAME
)
$ErrorActionPreference = "Stop"
$dir = "$env:LOCALAPPDATA\local-clipboard"
$bin = "$dir\local-clipboard.exe"
$config = "$dir\config.json"
$tokenFile = "$dir\token.dpapi"
if (-not $Server -and (Test-Path -LiteralPath $config)) {
  $saved = Get-Content -LiteralPath $config -Raw | ConvertFrom-Json
  $Server = [string]$saved.server
  if (-not $PSBoundParameters.ContainsKey('Name')) { $Name = [string]$saved.name }
}
if (-not $Server) { throw "set LOCALCLIP_SERVER env or pass -Server" }
if ($Token) {
  $secureToken = ConvertTo-SecureString $Token -AsPlainText -Force
} elseif (Test-Path -LiteralPath $tokenFile) {
  $secureToken = (Get-Content -LiteralPath $tokenFile -Raw).Trim() | ConvertTo-SecureString
} else {
  $secureToken = Read-Host "hub token" -AsSecureString
}
$sourceBin = "$PSScriptRoot\local-clipboard-windows-amd64.exe"
if (-not (Test-Path -LiteralPath $sourceBin)) {
  $sourceBin = [IO.Path]::GetFullPath("$PSScriptRoot\..\..\dist\local-clipboard-windows-amd64.exe")
}
if (-not (Test-Path -LiteralPath $sourceBin)) {
  throw "missing local-clipboard-windows-amd64.exe (build it into dist/ or place it beside install.ps1)"
}
New-Item -ItemType Directory -Force -Path $dir | Out-Null
$running = Get-Process -Name local-clipboard -ErrorAction SilentlyContinue | Where-Object { $_.Path -eq $bin }
if ($running) {
  $running | Stop-Process -Force
  $running | Wait-Process -Timeout 10 -ErrorAction SilentlyContinue
}
$copyDeadline = (Get-Date).AddSeconds(10)
while ($true) {
  try {
    Copy-Item -LiteralPath $sourceBin -Destination $bin -Force
    break
  } catch [IO.IOException] {
    # Windows can retain the executable mapping briefly after process exit.
    if ((Get-Date) -ge $copyDeadline) { throw }
    Start-Sleep -Milliseconds 200
  }
}
@{ server = $Server; name = $Name } | ConvertTo-Json | Set-Content -LiteralPath $config -Encoding UTF8
$secureToken | ConvertFrom-SecureString | Set-Content -LiteralPath $tokenFile -Encoding ASCII
$runKey = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run"
$runCommand = "`"$bin`" run -config `"$config`""
New-Item -Path $runKey -Force | Out-Null
Set-ItemProperty -Path $runKey -Name "LocalClipboardSync" -Value $runCommand
$legacyLauncher = Join-Path $dir 'start.ps1'
if (Test-Path -LiteralPath $legacyLauncher) { Remove-Item -LiteralPath $legacyLauncher }
Start-Process -FilePath $bin -WindowStyle Hidden -WorkingDirectory $dir -ArgumentList @("run", "-config", "`"$config`"")
Write-Host "installed and started. startup: HKCU Run / LocalClipboardSync"
Write-Host "log: $dir\local-clipboard.log"
