# Run using either Windows PowerShell 5.1 or PowerShell 7.
[CmdletBinding()]
param([string]$Binary = "bin/mosaic.exe")

$ErrorActionPreference = "Stop"
Push-Location (Split-Path $PSScriptRoot -Parent)
try {
    $exe = (Resolve-Path $Binary).Path
    $version = & $exe --version
    if ($LASTEXITCODE -ne 0 -or $version -notmatch '^mosaic ') { throw "Version command failed" }
    $snapshot = (& $exe --demo --snapshot) -join "`n"
    if ($LASTEXITCODE -ne 0 -or $snapshot -notmatch 'MOSAIC' -or $snapshot -notmatch 'PROCESSES') {
        throw "Snapshot command failed"
    }
    if ($snapshot.Contains([string][char]27)) { throw "Plain snapshot contains terminal escapes" }
    $data = ((& $exe --demo --json) -join "`n") | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0 -or -not $data.Host) { throw "JSON command failed" }
    # PowerShell 5.1 converts redirected native stderr to ErrorRecord objects.
    $ErrorActionPreference = "Continue"
    & $exe --interval 1ms 2>$null
    $ErrorActionPreference = "Stop"
    if ($LASTEXITCODE -eq 0) { throw "Invalid interval did not fail" }
    # The final native command deliberately failed. Actions checks this global
    # after the script, so report success once the expected failure is verified.
    $global:LASTEXITCODE = 0
    Write-Host "PowerShell smoke passed: executable launch, plain output, JSON, exit codes"
} finally {
    Pop-Location
}
