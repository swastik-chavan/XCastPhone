# ==============================================================================
# XCastPhone Windows PowerShell Installer
# ==============================================================================
# Usage:
#   iwr -useb https://raw.githubusercontent.com/swastik-chavan/XCastPhone/main/install.ps1 | iex
# ==============================================================================

$ErrorActionPreference = "Stop"

# ------------------------------------------------------------------------------
# Repository Configuration
# ------------------------------------------------------------------------------
$RepoOwner = "swastik-chavan"
$RepoName  = "XCastPhone"
$Version   = "latest"

$InstallDir = Join-Path $HOME ".xcast\bin"

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "       XCastPhone Windows Installer       " -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host ""

# 1. Ensure target directory exists
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

# 2. Detect Architecture
$Arch = "amd64"
if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
    $Arch = "arm64"
}
Write-Host "[+] Architecture: $Arch" -ForegroundColor Gray

# 3. Install XCastPhone Binary
$TargetBinary = Join-Path $InstallDir "xcast.exe"
$Installed = $false

# Check local repository or current working directory if available
$CandidateDirs = @()
if ($PSScriptRoot) {
    $CandidateDirs += $PSScriptRoot
    $CandidateDirs += (Join-Path $PSScriptRoot "..")
}
if ($PWD) {
    $CandidateDirs += $PWD.Path
}

foreach ($dir in $CandidateDirs) {
    if (-not $dir) { continue }
    $localExe = Join-Path $dir "xcast.exe"
    $localCmd = Join-Path $dir "cmd\xcast"
    if (Test-Path $localExe) {
        Write-Host "[+] Installing from local binary ($localExe)..." -ForegroundColor Green
        Copy-Item $localExe -Destination $TargetBinary -Force
        $Installed = $true
        break
    } elseif (Test-Path $localCmd) {
        if (Get-Command go -ErrorAction SilentlyContinue) {
            Write-Host "[+] Building xcast from local source ($dir)..." -ForegroundColor Green
            Push-Location $dir
            try {
                go build -o $TargetBinary ./cmd/xcast
                $Installed = $true
            } finally {
                Pop-Location
            }
            if ($Installed) { break }
        }
    }
}

# Remote installation if not installed locally
if (-not $Installed) {
    $DownloadUrl = "https://github.com/$RepoOwner/$RepoName/releases/$Version/download/xcast-windows-$Arch.exe"
    Write-Host "[+] Fetching XCastPhone..." -ForegroundColor Green

    $downloadSuccess = $false
    try {
        curl.exe -fL -o $TargetBinary $DownloadUrl 2>$null
        if ($LASTEXITCODE -eq 0 -and (Test-Path $TargetBinary)) {
            $downloadSuccess = $true
            $Installed = $true
        }
    } catch {}

    if (-not $downloadSuccess) {
        # Fallback: if Go is installed, download repository source archive and build
        if (Get-Command go -ErrorAction SilentlyContinue) {
            Write-Host "[+] Building latest xcast from GitHub source..." -ForegroundColor Green
            $tempZip = Join-Path $InstallDir "source.zip"
            $tempExtract = Join-Path $InstallDir "source_build"
            $zipUrl = "https://github.com/$RepoOwner/$RepoName/archive/refs/heads/main.zip"
            try {
                curl.exe -fL -o $tempZip $zipUrl
                if (-not (Test-Path $tempExtract)) { New-Item -ItemType Directory -Path $tempExtract -Force | Out-Null }
                tar.exe -xf $tempZip -C $tempExtract
                $sourceRoot = Join-Path $tempExtract "$RepoName-main"
                if (-not (Test-Path $sourceRoot)) {
                    $sourceRoot = (Get-ChildItem $tempExtract | Where-Object { $_.PSIsContainer } | Select-Object -First 1).FullName
                }
                Push-Location $sourceRoot
                go build -o $TargetBinary ./cmd/xcast
                Pop-Location
                $Installed = $true
            } catch {
                Write-Warning "Failed to build from source archive: $_"
            } finally {
                Remove-Item $tempZip -Force -ErrorAction SilentlyContinue
                Remove-Item $tempExtract -Recurse -Force -ErrorAction SilentlyContinue
            }
        } else {
            Write-Warning "Could not download prebuilt release. Install Go to build from source, or check https://github.com/$RepoOwner/$RepoName."
        }
    }
}

# 4. Check / Install Video Renderer (mpv)
$MpvInstalled = $false
if (Get-Command mpv -ErrorAction SilentlyContinue) {
    $MpvInstalled = $true
} elseif (Test-Path (Join-Path $InstallDir "mpv.exe")) {
    $MpvInstalled = $true
}

if (-not $MpvInstalled) {
    Write-Host "[+] Setting up lightweight hardware-accelerated video renderer (mpv)..." -ForegroundColor Yellow
    $MpvZip = Join-Path $InstallDir "mpv.7z"
    $MpvUrl = "https://github.com/shinchiro/mpv-winbuild-cmake/releases/download/20260928/mpv-x86_64-20260928-git-e470f8986e.7z"

    try {
        curl.exe -L -o $MpvZip $MpvUrl
        tar.exe -xf $MpvZip -C $InstallDir
        Remove-Item $MpvZip -Force -ErrorAction SilentlyContinue
        Write-Host "[+] Renderer (mpv) ready." -ForegroundColor Green
    } catch {
        Write-Warning "Could not automatically download portable mpv. Please install mpv manually via 'winget install shinchiro.mpv'."
    }
} else {
    Write-Host "[+] Video renderer: ready." -ForegroundColor Green
}

# 5. Check / Install ADB Platform-Tools
$AdbFound = $false
if (Get-Command adb -ErrorAction SilentlyContinue) {
    $AdbFound = $true
} elseif (Test-Path (Join-Path $InstallDir "adb.exe")) {
    $AdbFound = $true
} elseif (Test-Path "$env:LOCALAPPDATA\Android\Sdk\platform-tools\adb.exe") {
    $AdbFound = $true
}

if (-not $AdbFound) {
    Write-Host "[+] Android Platform Tools (ADB) not found. Setting up portable ADB..." -ForegroundColor Yellow
    $AdbZip = Join-Path $InstallDir "platform-tools.zip"
    $AdbUrl = "https://dl.google.com/android/repository/platform-tools-latest-windows.zip"
    try {
        curl.exe -L -o $AdbZip $AdbUrl
        tar.exe -xf $AdbZip -C $InstallDir
        $ExtractedDir = Join-Path $InstallDir "platform-tools"
        if (Test-Path $ExtractedDir) {
            Get-ChildItem $ExtractedDir | Move-Item -Destination $InstallDir -Force
            Remove-Item $ExtractedDir -Recurse -Force -ErrorAction SilentlyContinue
        }
        Remove-Item $AdbZip -Force -ErrorAction SilentlyContinue
        Write-Host "[+] ADB platform tools ready." -ForegroundColor Green
    } catch {
        Write-Warning "Could not automatically download ADB. Please install Android Platform Tools manually."
    }
} else {
    Write-Host "[+] ADB: ready." -ForegroundColor Green
}

# 6. Add $InstallDir to User PATH if not present
$UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($UserPath -notlike "*$InstallDir*") {
    Write-Host "[+] Adding $InstallDir to User PATH..." -ForegroundColor Cyan
    [Environment]::SetEnvironmentVariable("PATH", "$UserPath;$InstallDir", "User")
}
$env:PATH = "$env:PATH;$InstallDir"

Write-Host ""
Write-Host "==========================================" -ForegroundColor Green
Write-Host "   XCastPhone installed successfully!     " -ForegroundColor Green
Write-Host "==========================================" -ForegroundColor Green
Write-Host ""
Write-Host "Installed to: $TargetBinary"
Write-Host ""
Write-Host "To start casting, open a terminal and run:"
Write-Host "  xcast" -ForegroundColor Yellow
Write-Host ""
