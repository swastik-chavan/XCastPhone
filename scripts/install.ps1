# ==============================================================================
# XCastPhone Windows PowerShell Installer
# ==============================================================================
# Installation:
#   iwr -useb https://raw.githubusercontent.com/<OWNER>/xcastphone/main/scripts/install.ps1 | iex
# ==============================================================================

$ErrorActionPreference = "Stop"

# ------------------------------------------------------------------------------
# CONFIGURATION - Set your GitHub username or organization
# ------------------------------------------------------------------------------
$RepoOwner = "<OWNER>"
$RepoName  = "xcastphone"
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

# If running locally inside repository
$LocalBuild = Join-Path $PSScriptRoot "..\xcast.exe"
$LocalCmdDir = Join-Path $PSScriptRoot "..\cmd\xcast"

if (Test-Path $LocalBuild) {
    Write-Host "[+] Installing local build of xcast.exe..." -ForegroundColor Green
    Copy-Item $LocalBuild -Destination $TargetBinary -Force
} elseif (Test-Path $LocalCmdDir) {
    if (Get-Command go -ErrorAction SilentlyContinue) {
        Write-Host "[+] Building xcast from local source..." -ForegroundColor Green
        Push-Location (Join-Path $PSScriptRoot "..")
        go build -o $TargetBinary ./cmd/xcast
        Pop-Location
    }
} else {
    # Download from GitHub Releases
    $DownloadUrl = "https://github.com/$RepoOwner/$RepoName/releases/$Version/download/xcast-windows-$Arch.exe"
    Write-Host "[+] Downloading XCastPhone from $DownloadUrl..." -ForegroundColor Green
    try {
        curl.exe -fL -o $TargetBinary $DownloadUrl
    } catch {
        Write-Warning "Remote download failed. If repository owner '<OWNER>' hasn't been set, build locally via 'go build -o xcast.exe ./cmd/xcast'."
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
        # Move binaries directly into $InstallDir
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
