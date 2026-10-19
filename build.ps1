$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

$go = if ($env:GO) { $env:GO } else { "go" }
$winres = if ($env:WINRES) { $env:WINRES } else { "go-winres" }

Write-Host "[1/4] tests"
& $go test ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "[2/4] resources"
& $winres make --arch amd64
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "[3/4] build"
New-Item -ItemType Directory -Force -Path dist | Out-Null
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
& $go build -trimpath -ldflags "-H windowsgui -s -w" -o "dist\墨阅.exe" .
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "[4/4] checksum"
$hash = (Get-FileHash "dist\墨阅.exe" -Algorithm SHA256).Hash.ToLower()
"$hash" | Set-Content -Encoding ascii "dist\SHA256SUMS.txt"

Get-Item "dist\墨阅.exe" | Format-List Name, Length, LastWriteTime
