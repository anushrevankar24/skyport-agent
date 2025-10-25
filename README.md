# SkyPort Agent

A secure tunnel client that exposes local services to the internet through encrypted tunnels.

## 📥 Installation

### Quick Install (Recommended)

#### Linux / macOS
```bash
curl -fsSL https://raw.githubusercontent.com/anushrevankar24/skyport-agent/main/install.sh | bash
```

Or with wget:
```bash
wget -qO- https://raw.githubusercontent.com/anushrevankar24/skyport-agent/main/install.sh | bash
```

#### Windows (PowerShell)
Open PowerShell as Administrator and run:
```powershell
irm https://raw.githubusercontent.com/anushrevankar24/skyport-agent/main/install.ps1 | iex
```

### Manual Installation

If the automated scripts don't work, download the binary for your platform:

1. Go to [Releases](https://github.com/anushrevankar24/skyport-agent/releases/latest)
2. Download the binary for your OS:
   - **Linux (64-bit)**: `skyport-linux-amd64`
   - **Linux (ARM64)**: `skyport-linux-arm64`
   - **macOS (Intel)**: `skyport-darwin-amd64`
   - **macOS (M1/M2/M3)**: `skyport-darwin-arm64`
   - **Windows (64-bit)**: `skyport-windows-amd64.exe`

3. Install the binary:

**Linux/macOS:**
```bash
# Make it executable
chmod +x skyport-*

# Move to system PATH
sudo mv skyport-* /usr/local/bin/skyport

# Or install to user directory (no sudo needed)
mkdir -p ~/.local/bin
mv skyport-* ~/.local/bin/skyport
export PATH="$HOME/.local/bin:$PATH"
```

**Windows:**
1. Rename downloaded file to `skyport.exe`
2. Move to a directory in your PATH (e.g., `C:\Program Files\SkyPort\`)
3. Or add the directory to your PATH environment variable

### Verify Installation

```bash
skyport --version
```

## 🚀 Quick Start

### 1. Login to Your Account

```bash
skyport login
```

This will open your browser for authentication. Once logged in, your credentials are securely stored.

### 2. List Your Tunnels

```bash
skyport tunnel list
```

### 3. Start a Tunnel

```bash
skyport tunnel run <tunnel-name>
```

Your local service is now accessible via the internet!

### 4. Run in Background (Daemon Mode)

```bash
skyport tunnel run <tunnel-name> --background
```

## 📖 Usage Examples

### Expose Local Web Server

```bash
# Start your local service
python3 -m http.server 8000

# Start tunnel (in another terminal)
skyport tunnel run my-web-app
```

Your service is now live at: `https://your-subdomain.skyports.tech`

### Expose Local API

```bash
# Start your API server
npm run dev  # Running on localhost:3000

# Start tunnel
skyport tunnel run my-api
```

### Multiple Tunnels

```bash
# List all tunnels
skyport tunnel list

# Run specific tunnel
skyport tunnel run frontend-app
skyport tunnel run backend-api
```

## ⚙️ Advanced Usage

### Run as System Service

Install SkyPort as a system service (runs automatically on boot):

```bash
# Install service
skyport service install

# Start service
skyport service start

# Check status
skyport service status

# Stop service
skyport service stop

# Uninstall service
skyport service uninstall
```

### Daemon Management

```bash
# Start daemon
skyport daemon start

# Stop daemon
skyport daemon stop

# Restart daemon
skyport daemon restart

# Check status
skyport status
```

### Tunnel Management

```bash
# List all tunnels
skyport tunnel list

# Stop a running tunnel
skyport tunnel stop <tunnel-name>

# View tunnel details
skyport tunnel info <tunnel-name>
```

## 📋 Available Commands

```bash
skyport login              # Authenticate with SkyPort
skyport status             # Show agent and tunnel status
skyport tunnel list        # List all your tunnels
skyport tunnel run <name>  # Start a tunnel
skyport tunnel stop <name> # Stop a tunnel
skyport service install    # Install as system service
skyport service start      # Start the service
skyport service stop       # Stop the service
skyport service status     # Check service status
skyport daemon start       # Start daemon
skyport daemon stop        # Stop daemon
skyport --help             # Show all commands
```

## 🔧 Configuration

SkyPort Agent stores configuration in:
- **Linux/macOS**: `~/.skyport/`
- **Windows**: `%LOCALAPPDATA%\skyport\`

Configuration includes:
- Authentication tokens
- Tunnel settings
- Service configuration

## 🛠️ For Developers

### Building from Source

#### Prerequisites
- Go 1.21 or later
- Git

#### Build Production Binary

```bash
cd skyport-agent

# Build with production configuration
./scripts/build-prod.sh
```

This produces a `skyport` binary with production URLs baked in.

#### Configure Production URLs

Edit `scripts/build-config-prod.env`:

```bash
SKYPORT_SERVER_URL=https://api.skyports.tech/api/v1
SKYPORT_WEB_URL=https://skyports.tech
SKYPORT_TUNNEL_DOMAIN=tunnel.skyports.tech
```

Then rebuild:

```bash
./scripts/build-prod.sh
```

The URLs are now compiled into the binary!

#### Build for Different Platforms

```bash
# Linux (64-bit)
GOOS=linux GOARCH=amd64 ./scripts/build-prod.sh

# Linux (ARM64)
GOOS=linux GOARCH=arm64 ./scripts/build-prod.sh

# macOS (Intel)
GOOS=darwin GOARCH=amd64 ./scripts/build-prod.sh

# macOS (M1/M2/M3)
GOOS=darwin GOARCH=arm64 ./scripts/build-prod.sh

# Windows (64-bit)
GOOS=windows GOARCH=amd64 ./scripts/build-prod.sh
```

#### Local Development

Build with local development URLs:

```bash
./scripts/build-local.sh
```

Or override URLs at runtime:

```bash
export SKYPORT_SERVER_URL=http://localhost:8080/api/v1
export SKYPORT_WEB_URL=http://localhost:3000
export SKYPORT_TUNNEL_DOMAIN=localhost:8080
./skyport-local login
```

#### Verify Built-in URLs

```bash
strings skyport | grep -E "https://.*skyports"
```

Should show your production URLs.

### Project Structure

```
skyport-agent/
├── scripts/
│   ├── build-prod.sh          # Production build script
│   ├── build-local.sh         # Local development build
│   ├── build-config-prod.env  # Production URLs
│   └── build-config-local.env # Local URLs
├── cmd/
│   └── skyport/
│       └── main.go            # Application entry point
├── internal/
│   ├── auth/                  # Authentication logic
│   ├── cli/                   # CLI commands
│   ├── config/                # Configuration management
│   ├── service/               # System service management
│   └── tunnel/                # Tunnel protocol implementation
├── go.mod                     # Go dependencies
├── go.sum                     # Dependency checksums
└── README.md                  # This file
```

## 🚀 Deployment & Release Process

### Repository Setup

#### 1. Create a New GitHub Repository

```bash
# Navigate to the skyport-agent directory
cd skyport-agent

# Initialize git (if not already initialized)
git init

# Add all files
git add .

# Create initial commit
git commit -m "Initial commit: SkyPort Agent v1.0.0"

# Add remote
git remote add origin https://github.com/anushrevankar24/skyport-agent.git

# Push to GitHub
git branch -M main
git push -u origin main
```

#### 2. Verify Production Configuration

Before creating your first release, ensure `scripts/build-config-prod.env` has the correct production URLs:

```bash
cat scripts/build-config-prod.env
```

Should show:
```env
SKYPORT_SERVER_URL=https://api.skyports.tech/api/v1
SKYPORT_WEB_URL=https://skyports.tech
SKYPORT_TUNNEL_DOMAIN=tunnel.skyports.tech
```

Update if necessary:
```bash
nano scripts/build-config-prod.env
```

#### 3. Create Your First Release

The GitHub Actions workflow will automatically build binaries for all platforms when you push a version tag:

```bash
# Create and push a version tag
git tag v1.0.0
git push origin v1.0.0
```

This triggers the `.github/workflows/release.yml` workflow which will:
1. Build binaries for all platforms (Linux, macOS, Windows)
2. Generate checksums
3. Create a GitHub release
4. Attach all binaries to the release

#### 4. Verify Release

1. Go to https://github.com/anushrevankar24/skyport-agent/releases
2. You should see your release with all platform binaries
3. Test the install scripts:

**Linux/macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/anushrevankar24/skyport-agent/main/install.sh | bash
```

**Windows:**
```powershell
irm https://raw.githubusercontent.com/anushrevankar24/skyport-agent/main/install.ps1 | iex
```

### Release Process

#### Creating New Releases

For subsequent releases:

1. Make your changes and commit them
2. Update version and create a tag:
   ```bash
   git tag v1.1.0
   git push origin v1.1.0
   ```

#### Version Naming Convention

Use semantic versioning:
- `v1.0.0` - Major release (breaking changes)
- `v1.1.0` - Minor release (new features)
- `v1.0.1` - Patch release (bug fixes)

### Supported Platforms

The release workflow builds for:

- **Linux**
  - `skyport-linux-amd64` - 64-bit Intel/AMD
  - `skyport-linux-arm64` - 64-bit ARM
  - `skyport-linux-arm` - 32-bit ARM

- **macOS**
  - `skyport-darwin-amd64` - Intel Macs
  - `skyport-darwin-arm64` - M1/M2/M3 Macs

- **Windows**
  - `skyport-windows-amd64.exe` - 64-bit
  - `skyport-windows-386.exe` - 32-bit

### Updating Production URLs

If your server URLs change:

1. Update `scripts/build-config-prod.env`
2. Commit the changes
3. Create a new release tag
4. The new binaries will have the updated URLs compiled in

### Manual Build (For Testing)

Build locally before pushing:

```bash
# Test production build
./scripts/build-prod.sh

# Test on your system
./skyport --help
./skyport login
```

Cross-compile for other platforms:
```bash
# Linux ARM64
GOOS=linux GOARCH=arm64 ./scripts/build-prod.sh

# macOS M1
GOOS=darwin GOARCH=arm64 ./scripts/build-prod.sh

# Windows
GOOS=windows GOARCH=amd64 ./scripts/build-prod.sh
```

## 🔍 How It Works

1. **Compile-time Configuration**: Production URLs are baked into the binary during build
2. **Authentication**: Secure OAuth-based login via web browser
3. **Tunnel Protocol**: WebSocket-based encrypted tunnel connection
4. **Service Management**: Optional systemd/launchd integration for auto-start
5. **Health Monitoring**: Automatic reconnection and health checks

## 🐛 Troubleshooting

### Command not found

If you get "command not found" after installation:

**Linux/macOS:**
```bash
# Check if binary exists
ls -la /usr/local/bin/skyport

# Add to PATH if installed in custom location
export PATH="$HOME/.local/bin:$PATH"

# Add to your shell profile (~/.bashrc or ~/.zshrc)
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

**Windows:**
1. Open "Environment Variables" in System Settings
2. Add the installation directory to your PATH
3. Restart PowerShell/CMD

### Permission denied

**Linux/macOS:**
```bash
# Make sure binary is executable
chmod +x /usr/local/bin/skyport

# Or reinstall with proper permissions
curl -fsSL https://raw.githubusercontent.com/anushrevankar24/skyport-agent/main/install.sh | bash
```

### Connection issues

```bash
# Check your internet connection
ping skyports.tech

# Check agent status
skyport status

# View logs
journalctl -u skyport -f  # Linux (systemd)
```

### Build Fails

Check that:
1. `scripts/build-config-prod.env` exists and has valid URLs
2. All Go dependencies are specified in `go.mod`
3. Code compiles locally: `./scripts/build-prod.sh`

### Install Scripts Not Working

1. Verify the release exists and has all binaries
2. Check that binary names match what's in the install scripts
3. Test download URL manually:
   ```bash
   curl -I https://github.com/anushrevankar24/skyport-agent/releases/latest/download/skyport-linux-amd64
   ```

### Users Can't Run `skyport` Command

The install scripts automatically add the binary to PATH. If users still have issues:

**Linux/macOS:**
- Check: `which skyport`
- If not found, ensure `/usr/local/bin` is in PATH
- Alternative: Install to `~/.local/bin` and add to PATH

**Windows:**
- Check: `where.exe skyport`
- If not found, manually add installation directory to PATH
- May need to restart terminal

### Uninstall

**Linux/macOS:**
```bash
# Remove binary
sudo rm /usr/local/bin/skyport

# Remove configuration
rm -rf ~/.skyport

# Remove service (if installed)
skyport service uninstall
```

**Windows:**
```powershell
# Remove binary
Remove-Item "$env:LOCALAPPDATA\SkyPort" -Recurse

# Remove from PATH manually via Environment Variables
```

## 🔒 Security Best Practices

1. **Never commit secrets** to the repository
2. **Sign releases** (optional): Enable GPG signing for tags
3. **Verify checksums**: Instruct users to verify `checksums.txt`
4. **Use HTTPS**: All URLs in build config should use HTTPS

## 📊 Monitoring Releases

Track release downloads:
1. Go to repository Insights → Traffic
2. Check release download counts
3. Monitor issues for installation problems

## 📚 Additional Resources

- **Documentation**: [https://docs.skyports.tech](https://docs.skyports.tech)
- **Issues**: [GitHub Issues](https://github.com/anushrevankar24/skyport-agent/issues)
- **Website**: [https://skyports.tech](https://skyports.tech)
- **Repository**: [https://github.com/anushrevankar24/skyport-agent](https://github.com/anushrevankar24/skyport-agent)
- **Releases**: [https://github.com/anushrevankar24/skyport-agent/releases](https://github.com/anushrevankar24/skyport-agent/releases)

### Development Resources

- GitHub Actions Docs: https://docs.github.com/en/actions
- Go Cross-Compilation: https://go.dev/doc/install/source#environment
- Semantic Versioning: https://semver.org/

## 📄 License

See LICENSE file.