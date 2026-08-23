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
$launcher = "$dir\start.ps1"
if ($Token) {
  $secureToken = ConvertTo-SecureString $Token -AsPlainText -Force
} elseif (Test-Path -LiteralPath $tokenFile) {
  $secureToken = (Get-Content -LiteralPath $tokenFile -Raw).Trim() | ConvertTo-SecureString
} else {
  $secureToken = Read-Host "hub token" -AsSecureString
}
if (-not $Server) { Write-Host "set LOCALCLIP_SERVER env or pass -Server"; exit 1 }
$sourceBin = "$PSScriptRoot\local-clipboard-windows-amd64.exe"
if (-not (Test-Path -LiteralPath $sourceBin)) {
  $sourceBin = [IO.Path]::GetFullPath("$PSScriptRoot\..\..\dist\local-clipboard-windows-amd64.exe")
}
if (-not (Test-Path -LiteralPath $sourceBin)) {
  throw "missing local-clipboard-windows-amd64.exe (build it into dist/ or place it beside install.ps1)"
}
New-Item -ItemType Directory -Force -Path $dir | Out-Null
Copy-Item -LiteralPath $sourceBin -Destination $bin -Force
@{ server = $Server; name = $Name } | ConvertTo-Json | Set-Content -LiteralPath $config -Encoding UTF8
$secureToken | ConvertFrom-SecureString | Set-Content -LiteralPath $tokenFile -Encoding ASCII
@'
$ErrorActionPreference = "Stop"
$dir = Split-Path -Parent $MyInvocation.MyCommand.Path
$bin = Join-Path $dir "local-clipboard.exe"
$config = Get-Content -LiteralPath (Join-Path $dir "config.json") -Raw | ConvertFrom-Json
$secure = (Get-Content -LiteralPath (Join-Path $dir "token.dpapi") -Raw).Trim() | ConvertTo-SecureString
$secretPtr = [IntPtr]::Zero
try {
  $secretPtr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
  $env:LOCALCLIP_TOKEN = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($secretPtr)
  & $bin run -server ([string]$config.server) -name ([string]$config.name)
} finally {
  if ($secretPtr -ne [IntPtr]::Zero) {
    [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($secretPtr)
  }
  Remove-Item Env:LOCALCLIP_TOKEN -ErrorAction SilentlyContinue
}
'@ | Set-Content -LiteralPath $launcher -Encoding UTF8
$runKey = "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run"
$runCommand = "powershell.exe -NoProfile -NonInteractive -WindowStyle Hidden -ExecutionPolicy Bypass -File `"$launcher`""
New-Item -Path $runKey -Force | Out-Null
Set-ItemProperty -Path $runKey -Name "LocalClipboardSync" -Value $runCommand
Start-Process -FilePath "powershell.exe" -WindowStyle Hidden -ArgumentList @(
  "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-ExecutionPolicy", "Bypass", "-File", "`"$launcher`""
)
Write-Host "installed and started. startup: HKCU Run / LocalClipboardSync"
