# Installation

Install Station on any platform - Linux, macOS, Windows, Docker, or cloud environments.

## Quick Install

The fastest way to get started:

```bash
curl -fsSL https://raw.githubusercontent.com/cloudshipai/station/main/install.sh | bash
```

This script:
- Detects your OS and architecture
- Downloads the appropriate binary
- Installs to `~/.local/bin/stn`
- Adds to your PATH automatically

## Platform-Specific Installation

### Linux

**Supported Distributions:**
- Ubuntu 20.04+
- Debian 11+
- Fedora 35+
- CentOS/RHEL 8+
- Alpine Linux 3.14+

**Install via curl:**
```bash
curl -fsSL https://raw.githubusercontent.com/cloudshipai/station/main/install.sh | bash
```

**Manual installation:**
```bash
# Download binary
wget https://github.com/cloudshipai/station/releases/latest/download/station-linux-amd64.tar.gz

# Extract
tar -xzf station-linux-amd64.tar.gz

# Move to PATH
sudo mv stn /usr/local/bin/

# Verify
stn --version
```

**ARM64/aarch64:**
```bash
wget https://github.com/cloudshipai/station/releases/latest/download/station-linux-arm64.tar.gz
tar -xzf station-linux-arm64.tar.gz
sudo mv stn /usr/local/bin/
```

### macOS

**Supported Versions:**
- macOS 12 (Monterey) or later
- Both Intel and Apple Silicon (M1/M2/M3)

**Install via curl:**
```bash
curl -fsSL https://raw.githubusercontent.com/cloudshipai/station/main/install.sh | bash
```

**Homebrew (coming soon):**
```bash
brew install cloudshipai/tap/station
```

**Manual installation:**
```bash
# Intel Macs
curl -LO https://github.com/cloudshipai/station/releases/latest/download/station-darwin-amd64.tar.gz
tar -xzf station-darwin-amd64.tar.gz

# Apple Silicon (M1/M2/M3)
curl -LO https://github.com/cloudshipai/station/releases/latest/download/station-darwin-arm64.tar.gz
tar -xzf station-darwin-arm64.tar.gz

# Install (both)
sudo mv stn /usr/local/bin/
stn --version
```

**First run on macOS:**

macOS Gatekeeper may block the first run. If you see "cannot be opened because it is from an unidentified developer":

```bash
# Remove quarantine attribute
xattr -d com.apple.quarantine /usr/local/bin/stn

# Or allow in System Preferences
# System Preferences → Security & Privacy → General → "Allow Anyway"
```

### Windows

**Supported Versions:**
- Windows 10 (1809+)
- Windows 11
- Windows Server 2019+

**Install via PowerShell:**
```powershell
# Download installer
Invoke-WebRequest -Uri "https://github.com/cloudshipai/station/releases/latest/download/station-windows-amd64.zip" -OutFile "station.zip"

# Extract
Expand-Archive -Path "station.zip" -DestinationPath "$env:LOCALAPPDATA\Programs\Station"

# Add to PATH
$path = [Environment]::GetEnvironmentVariable("Path", "User")
[Environment]::SetEnvironmentVariable("Path", "$path;$env:LOCALAPPDATA\Programs\Station", "User")

# Verify (restart terminal first)
stn --version
```

**Windows Subsystem for Linux (WSL):**

Use the Linux installation method inside WSL:
```bash
curl -fsSL https://raw.githubusercontent.com/cloudshipai/station/main/install.sh | bash
```

### Docker

**Pre-built images:**
```bash
# Pull latest image
docker pull ghcr.io/cloudshipai/station:latest

# Run server mode
docker run -p 8585:8585 -p 8586:8586 \
  -e OPENAI_API_KEY=sk-... \
  ghcr.io/cloudshipai/station:latest

# Run with volume mounts (persistent data)
docker run -p 8585:8585 \
  -v $(pwd)/environments:/app/environments \
  -v $(pwd)/data:/app/data \
  -e OPENAI_API_KEY=sk-... \
  ghcr.io/cloudshipai/station:latest
```

**Build from source:**
```bash
git clone https://github.com/cloudshipai/station.git
cd station
docker build -t station:local .
```

## Verify Installation

After installation, verify Station is working:

```bash
# Check version
stn --version

# Check help
stn --help

# Verify database initialization
stn env list
```

Expected output:
```
Station v0.16.x
```

## Configuration

### AI Provider Setup

Station needs an AI provider to run agents. Configure one:

**OpenAI:**
```bash
export OPENAI_API_KEY=sk-your-key-here
stn up --provider openai
```

**Anthropic:**
```bash
export ANTHROPIC_API_KEY=sk-ant-your-key
stn up --provider anthropic
```

**Google Gemini:**
```bash
export GOOGLE_API_KEY=your-key
stn up --provider gemini --model gemini-2.5-flash
```

**Custom/Ollama:**
```bash
stn up --provider custom --base-url http://localhost:11434/v1 --model llama3-groq-tool-use
```

### Workspace Location

Station stores configuration and data in:
- **Linux/macOS**: `~/.config/station/`
- **Windows**: `%APPDATA%\station\`

Override with:
```bash
export STATION_HOME=/custom/path
```

## Updating Station

**Linux/macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/cloudshipai/station/main/install.sh | bash
```

**Windows:**
```powershell
# Download and replace existing installation
Invoke-WebRequest -Uri "https://github.com/cloudshipai/station/releases/latest/download/station-windows-amd64.zip" -OutFile "station.zip"
Expand-Archive -Path "station.zip" -DestinationPath "$env:LOCALAPPDATA\Programs\Station" -Force
```

**Docker:**
```bash
docker pull ghcr.io/cloudshipai/station:latest
```

## Uninstalling Station

**Remove binary:**
```bash
# Linux/macOS
rm -f ~/.local/bin/stn  # or /usr/local/bin/stn

# Windows
Remove-Item "$env:LOCALAPPDATA\Programs\Station" -Recurse
```

**Remove data (optional):**
```bash
# Linux/macOS
rm -rf ~/.config/station/

# Windows
Remove-Item "$env:APPDATA\station" -Recurse
```

## Troubleshooting

### Installation Issues

**"Command not found: stn"**

Ensure `~/.local/bin` is in your PATH:
```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

**Permission denied on Linux**

Add execute permission:
```bash
chmod +x ~/.local/bin/stn
```

**macOS Gatekeeper blocks execution**

Remove quarantine attribute:
```bash
xattr -d com.apple.quarantine /usr/local/bin/stn
```

### Runtime Issues

**"Failed to connect to database"**

Initialize workspace:
```bash
mkdir -p ~/.config/station
stn env list
```

**"AI provider not configured"**

Set API key:
```bash
export OPENAI_API_KEY=sk-your-key
```

**Port already in use**

Change ports:
```bash
stn up --port 8080 --mcp-port 8081
```

## System Requirements

**Minimum:**
- CPU: 1 core
- RAM: 512MB
- Disk: 200MB for binary, 1GB+ for agent data
- Network: Outbound HTTPS for AI providers

**Recommended:**
- CPU: 2+ cores
- RAM: 2GB+
- Disk: 5GB+ for multiple environments
- Network: Low-latency connection to AI provider

## Next Steps

- [Quick Start Guide](../../README.md#quick-start) - Get running in 2 minutes
- [Architecture Overview](./architecture.md) - Understand how Station works
- [Deployment Modes](./deployment-modes.md) - Choose your deployment mode
- [Agent Development](./agent-development.md) - Create your first agent
