<#
.SYNOPSIS
    GCC-only setup helper (use setup_build.ps1 for full environment setup).

.DESCRIPTION
    Checks for an existing GCC installation, and if none is found,
    installs MinGW-w64 via winget and adds it to your user PATH.

    For a complete first-time setup (Go + GCC + make), use:
        powershell -ExecutionPolicy Bypass -File scripts/setup_build.ps1

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File scripts/setup-gcc.ps1
#>

$ErrorActionPreference = "Stop"

Write-Host ""
Write-Host "=== codeindex-mcp: GCC setup ===" -ForegroundColor Cyan
Write-Host ""

# --- 1. Check if gcc is already usable ---
$gccInPath = Get-Command gcc -ErrorAction SilentlyContinue
if ($gccInPath) {
    $ver = & gcc --version 2>&1 | Select-Object -First 1
    Write-Host "OK: gcc already available in PATH." -ForegroundColor Green
    Write-Host "    $ver" -ForegroundColor DarkGray
    Write-Host ""
    Write-Host "You can build right now (restart terminal if this is a fresh install):" -ForegroundColor Yellow
    Write-Host "  make build" -ForegroundColor Cyan
    Write-Host "  make test" -ForegroundColor Cyan
    exit 0
}

# --- 2. Search common MinGW installation locations ---
$candidates = @(
    # MSYS2 (most common)
    "C:\msys64\mingw64\bin",
    "C:\msys2\mingw64\bin",
    # Standalone MinGW
    "C:\mingw64\bin",
    "C:\mingw\bin",
    "C:\MinGW\bin",
    # Scoop
    "$env:USERPROFILE\scoop\apps\mingw\current\bin",
    # winget (any BrechtSanders WinLibs variant)
    (Get-ChildItem "$env:LOCALAPPDATA\Microsoft\WinGet\Packages" -Directory -Filter "BrechtSanders.WinLibs*" -ErrorAction SilentlyContinue |
     ForEach-Object { Join-Path $_.FullName "mingw64\bin" } |
     Where-Object { Test-Path $_ } |
     Select-Object -First 1),
    # Chocolatey
    "C:\ProgramData\chocolatey\lib\mingw\tools\install\mingw64\bin"
)

$found = $candidates | Where-Object { $_ -and (Test-Path "$_\gcc.exe") } | Select-Object -First 1

if ($found) {
    Write-Host "Found existing GCC at: $found" -ForegroundColor Green
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -notlike "*$found*") {
        [Environment]::SetEnvironmentVariable("Path", "$userPath;$found", "User")
        Write-Host "Added to user PATH." -ForegroundColor Green
    } else {
        Write-Host "Already in user PATH." -ForegroundColor DarkGray
    }

    # Create make.exe alias if missing
    $makeExe = Join-Path $found "make.exe"
    $mingwMake = Join-Path $found "mingw32-make.exe"
    if ((Test-Path $mingwMake) -and -not (Test-Path $makeExe)) {
        Copy-Item $mingwMake $makeExe
        Write-Host "Created make.exe alias for mingw32-make.exe." -ForegroundColor DarkGray
    }

    Write-Host ""
    Write-Host "NEXT: Restart your terminal, then:" -ForegroundColor Yellow
    Write-Host "  make build" -ForegroundColor Cyan
    exit 0
}

# --- 3. Nothing found — install via winget ---
Write-Host "No GCC found. Installing MinGW-w64 via winget..." -ForegroundColor Yellow
Write-Host ""

winget install `
    --id BrechtSanders.WinLibs.POSIX.UCRT `
    --accept-source-agreements `
    --accept-package-agreements

Write-Host ""
Write-Host "Installation complete. Re-running detection..." -ForegroundColor DarkGray

# Re-detect after install
$newFound = Get-ChildItem "$env:LOCALAPPDATA\Microsoft\WinGet\Packages" -Directory `
                -Filter "BrechtSanders.WinLibs*" -ErrorAction SilentlyContinue |
            ForEach-Object { Join-Path $_.FullName "mingw64\bin" } |
            Where-Object { Test-Path "$_\gcc.exe" } |
            Select-Object -First 1

if (-not $newFound) {
    Write-Error "Installation finished but gcc.exe not found. Please restart and re-run this script."
    exit 1
}

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
[Environment]::SetEnvironmentVariable("Path", "$userPath;$newFound", "User")

$makeExe = Join-Path $newFound "make.exe"
$mingwMake = Join-Path $newFound "mingw32-make.exe"
if ((Test-Path $mingwMake) -and -not (Test-Path $makeExe)) {
    Copy-Item $mingwMake $makeExe
    Write-Host "Created make.exe alias for mingw32-make.exe." -ForegroundColor DarkGray
}

Write-Host ""
Write-Host "GCC + make added to PATH:" -ForegroundColor Green
Write-Host "  $newFound" -ForegroundColor DarkGray
Write-Host ""
Write-Host "NEXT: Restart your terminal, then:" -ForegroundColor Yellow
Write-Host "  make build" -ForegroundColor Cyan
Write-Host "  make test" -ForegroundColor Cyan
Write-Host ""
