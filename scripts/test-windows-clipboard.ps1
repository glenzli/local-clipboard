# Run with: powershell.exe -STA -NoProfile -File scripts/test-windows-clipboard.ps1
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
if ([Threading.Thread]::CurrentThread.GetApartmentState() -ne 'STA') {
  throw 'Run this script with powershell.exe -STA.'
}
$backup = New-Object System.Windows.Forms.DataObject
$current = [System.Windows.Forms.Clipboard]::GetDataObject()
if ($current) {
  foreach ($format in $current.GetFormats($false)) {
    $value = $current.GetData($format, $false)
    if ($value -is [IO.MemoryStream]) {
      $value = New-Object IO.MemoryStream(,$value.ToArray())
    } elseif ($value -is [System.Drawing.Image]) {
      $value = $value.Clone()
    }
    if ($null -ne $value) { $backup.SetData($format, $false, $value) }
  }
}
$previousTestSetting = $env:LOCALCLIP_TEST_CLIPBOARD
Push-Location (Split-Path -Parent $PSScriptRoot)
try {
  $env:LOCALCLIP_TEST_CLIPBOARD = '1'
  go test ./internal/client -run '^TestWindowsClipboardImages$' -count=1 -v
  $testExitCode = $LASTEXITCODE
} finally {
  [System.Windows.Forms.Clipboard]::SetDataObject($backup, $true)
  $env:LOCALCLIP_TEST_CLIPBOARD = $previousTestSetting
  Pop-Location
}
exit $testExitCode
