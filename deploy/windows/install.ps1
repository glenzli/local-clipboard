param(
  [Parameter(Mandatory=$true)][string]$Server,
  [Parameter(Mandatory=$true)][string]$Token,
  [string]$Name = $env:COMPUTERNAME
)
$ErrorActionPreference = "Stop"
$dir = "$env:LOCALAPPDATA\local-clipboard"
$bin = "$dir\local-clipboard.exe"
New-Item -ItemType Directory -Force -Path $dir | Out-Null
Copy-Item "$PSScriptRoot\local-clipboard-windows-amd64.exe" $bin -Force
schtasks /Create /F /SC ONLOGON /TN "LocalClipboardSync" /TR "`"$bin`" run -server $Server -token $Token -name $Name" | Out-Null
Start-Process -WindowStyle Hidden $bin -ArgumentList "run","-server",$Server,"-token",$Token,"-name",$Name
Write-Host "installed and started. task: schtasks /Query /TN LocalClipboardSync"
