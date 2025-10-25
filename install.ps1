# SkyPort Agent Installer for Windows
# https://github.com/anushrevankar24/skyport-agent

$ErrorActionPreference = "Stop"

$BINARY_NAME = "skyport.exe"
$GITHUB_REPO = "anushrevankar24/skyport-agent"
$INSTALL_DIR = "$env:LOCALAPPDATA\SkyPort"

Write-Host ""
Write-Host "Installing SkyPort Agent..." -ForegroundColor Cyan
Write-Host ""

# Detect architecture
$ARCH = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }

Write-Host "Detected system: windows-$ARCH" -ForegroundColor White

# Build download URL
$DOWNLOAD_URL = "https://github.com/$GITHUB_REPO/releases/latest/download/skyport-windows-$ARCH.exe"

Write-Host "Downloading from GitHub releases..." -ForegroundColor White
Write-Host ""

# Create install directory if it doesn't exist
if (-not (Test-Path $INSTALL_DIR)) {
    New-Item -ItemType Directory -Path $INSTALL_DIR -Force | Out-Null
}

$BINARY_PATH = Join-Path $INSTALL_DIR $BINARY_NAME

# Download the binary
try {
    $ProgressPreference = 'SilentlyContinue'
    Invoke-WebRequest -Uri $DOWNLOAD_URL -OutFile $BINARY_PATH -UseBasicParsing
    Write-Host "Download complete" -ForegroundColor Green
} catch {
    Write-Host ""
    Write-Host "Error: Download failed!" -ForegroundColor Red
    Write-Host ""
    Write-Host "Please check:" -ForegroundColor Yellow
    Write-Host "  1. Your internet connection"
    Write-Host "  2. The release exists at: $DOWNLOAD_URL"
    Write-Host "  3. Visit https://github.com/$GITHUB_REPO/releases for manual download"
    Write-Host ""
    Write-Host "If this is a new release, it may take a few minutes to become available." -ForegroundColor Yellow
    Write-Host ""
    exit 1
}

Write-Host ""
Write-Host "Installing to: $INSTALL_DIR" -ForegroundColor White

# Add to PATH if not already there
$USER_PATH = [Environment]::GetEnvironmentVariable("Path", "User")
if ($USER_PATH -notlike "*$INSTALL_DIR*") {
    Write-Host "Adding to PATH..." -ForegroundColor White
    try {
        [Environment]::SetEnvironmentVariable(
            "Path",
            "$USER_PATH;$INSTALL_DIR",
            "User"
        )
        $env:Path += ";$INSTALL_DIR"
        Write-Host "Added to PATH" -ForegroundColor Green
    } catch {
        Write-Host "Warning: Could not add to PATH automatically" -ForegroundColor Yellow
        Write-Host "Please add manually: $INSTALL_DIR" -ForegroundColor Yellow
    }
} else {
    Write-Host "Already in PATH" -ForegroundColor Green
}

Write-Host ""
Write-Host "SkyPort Agent installed successfully!" -ForegroundColor Green
Write-Host ""
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
Write-Host ""
Write-Host "Get started:" -ForegroundColor White
Write-Host "   skyport login" -ForegroundColor Yellow
Write-Host ""
Write-Host "View available commands:" -ForegroundColor White
Write-Host "   skyport --help" -ForegroundColor Yellow
Write-Host ""
Write-Host "Check installation:" -ForegroundColor White
Write-Host "   skyport --version" -ForegroundColor Yellow
Write-Host ""
Write-Host "Note: You may need to restart your terminal" -ForegroundColor Yellow
Write-Host "   for PATH changes to take effect." -ForegroundColor Yellow
Write-Host ""
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
Write-Host ""

