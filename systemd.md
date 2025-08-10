# Station Systemd Service Setup

This guide helps you set up Station as a systemd service for automatic startup and management on Linux systems.

## Quick Setup (Recommended)

The easiest way is to use the install script with service setup:

```bash
curl -sSL https://getstation.cloudshipai.com | bash
# When prompted, choose to set up systemd service
```

## Manual Setup

### 1. User Service (Recommended)

Create a user service that runs Station without root privileges:

```bash
# Create systemd user directory
mkdir -p ~/.config/systemd/user

# Create service file
cat > ~/.config/systemd/user/station.service << 'EOF'
[Unit]
Description=Station AI Infrastructure Platform
After=network.target

[Service]
Type=exec
ExecStart=%h/.local/bin/stn serve --config %h/.config/station/config.yaml
Restart=always
RestartSec=10
Environment=HOME=%h
WorkingDirectory=%h

[Install]
WantedBy=default.target
EOF

# Reload systemd and enable service
systemctl --user daemon-reload
systemctl --user enable station.service
systemctl --user start station.service
```

### 2. System Service (Advanced)

For system-wide installation (requires root):

```bash
# Create system service file
sudo tee /etc/systemd/system/station.service << 'EOF'
[Unit]
Description=Station AI Infrastructure Platform
After=network.target
Wants=network.target

[Service]
Type=exec
User=station
Group=station
ExecStart=/usr/local/bin/stn serve --config /etc/station/config.yaml
Restart=always
RestartSec=10
WorkingDirectory=/var/lib/station

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/station /var/log/station

[Install]
WantedBy=multi-user.target
EOF

# Create station user
sudo useradd --system --home /var/lib/station --shell /bin/false station

# Create directories
sudo mkdir -p /var/lib/station /var/log/station /etc/station
sudo chown station:station /var/lib/station /var/log/station

# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable station.service
sudo systemctl start station.service
```

## Service Management

### Control Commands

```bash
# User service commands
systemctl --user start station      # Start service
systemctl --user stop station       # Stop service
systemctl --user restart station    # Restart service
systemctl --user status station     # Check status
systemctl --user enable station     # Enable auto-start
systemctl --user disable station    # Disable auto-start

# System service commands (use sudo)
sudo systemctl start station
sudo systemctl stop station
sudo systemctl restart station
sudo systemctl status station
sudo systemctl enable station
sudo systemctl disable station
```

### Logs

```bash
# User service logs
journalctl --user -u station -f     # Follow logs
journalctl --user -u station -n 50  # Last 50 lines

# System service logs
sudo journalctl -u station -f       # Follow logs
sudo journalctl -u station -n 50    # Last 50 lines
```

## Configuration

### Environment Variables

You can set environment variables in the service file:

```ini
[Service]
Environment=STATION_LOG_LEVEL=debug
Environment=STATION_DATABASE_URL=/path/to/station.db
Environment=GENKIT_API_KEY=your-api-key
EnvironmentFile=/etc/station/station.env  # Load from file
```

### Configuration File

Create `/etc/station/station.env` (system) or `~/.config/station/station.env` (user):

```bash
# Station configuration
STATION_LOG_LEVEL=info
STATION_DATABASE_URL=/var/lib/station/station.db
STATION_SSH_PORT=2222
STATION_MCP_PORT=3000
STATION_API_PORT=8080

# AI model configuration
GENKIT_API_KEY=your-genkit-api-key
OPENAI_API_KEY=your-openai-api-key
ANTHROPIC_API_KEY=your-anthropic-api-key
```

## Troubleshooting

### Service Won't Start

1. **Check service status:**
   ```bash
   systemctl --user status station
   ```

2. **Check logs:**
   ```bash
   journalctl --user -u station -n 50
   ```

3. **Verify binary path:**
   ```bash
   which stn
   # Update ExecStart path in service file if needed
   ```

4. **Check configuration:**
   ```bash
   stn --config ~/.config/station/config.yaml --help
   ```

### Permission Issues

For user services:
```bash
# Ensure directories exist and are writable
mkdir -p ~/.config/station ~/.local/share/station
chmod 755 ~/.config/station ~/.local/share/station
```

For system services:
```bash
# Fix ownership
sudo chown -R station:station /var/lib/station
sudo chmod 755 /var/lib/station
```

### Port Conflicts

Check if ports are in use:
```bash
# Check if Station ports are available
sudo netstat -tlnp | grep -E ':(2222|3000|8080)\s'

# Or use ss command
ss -tlnp | grep -E ':(2222|3000|8080)\s'
```

Update configuration if ports conflict:
```yaml
# ~/.config/station/config.yaml
ssh_port: 2223
mcp_port: 3001
api_port: 8081
```

### Restart on Boot

User services require login session:
```bash
# Enable lingering for user services to start at boot
sudo loginctl enable-linger $USER
```

## Advanced Configuration

### Multiple Instances

Run multiple Station instances with different configurations:

```bash
# Create additional service file
cp ~/.config/systemd/user/station.service ~/.config/systemd/user/station-dev.service

# Edit the new service file
sed -i 's/station.service/station-dev.service/' ~/.config/systemd/user/station-dev.service
sed -i 's/config.yaml/config-dev.yaml/' ~/.config/systemd/user/station-dev.service

# Enable and start
systemctl --user daemon-reload
systemctl --user enable station-dev.service
systemctl --user start station-dev.service
```

### Resource Limits

Add resource limits to prevent resource exhaustion:

```ini
[Service]
# Memory limit (optional)
MemoryMax=1G
MemoryHigh=800M

# CPU limit (optional)
CPUQuota=200%

# File descriptor limit
LimitNOFILE=65536
```

### Security Hardening

Additional security settings for system services:

```ini
[Service]
# Process security
NoNewPrivileges=true
PrivateTmp=true
PrivateDevices=true
ProtectSystem=strict
ProtectHome=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true

# Network security
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
RestrictNamespaces=true
RestrictRealtime=true

# System call filtering
SystemCallFilter=@system-service
SystemCallErrorNumber=EPERM

# Writable directories
ReadWritePaths=/var/lib/station /var/log/station
```

## Integration with Other Services

### Nginx Reverse Proxy

If using nginx as a reverse proxy:

```bash
# Create nginx dependency
sudo mkdir -p /etc/systemd/system/station.service.d
sudo tee /etc/systemd/system/station.service.d/nginx.conf << 'EOF'
[Unit]
After=nginx.service
EOF
```

### Database Services

If using external database:

```bash
# Create database dependency
sudo tee /etc/systemd/system/station.service.d/database.conf << 'EOF'
[Unit]
After=postgresql.service
Wants=postgresql.service
EOF
```