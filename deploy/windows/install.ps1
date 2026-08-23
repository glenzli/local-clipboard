param(
  [string]$Server = $env:LOCALCLIP_SERVER,
  [string]$Token = $env:LOCALCLIP_TOKEN,
  [string]$Name = $env:COMPUTERNAME
)
$ErrorActionPreference = "Stop"
if (-not $Token) { $Token = Read-Host "hub token" }
if (-not $Server) { Write-Host "set LOCALCLIP_SERVER env or pass -Server"; exit 1 }
$dir = "$env:LOCALAPPDATA\local-clipboard"
$bin = "$dir\local-clipboard.exe"
New-Item -ItemType Directory -Force -Path $dir | Out-Null
Copy-Item "$PSScriptRoot\local-clipboard-windows-amd64.exe" $bin -Force
schtasks /Create /F /SC ONLOGON /TN "LocalClipboardSync" /TR "`"$bin`" run -server $Server -token $Token -name $Name" | Out-Null
Start-Process -WindowStyle Hidden $bin -ArgumentList "run","-server",$Server,"-token",$Token,"-name",$Name
Write-Host "installed and started. task: schtasks /Query /TN LocalClipboardSync"
