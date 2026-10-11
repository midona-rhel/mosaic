[CmdletBinding()]
param(
    [string]$Output = "bin/mosaic.exe"
)

$ErrorActionPreference = "Stop"
Push-Location (Split-Path $PSScriptRoot -Parent)
try {
    $parent = Split-Path $Output -Parent
    if ($parent) { New-Item -ItemType Directory -Force -Path $parent | Out-Null }
    & go build -trimpath -o $Output ./cmd/mosaic
    if ($LASTEXITCODE -ne 0) { throw "Go build failed with exit code $LASTEXITCODE" }
    Write-Host "Built $Output. Run it directly in your terminal."
} finally {
    Pop-Location
}
