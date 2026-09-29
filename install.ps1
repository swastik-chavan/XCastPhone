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
$Branch    = "main"
$RepoUrl   = "https://github.com/$RepoOwner/$RepoName"
$RawBaseUrl = "https://raw.githubusercontent.com/$RepoOwner/$RepoName/$Branch"

# ------------------------------------------------------------------------------
# Installation Directory Setup
# ------------------------------------------------------------------------------
$HomeDir = [Environment]::GetFolderPath([Environment+SpecialFolder]::UserProfile)
if ([string]::IsNullOrWhiteSpace($HomeDir)) {
    $HomeDir = $env:USERPROFILE
}
if ([string]::IsNullOrWhiteSpace($HomeDir)) {
    $HomeDir = $HOME
}

$InstallDir = Join-Path -Path $HomeDir -ChildPath ".xcast\bin"

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "       XCastPhone Windows Installer       " -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host ""

if (-not (Test-Path -Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

# ------------------------------------------------------------------------------
# 1. Architecture Detection
# ------------------------------------------------------------------------------
$Arch = "amd64"
if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
    $Arch = "arm64"
}
Write-Host "[+] Architecture: $Arch" -ForegroundColor Gray

# ------------------------------------------------------------------------------
# 2. Install XCastPhone Binary
# ------------------------------------------------------------------------------
$TargetBinary = Join-Path -Path $InstallDir -ChildPath "xcast.exe"
$Installed = $false

# Check local candidate directories (local repo execution or current working directory)
$CandidateDirs = @()
if (-not [string]::IsNullOrWhiteSpace($PSScriptRoot)) {
    $CandidateDirs += $PSScriptRoot
    $CandidateDirs += (Split-Path -Parent $PSScriptRoot)
}

$CurrentLocation = (Get-Location).Path
if (-not [string]::IsNullOrWhiteSpace($CurrentLocation)) {
    $CandidateDirs += $CurrentLocation
}

foreach ($cDir in $CandidateDirs) {
    if ([string]::IsNullOrWhiteSpace($cDir)) { continue }

    $localExe = Join-Path -Path $cDir -ChildPath "xcast.exe"
    $localCmd = Join-Path -Path $cDir -ChildPath "cmd\xcast"

    if (Test-Path -Path $localExe) {
        Write-Host "[+] Installing from local binary ($localExe)..." -ForegroundColor Green
        Copy-Item -Path $localExe -Destination $TargetBinary -Force
        $Installed = $true
        break
    } elseif ((Test-Path -Path $localCmd) -and (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Host "[+] Building xcast from local source ($cDir)..." -ForegroundColor Green
        Push-Location -Path $cDir
        try {
            go build -o $TargetBinary ./cmd/xcast
            $Installed = $true
        } finally {
            Pop-Location
        }
        if ($Installed) { break }
    }
}

# Remote installation if not installed from local files
if (-not $Installed) {
    Write-Host "[+] Fetching XCastPhone..." -ForegroundColor Green
    $TempBase = [System.IO.Path]::GetTempPath()
    $TempWorkingDir = Join-Path -Path $TempBase -ChildPath ("xcast_install_" + [System.Guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Path $TempWorkingDir -Force | Out-Null

    try {
        # Try downloading prebuilt release executable
        $ReleaseBinaryUrl = "$RepoUrl/releases/latest/download/xcast-windows-$Arch.exe"
        $TempBinary = Join-Path -Path $TempWorkingDir -ChildPath "xcast_download.exe"

        $downloadOk = $false
        try {
            curl.exe -fL -o $TempBinary $ReleaseBinaryUrl 2>$null
            if (($LASTEXITCODE -eq 0) -and (Test-Path -Path $TempBinary)) {
                $downloadOk = $true
                Copy-Item -Path $TempBinary -Destination $TargetBinary -Force
                $Installed = $true
            }
        } catch {}

        # Fallback: if Go is installed, download repository source archive and build
        if (-not $downloadOk -and (Get-Command go -ErrorAction SilentlyContinue)) {
            Write-Host "[+] Building latest xcast from GitHub source archive..." -ForegroundColor Green
            $SourceZipUrl = "$RepoUrl/archive/refs/heads/$Branch.zip"
            $SourceZipPath = Join-Path -Path $TempWorkingDir -ChildPath "source.zip"
            $SourceExtractDir = Join-Path -Path $TempWorkingDir -ChildPath "extracted"

            curl.exe -fL -o $SourceZipPath $SourceZipUrl
            New-Item -ItemType Directory -Path $SourceExtractDir -Force | Out-Null
            tar.exe -xf $SourceZipPath -C $SourceExtractDir

            $extractedDirs = Get-ChildItem -Path $SourceExtractDir | Where-Object { $_.PSIsContainer }
            $sourceDir = $null
            if ($extractedDirs -and $extractedDirs.Count -gt 0) {
                $sourceDir = $extractedDirs[0].FullName
            }

            if (-not [string]::IsNullOrWhiteSpace($sourceDir) -and (Test-Path -Path $sourceDir)) {
                Push-Location -Path $sourceDir
                try {
                    go build -o $TargetBinary ./cmd/xcast
                    $Installed = $true
                } finally {
                    Pop-Location
                }
            }
        }
    } finally {
        if (Test-Path -Path $TempWorkingDir) {
            Remove-Item -Path $TempWorkingDir -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

if (-not $Installed -or -not (Test-Path -Path $TargetBinary)) {
    throw "Failed to install xcast.exe. Please ensure Go is installed or check $RepoUrl"
}

# ------------------------------------------------------------------------------
# 3. Check / Install Video Renderer (mpv)
# ------------------------------------------------------------------------------
$MpvInstalled = $false
$MpvBinary = Join-Path -Path $InstallDir -ChildPath "mpv.exe"

if (Get-Command mpv -ErrorAction SilentlyContinue) {
    $MpvInstalled = $true
} elseif (Test-Path -Path $MpvBinary) {
    $MpvInstalled = $true
}

if (-not $MpvInstalled) {
    Write-Host "[+] Setting up lightweight hardware-accelerated video renderer (mpv)..." -ForegroundColor Yellow
    $TempBase = [System.IO.Path]::GetTempPath()
    $MpvZip = Join-Path -Path $TempBase -ChildPath ("mpv_" + [System.Guid]::NewGuid().ToString("N") + ".7z")
    $MpvUrl = "https://github.com/shinchiro/mpv-winbuild-cmake/releases/download/20260928/mpv-x86_64-20260928-git-e470f8986e.7z"

    try {
        curl.exe -L -o $MpvZip $MpvUrl
        tar.exe -xf $MpvZip -C $InstallDir
        Write-Host "[+] Renderer (mpv) ready." -ForegroundColor Green
    } catch {
        Write-Warning "Could not automatically download portable mpv. Install mpv manually via 'winget install shinchiro.mpv'."
    } finally {
        if (Test-Path -Path $MpvZip) {
            Remove-Item -Path $MpvZip -Force -ErrorAction SilentlyContinue
        }
    }
} else {
    Write-Host "[+] Video renderer: ready." -ForegroundColor Green
}

# ------------------------------------------------------------------------------
# 4. Check / Install ADB Platform-Tools
# ------------------------------------------------------------------------------
$AdbFound = $false
$LocalAdb = Join-Path -Path $InstallDir -ChildPath "adb.exe"

if (Get-Command adb -ErrorAction SilentlyContinue) {
    $AdbFound = $true
} elseif (Test-Path -Path $LocalAdb) {
    $AdbFound = $true
} elseif (-not [string]::IsNullOrWhiteSpace($env:LOCALAPPDATA)) {
    $SdkAdb = Join-Path -Path $env:LOCALAPPDATA -ChildPath "Android\Sdk\platform-tools\adb.exe"
    if (Test-Path -Path $SdkAdb) {
        $AdbFound = $true
    }
}

if (-not $AdbFound) {
    Write-Host "[+] Android Platform Tools (ADB) not found. Setting up portable ADB..." -ForegroundColor Yellow
    $TempBase = [System.IO.Path]::GetTempPath()
    $AdbZip = Join-Path -Path $TempBase -ChildPath ("adb_" + [System.Guid]::NewGuid().ToString("N") + ".zip")
    $AdbUrl = "https://dl.google.com/android/repository/platform-tools-latest-windows.zip"
    try {
        curl.exe -L -o $AdbZip $AdbUrl
        tar.exe -xf $AdbZip -C $InstallDir
        $ExtractedDir = Join-Path -Path $InstallDir -ChildPath "platform-tools"
        if (Test-Path -Path $ExtractedDir) {
            Get-ChildItem -Path $ExtractedDir | Move-Item -Destination $InstallDir -Force
            Remove-Item -Path $ExtractedDir -Recurse -Force -ErrorAction SilentlyContinue
        }
        Write-Host "[+] ADB platform tools ready." -ForegroundColor Green
    } catch {
        Write-Warning "Could not automatically download ADB. Please install Android Platform Tools manually."
    } finally {
        if (Test-Path -Path $AdbZip) {
            Remove-Item -Path $AdbZip -Force -ErrorAction SilentlyContinue
        }
    }
} else {
    Write-Host "[+] ADB: ready." -ForegroundColor Green
}

# ------------------------------------------------------------------------------
# 5. Add $InstallDir to User and Session PATH
# ------------------------------------------------------------------------------
$UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ([string]::IsNullOrWhiteSpace($UserPath)) {
    $UserPath = ""
}

$UserPathEntries = $UserPath -split ';' | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }
if ($UserPathEntries -notcontains $InstallDir) {
    Write-Host "[+] Adding $InstallDir to User PATH..." -ForegroundColor Cyan
    $NewUserPath = ($UserPathEntries + $InstallDir) -join ';'
    [Environment]::SetEnvironmentVariable("PATH", $NewUserPath, "User")
}

$CurrentPathEntries = $env:PATH -split ';' | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }
if ($CurrentPathEntries -notcontains $InstallDir) {
    $env:PATH = "$env:PATH;$InstallDir"
}

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
