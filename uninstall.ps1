# SkyPort Agent Uninstaller for Windows
# Usage: irm https://raw.githubusercontent.com/your-org/skyport-agent/main/uninstall.ps1 | iex

$ErrorActionPreference = "Stop"

Write-Host ""
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
Write-Host "SkyPort Agent Uninstaller (Windows)" -ForegroundColor Cyan
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
Write-Host ""
Write-Host "💡 Tip: If skyport command is available, you can use:" -ForegroundColor Yellow
Write-Host "   skyport uninstall" -ForegroundColor Yellow
Write-Host ""
Write-Host "This script is a fallback for manual uninstallation."
Write-Host ""

$confirmation = Read-Host "Continue with manual uninstall? [y/N]"
if ($confirmation -ne 'y' -and $confirmation -ne 'Y') {
    Write-Host "Uninstall cancelled." -ForegroundColor Yellow
    exit 0
}

Write-Host ""

$INSTALL_DIR = "$env:ProgramFiles\SkyPort"
$BINARY_NAME = "skyport.exe"
$CONFIG_DIR = "$env:USERPROFILE\.skyport"
$SERVICE_NAME = "SkyPortAgent"

# Check if binary exists
$binaryPath = Join-Path $INSTALL_DIR $BINARY_NAME
if (-not (Test-Path $binaryPath)) {
    Write-Host "⚠️  SkyPort agent not found at $binaryPath" -ForegroundColor Yellow
    Write-Host "   It may already be uninstalled or installed in a different location."
    Write-Host ""
} else {
    Write-Host "✓ Found SkyPort agent at $binaryPath" -ForegroundColor Green
}

# Step 1: Check and stop/remove Windows service
Write-Host ""
Write-Host "Step 1: Checking for Windows service..." -ForegroundColor Cyan
$service = Get-Service -Name $SERVICE_NAME -ErrorAction SilentlyContinue
if ($service) {
    Write-Host "   Found Windows service. Removing..." -ForegroundColor Yellow
    
    # Stop the service
    if ($service.Status -eq 'Running') {
        Write-Host "   Stopping service..."
        Stop-Service -Name $SERVICE_NAME -Force
    }
    
    # Remove service
    Write-Host "   Removing service..."
    sc.exe delete $SERVICE_NAME | Out-Null
    
    Write-Host "   ✓ Service removed successfully" -ForegroundColor Green
} else {
    Write-Host "   ✓ No Windows service found" -ForegroundColor Green
}

# Step 2: Remove binary and installation directory
Write-Host ""
Write-Host "Step 2: Removing installation directory..." -ForegroundColor Cyan
if (Test-Path $INSTALL_DIR) {
    try {
        Remove-Item -Path $INSTALL_DIR -Recurse -Force
        Write-Host "   ✓ Installation directory removed" -ForegroundColor Green
    } catch {
        Write-Host "   ⚠️  Warning: Could not remove installation directory: $_" -ForegroundColor Yellow
        Write-Host "   You may need to manually delete: $INSTALL_DIR"
    }
} else {
    Write-Host "   ✓ Installation directory not found" -ForegroundColor Green
}

# Step 3: Remove from PATH
Write-Host ""
Write-Host "Step 3: Removing from PATH..." -ForegroundColor Cyan
$currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($currentPath -like "*$INSTALL_DIR*") {
    $newPath = ($currentPath -split ';' | Where-Object { $_ -ne $INSTALL_DIR }) -join ';'
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    Write-Host "   ✓ Removed from PATH" -ForegroundColor Green
} else {
    Write-Host "   ✓ Not found in PATH" -ForegroundColor Green
}

# Step 4: Remove configuration files
Write-Host ""
Write-Host "Step 4: Removing configuration files..." -ForegroundColor Cyan
if (Test-Path $CONFIG_DIR) {
    Write-Host "   Found config directory at $CONFIG_DIR"
    Write-Host "   This contains:"
    Write-Host "     - User authentication data"
    Write-Host "     - Tunnel configurations"
    Write-Host ""
    $removeConfig = Read-Host "   Do you want to remove it? [y/N]"
    if ($removeConfig -eq 'y' -or $removeConfig -eq 'Y') {
        try {
            Remove-Item -Path $CONFIG_DIR -Recurse -Force
            Write-Host "   ✓ Configuration files removed" -ForegroundColor Green
        } catch {
            Write-Host "   ⚠️  Warning: Could not remove config: $_" -ForegroundColor Yellow
        }
    } else {
        Write-Host "   ⊙ Configuration files kept" -ForegroundColor Yellow
    }
} else {
    Write-Host "   ✓ No configuration directory found" -ForegroundColor Green
}

Write-Host ""
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
Write-Host "✓ SkyPort Agent uninstalled successfully!" -ForegroundColor Green
Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
Write-Host ""

# Verify uninstallation
$skyportCommand = Get-Command skyport -ErrorAction SilentlyContinue
if ($skyportCommand) {
    Write-Host "⚠️  Warning: 'skyport' command still available" -ForegroundColor Yellow
    Write-Host "   It may be installed in another location."
    Write-Host "   Run: where.exe skyport"
} else {
    Write-Host "✓ Verified: 'skyport' command no longer available" -ForegroundColor Green
}

Write-Host ""
Write-Host "Thank you for using SkyPort! 👋" -ForegroundColor Cyan
Write-Host ""
Write-Host "Please restart your terminal or PowerShell session for PATH changes to take effect." -ForegroundColor Yellow
Write-Host ""

