<#
.SYNOPSIS
    Build environment setup for codeindex-mcp on Windows.

.DESCRIPTION
    Two build variants are available:

      make build       Lightweight — Go AST only, no CGo, no GCC required (~18 MB)
      make build-ast   Full — all 4 languages via tree-sitter, CGo + GCC required (~31 MB)

    This script checks Go and make (always required), and GCC (only for build-ast).
    Missing tools are installed automatically via winget.

    Run once, then restart your terminal.

.EXAMPLE
    powershell -ExecutionPolicy Bypass -File scripts/setup_build.ps1
#>

$ErrorActionPreference = "Stop"

# ── Required versions ──────────────────────────────────────────────────────────
$MIN_GO_MAJOR = 1
$MIN_GO_MINOR = 25

# ── Output helpers ─────────────────────────────────────────────────────────────
function ok($msg)   { Write-Host "  [OK] $msg" -ForegroundColor Green }
function warn($msg) { Write-Host "  [!!] $msg" -ForegroundColor Yellow }
function err($msg)  { Write-Host "  [X]  $msg" -ForegroundColor Red }
function info($msg) { Write-Host "       $msg" -ForegroundColor DarkGray }
function hdr($msg)  { Write-Host ""; Write-Host "── $msg" -ForegroundColor Cyan }

function Ask-Install($what) {
    $ans = Read-Host "  --> Install $what automatically? [Y/n]"
    return ($ans -eq "" -or $ans -match "^[Yy]")
}

function Refresh-Path {
    $machine = [Environment]::GetEnvironmentVariable("Path", "Machine")
    $user    = [Environment]::GetEnvironmentVariable("Path", "User")
    $env:PATH = "$machine;$user"
}

# ── Winget availability ────────────────────────────────────────────────────────
function Assert-Winget {
    if (-not (Get-Command winget -ErrorAction SilentlyContinue)) {
        err "winget not found."
        info "Install the App Installer from the Microsoft Store, or"
        info "update to Windows 10 21H1+ / Windows 11."
        return $false
    }
    return $true
}

# ── Go ─────────────────────────────────────────────────────────────────────────
function Get-GoVersion {
    $cmd = Get-Command go -ErrorAction SilentlyContinue
    if (-not $cmd) { return $null, "not installed" }
    $raw = & go version 2>&1
    if ($raw -match 'go(\d+)\.(\d+)') {
        return [int]$Matches[1], [int]$Matches[2], $raw
    }
    return $null, "unrecognised output: $raw"
}

function Check-Go {
    hdr "Go"
    $result = Get-GoVersion
    $major  = $result[0]
    $minor  = $result[1]
    $raw    = $result[2]

    if ($null -eq $major) {
        err "Go not found ($($result[1]))"
        if (-not (Assert-Winget)) { return $false }
        if (Ask-Install "Go $MIN_GO_MAJOR.$MIN_GO_MINOR+") {
            winget install --id GoLang.Go --source winget --accept-source-agreements --accept-package-agreements
            Refresh-Path
            $result2 = Get-GoVersion
            $major = $result2[0]; $minor = $result2[1]; $raw = $result2[2]
            if ($null -eq $major) {
                err "Go still not found after install. Restart terminal and re-run."
                return $false
            }
        } else {
            warn "Manual install: https://go.dev/dl/"
            return $false
        }
    }

    if ($major -gt $MIN_GO_MAJOR -or ($major -eq $MIN_GO_MAJOR -and $minor -ge $MIN_GO_MINOR)) {
        ok "$raw"
        return $true
    }

    err "Go $major.$minor found but $MIN_GO_MAJOR.$MIN_GO_MINOR+ required."
    info "Download the latest installer from: https://go.dev/dl/"
    if (-not (Assert-Winget)) { return $false }
    if (Ask-Install "Go (upgrade)") {
        winget upgrade --id GoLang.Go --source winget --accept-source-agreements --accept-package-agreements
        Refresh-Path
        ok "Go upgraded. $(& go version 2>&1)"
        return $true
    }
    return $false
}

# ── GCC ────────────────────────────────────────────────────────────────────────
function Find-MinGW {
    # Search all common Windows GCC locations
    $candidates = @(
        # MSYS2
        "C:\msys64\mingw64\bin",
        "C:\msys2\mingw64\bin",
        # Standalone
        "C:\mingw64\bin",
        "C:\mingw\bin",
        "C:\MinGW\bin",
        # Scoop
        "$env:USERPROFILE\scoop\apps\mingw\current\bin",
        # Chocolatey
        "C:\ProgramData\chocolatey\lib\mingw\tools\install\mingw64\bin"
    )
    # winget (any BrechtSanders WinLibs variant)
    $wingetBase = "$env:LOCALAPPDATA\Microsoft\WinGet\Packages"
    if (Test-Path $wingetBase) {
        $wingetCandidates = Get-ChildItem $wingetBase -Directory -Filter "BrechtSanders.WinLibs*" -ErrorAction SilentlyContinue |
            ForEach-Object { Join-Path $_.FullName "mingw64\bin" }
        $candidates = $wingetCandidates + $candidates
    }
    return $candidates | Where-Object { $_ -and (Test-Path "$_\gcc.exe") } | Select-Object -First 1
}

function Ensure-MakeAlias($binDir) {
    $makeExe   = Join-Path $binDir "make.exe"
    $mingwMake = Join-Path $binDir "mingw32-make.exe"
    if ((Test-Path $mingwMake) -and -not (Test-Path $makeExe)) {
        Copy-Item $mingwMake $makeExe
        info "Created make.exe → mingw32-make.exe"
    }
}

function Check-GCC {
    hdr "GCC (C compiler for CGo)"

    # Already in PATH?
    if (Get-Command gcc -ErrorAction SilentlyContinue) {
        ok "$(& gcc --version 2>&1 | Select-Object -First 1)"
        return $true
    }

    # Exists somewhere but not in PATH?
    $found = Find-MinGW
    if ($found) {
        ok "Found at: $found"
        $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
        if ($userPath -notlike "*$found*") {
            [Environment]::SetEnvironmentVariable("Path", "$userPath;$found", "User")
            $env:PATH = "$env:PATH;$found"
            info "Added to PATH."
        }
        Ensure-MakeAlias $found
        return $true
    }

    # Not found — install
    err "GCC not found."
    if (-not (Assert-Winget)) { return $false }
    if (Ask-Install "MinGW-w64 (GCC for Windows)") {
        winget install `
            --id BrechtSanders.WinLibs.POSIX.UCRT `
            --accept-source-agreements `
            --accept-package-agreements
        Refresh-Path
        $found = Find-MinGW
        if (-not $found) {
            err "GCC still not found after install. Restart terminal and re-run."
            return $false
        }
        $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
        if ($userPath -notlike "*$found*") {
            [Environment]::SetEnvironmentVariable("Path", "$userPath;$found", "User")
            $env:PATH = "$env:PATH;$found"
        }
        Ensure-MakeAlias $found
        ok "GCC installed: $(& gcc --version 2>&1 | Select-Object -First 1)"
        return $true
    }

    warn "Manual install options:"
    info "  winget install BrechtSanders.WinLibs.POSIX.UCRT"
    info "  or: https://www.msys2.org/"
    return $false
}

# ── make ───────────────────────────────────────────────────────────────────────
function Check-Make {
    hdr "make"
    if (Get-Command make -ErrorAction SilentlyContinue) {
        ok "$(& make --version 2>&1 | Select-Object -First 1)"
        return $true
    }
    # After GCC check, mingw32-make.exe exists and we may have created make.exe
    if (Get-Command make -ErrorAction SilentlyContinue) {
        ok "$(& make --version 2>&1 | Select-Object -First 1)"
        return $true
    }
    warn "make not found — the Makefile targets won't work."
    info "Install MSYS2 (https://www.msys2.org/) which includes make, or"
    info "use the manual command directly (shown in the summary below)."
    return $false  # non-fatal
}

# ── Main ───────────────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "╔══════════════════════════════════════════╗" -ForegroundColor Cyan
Write-Host "║   codeindex-mcp  —  build setup (Win)   ║" -ForegroundColor Cyan
Write-Host "╚══════════════════════════════════════════╝" -ForegroundColor Cyan

$goOk   = Check-Go
$gccOk  = Check-GCC
$makeOk = Check-Make

# ── Summary ────────────────────────────────────────────────────────────────────
hdr "Summary"

if (-not $goOk) {
    err "Go $MIN_GO_MAJOR.$MIN_GO_MINOR+ is required."
    Write-Host ""
    warn "Fix the error above, then re-run this script."
    Write-Host ""
    exit 1
}

Write-Host ""

if ($makeOk) {
    Write-Host "  Restart your terminal, then:" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "  Lightweight build  (no GCC needed, Go AST only):" -ForegroundColor DarkGray
    Write-Host "    make build" -ForegroundColor Cyan
    Write-Host "    make test" -ForegroundColor Cyan
    if ($gccOk) {
        Write-Host ""
        Write-Host "  Full AST build  (GCC ready, all 4 languages):" -ForegroundColor DarkGray
        Write-Host "    make build-ast" -ForegroundColor Cyan
        Write-Host "    make test-ast" -ForegroundColor Cyan
    } else {
        Write-Host ""
        Write-Host "  Full AST build  (TypeScript/Python/JavaScript support, requires GCC):" -ForegroundColor DarkGray
        warn "GCC not installed — run this script again to install it, then use 'make build-ast'."
    }
} else {
    Write-Host "  Restart your terminal, then:" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "  Lightweight:  go build -o codeindex-mcp.exe ." -ForegroundColor Cyan
    if ($gccOk) {
        Write-Host "  Full AST:     CGO_ENABLED=1 go build -tags ast -o codeindex-mcp.exe ." -ForegroundColor Cyan
    }
}

Write-Host ""
